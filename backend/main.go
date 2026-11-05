package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/config"
	"smart-visitors/internal/cryptoutil"
	"smart-visitors/internal/db"
	"smart-visitors/internal/handlers"
	"smart-visitors/internal/jobs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}
	if err := cfg.ValidateForStartup(); err != nil {
		log.Fatalf("configuration: %v", err)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database initialization: %v", err)
	}
	defer database.Close()

	migrationCtx, migrationCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	if err := handlers.MigrateVisitorPII(migrationCtx, database, cfg.MasterKey); err != nil {
		migrationCancel()
		log.Fatalf("visitor PII encryption migration: %v", err)
	}
	migrationCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := handlers.Bootstrap(ctx, database, cfg); err != nil {
		log.Fatalf("application bootstrap: %v", err)
	}
	go runMaintenance(database, cfg)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(requestLogger)
	healthCheck := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	router.Get("/healthz", healthCheck)
	router.Get("/readyz", readiness(database, cfg))

	router.Route("/api", func(api chi.Router) {
		api.Get("/healthz", healthCheck)
		api.Get("/fill/{token}", handlers.InviteGet(database, cfg))
		api.Post("/fill/{token}", handlers.InviteFill(database, cfg))
		api.Post("/fill/{token}/photo", handlers.VisitorPhotoUpload(database, cfg))
		api.Get("/p/{token}", handlers.PublicPass(database, cfg))
		api.Post("/auth/login", handlers.Login(database, cfg))
		api.Group(func(protected chi.Router) {
			protected.Use(auth.Middleware(cfg.JWTSecret, func(ctx context.Context, id string) (auth.User, error) {
				return handlers.LookupUser(ctx, database, id)
			}))

			protected.Get("/me", handlers.Me)
			protected.Post("/auth/logout", handlers.Logout)
			protected.Post("/me/password", handlers.ChangePassword(database))

			protected.With(auth.RequireRoles("admin")).Post("/users", handlers.UsersCreate(database))
			protected.With(auth.RequireRoles("admin")).Get("/users", handlers.UsersList(database))
			protected.With(auth.RequireRoles("admin", "guard")).Get("/hosts", handlers.HostsList(database))
			protected.With(auth.RequireRoles("admin")).Post("/users/{id}/resetpassword", handlers.UserResetPassword(database))
			protected.With(auth.RequireRoles("admin")).Post("/users/{id}/disable", handlers.UserDisable(database))
			protected.With(auth.RequireRoles("admin")).Get("/events", handlers.AuditEventsList(database))
			protected.With(auth.RequireRoles("admin")).Get("/logs/verify", handlers.VerifyLogs(database))
			protected.With(auth.RequireRoles("admin", "host")).Post("/visits", handlers.VisitsCreate(database, cfg))
			protected.With(auth.RequireRoles("admin", "host")).Get("/visits", handlers.VisitsList(database, cfg))
			protected.With(auth.RequireRoles("admin", "host")).Get("/passes/{id}/details", handlers.PassDetails(database, cfg))
			protected.With(auth.RequireRoles("admin", "host", "guard")).Get("/visitors/{id}/photo", handlers.VisitorPhotoGet(database, cfg))
			protected.With(auth.RequireRoles("admin", "host")).Get("/passes/{id}/qr.png", handlers.PassQR(database, cfg))
			protected.With(auth.RequireRoles("admin", "host")).Post("/passes/{id}/revoke", handlers.PassRevoke(database))
			protected.With(auth.RequireRoles("admin")).Post("/passes/{id}/extend", handlers.PassExtend(database, cfg))
			protected.With(auth.RequireRoles("admin", "guard")).Post("/scan", handlers.PassesScan(database, cfg))
			protected.With(auth.RequireRoles("admin")).Get("/logs", handlers.AccessLogsList(database))
			protected.With(auth.RequireRoles("admin")).Post("/ai/query", handlers.AIQuery(database, cfg))
			protected.With(auth.RequireRoles("admin")).Get("/ai/anomaly/flags", handlers.AIFlags(database))
			protected.With(auth.RequireRoles("admin")).Post("/ai/anomaly/train", handlers.AITrain(database, cfg))
			protected.With(auth.RequireRoles("admin")).Post("/stickers/batches", handlers.StickerBatchCreate(database, cfg))
			protected.With(auth.RequireRoles("admin")).Get("/stickers", handlers.StickersList(database))
			protected.With(auth.RequireRoles("admin")).Post("/stickers/{id}/retire", handlers.StickerRetire(database))
			protected.With(auth.RequireRoles("guard")).Post("/stickers/claim", handlers.StickerClaim(database, cfg))
			protected.With(auth.RequireRoles("guard")).Post("/walkins", handlers.WalkinCreate(database, cfg))
			protected.With(auth.RequireRoles("admin", "guard")).Get("/gates", handlers.GatesList(database))
			protected.With(auth.RequireRoles("admin")).Post("/gates", handlers.GatesCreate(database))
			protected.With(auth.RequireRoles("admin")).Patch("/gates/{id}", handlers.GatesUpdate(database))
		})
		api.With(handlers.RequireMLSignature(cfg.MLServiceToken)).Get("/internal/scans/{pass_id}/history", handlers.InternalScanHistory(database))
		api.With(handlers.RequireMLSignature(cfg.MLServiceToken)).Post("/internal/logs/{log_id}/anomaly", handlers.InternalAnomalyWriteback(database))
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	slog.Info("starting Smart Visitors API", "port", cfg.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}

func runMaintenance(database *sql.DB, cfg config.Config) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	lastPassRetentionDate := ""
	lastLogRetentionDate := ""
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		now := time.Now()
		if now.Minute()%5 == 0 {
			if err := jobs.ExpirePasses(ctx, database); err != nil {
				slog.Error("pass expiry sweep failed", "error", err)
			}
		}
		date := now.Format("2006-01-02")
		afterPassRetentionTime := now.Hour() > 3 || (now.Hour() == 3 && now.Minute() >= 30)
		if afterPassRetentionTime && date != lastPassRetentionDate {
			if err := jobs.ShredExpiredPasses(ctx, database, cfg); err != nil {
				slog.Error("pass retention sweep failed", "error", err)
			} else {
				lastPassRetentionDate = date
			}
		}
		if now.Hour() >= 4 && date != lastLogRetentionDate {
			if err := jobs.PurgeLogs(ctx, database, cfg.RetentionLogDays); err != nil {
				slog.Error("log retention sweep failed", "error", err)
			} else {
				lastLogRetentionDate = date
			}
		}
		cancel()
	}
}

func readiness(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := database.PingContext(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database"})
			return
		}
		plaintext := []byte("readiness-check")
		sealed, err := cryptoutil.Seal(cfg.MasterKey, plaintext, []byte("readyz"))
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "master_key"})
			return
		}
		opened, err := cryptoutil.Open(cfg.MasterKey, sealed, []byte("readyz"))
		if err != nil || string(opened) != string(plaintext) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "master_key"})
			return
		}
		var issuerExists bool
		if err := database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM keys WHERE name = 'issuer-ed25519')`).Scan(&issuerExists); err != nil || !issuerExists {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "issuer_key"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *responseRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *responseRecorder) Write(body []byte) (int, error) {
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}
	return recorder.ResponseWriter.Write(body)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		r = r.WithContext(auth.WithRequestMetadata(r.Context()))
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		slog.Info("http request",
			"request_id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"route", route,
			"role", auth.RequestRole(r.Context()),
			"status", recorder.status,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
