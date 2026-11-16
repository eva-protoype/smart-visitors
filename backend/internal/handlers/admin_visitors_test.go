package handlers

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestVisitorExportWindow(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/admin/visitors?from=2026-01-02T03:04:05Z&to=2026-01-03T03:04:05%2B01:00", nil)
	from, to, err := visitorExportWindow(request)
	if err != nil {
		t.Fatalf("visitorExportWindow() error = %v", err)
	}
	if !from.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("unexpected from: %s", from)
	}
	if !to.Equal(time.Date(2026, 1, 3, 2, 4, 5, 0, time.UTC)) {
		t.Fatalf("unexpected to: %s", to)
	}
}

func TestVisitorExportWindowRejectsInvalidRange(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/admin/visitors?from=2026-01-03T00:00:00Z&to=2026-01-02T00:00:00Z", nil)
	if _, _, err := visitorExportWindow(request); err == nil {
		t.Fatal("visitorExportWindow() accepted an invalid range")
	}
}
