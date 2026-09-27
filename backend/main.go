package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"smart-visitors/internal/config"
	"smart-visitors/internal/db"
	"smart-visitors/internal/handlers"
)

func main() {
	cfg := config.Load()
	sqlDB, dbPath, err := db.Open(cfg.DBConnection)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	log.Printf("db: %s artifacts: %s", dbPath, cfg.AIArtifactDir)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/openapi.json", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"openapi": "3.0.0",
			"info":    map[string]any{"title": "Smart Visitor Access Control API", "version": "2.0.0-go"},
			"paths": map[string]any{
				"/users/create":        map[string]any{},
				"/passes/create":       map[string]any{},
				"/passes/scan":         map[string]any{},
				"/ai/anomaly/train":    map[string]any{},
				"/ai/anomaly/flags":    map[string]any{},
				"/ai/anomaly/score/{log_id}": map[string]any{},
				"/ai/query":            map[string]any{},
			},
		})
	})
	r.Get("/docs", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1>Smart Visitors API (Go)</h1><p>See /openapi.json</p></body></html>`))
	})
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Post("/users/create", handlers.UsersCreate(sqlDB))
	r.Post("/passes/create", handlers.PassesCreate(sqlDB))
	r.Post("/passes/scan", handlers.PassesScan(sqlDB))
	r.Post("/ai/anomaly/train", handlers.AITrain(sqlDB, cfg))
	r.Get("/ai/anomaly/flags", handlers.AIFlags(sqlDB, cfg))
	r.Get("/ai/anomaly/score/{log_id}", handlers.AIScore(sqlDB, cfg))
	r.Post("/ai/query", handlers.AIQuery(sqlDB, cfg))

	addr := ":" + cfg.Port
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
