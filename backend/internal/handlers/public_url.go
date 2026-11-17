package handlers

import (
	"strings"

	"smart-visitors/internal/config"
)

func visitorBaseURL(cfg config.Config) string {
	base := cfg.VisitorBaseURL
	if base == "" {
		base = cfg.AppBaseURL
	}
	return strings.TrimRight(base, "/")
}

func visitorFillURL(cfg config.Config, token string) string {
	return visitorURL(cfg, "fill", token)
}

func visitorPassURL(cfg config.Config, token string) string {
	return visitorURL(cfg, "p", token)
}

func visitorURL(cfg config.Config, route, token string) string {
	base := visitorBaseURL(cfg)
	if cfg.VisitorHashRoutes {
		return base + "/#/" + route + "/" + token
	}
	return base + "/" + route + "/" + token
}
