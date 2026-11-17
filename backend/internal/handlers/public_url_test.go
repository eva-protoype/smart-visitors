package handlers

import (
	"testing"

	"smart-visitors/internal/config"
)

func TestVisitorLinksFallBackToAppBaseURL(t *testing.T) {
	cfg := config.Config{AppBaseURL: "https://tunnel.example/"}
	if got := visitorFillURL(cfg, "invite-token"); got != "https://tunnel.example/fill/invite-token" {
		t.Fatalf("visitorFillURL = %q", got)
	}
	if got := visitorPassURL(cfg, "qr-token"); got != "https://tunnel.example/p/qr-token" {
		t.Fatalf("visitorPassURL = %q", got)
	}
}

func TestVisitorLinksUseVisitorBaseURL(t *testing.T) {
	cfg := config.Config{
		AppBaseURL:     "https://tunnel.example",
		VisitorBaseURL: "https://project-dawnguard.github.io/visitor/",
	}
	if got := visitorFillURL(cfg, "invite-token"); got != "https://project-dawnguard.github.io/visitor/fill/invite-token" {
		t.Fatalf("visitorFillURL = %q", got)
	}
	if got := visitorPassURL(cfg, "qr-token"); got != "https://project-dawnguard.github.io/visitor/p/qr-token" {
		t.Fatalf("visitorPassURL = %q", got)
	}
}

func TestVisitorLinksUseHashRoutesWhenEnabled(t *testing.T) {
	cfg := config.Config{
		AppBaseURL:        "https://tunnel.example",
		VisitorBaseURL:    "https://project-dawnguard.github.io/visitor",
		VisitorHashRoutes: true,
	}
	if got := visitorFillURL(cfg, "invite-token"); got != "https://project-dawnguard.github.io/visitor/#/fill/invite-token" {
		t.Fatalf("visitorFillURL = %q", got)
	}
	if got := visitorPassURL(cfg, "qr-token"); got != "https://project-dawnguard.github.io/visitor/#/p/qr-token" {
		t.Fatalf("visitorPassURL = %q", got)
	}
}
