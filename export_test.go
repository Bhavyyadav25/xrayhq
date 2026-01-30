package xrayhq

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExportJSON(t *testing.T) {
	c, cfg := setupTestCollector()
	c.Record(&RequestTrace{
		ID:             "export-1",
		Method:         "GET",
		Path:           "/api/test",
		RoutePattern:   "/api/test",
		ResponseStatus: 200,
		Latency:        10 * time.Millisecond,
		StartTime:      time.Now(),
	})

	ds := &DashboardServer{collector: c, config: cfg}
	req := httptest.NewRequest("GET", "/xrayhq/export?format=json", nil)
	rec := httptest.NewRecorder()
	ds.handleExport(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type")
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 request in export, got %d", len(result))
	}
}

func TestExportCSV(t *testing.T) {
	c, cfg := setupTestCollector()
	c.Record(&RequestTrace{
		ID:             "export-2",
		Method:         "POST",
		Path:           "/api/users",
		RoutePattern:   "/api/users",
		ResponseStatus: 201,
		Latency:        20 * time.Millisecond,
		StartTime:      time.Now(),
	})

	ds := &DashboardServer{collector: c, config: cfg}
	req := httptest.NewRequest("GET", "/xrayhq/export?format=csv", nil)
	rec := httptest.NewRecorder()
	ds.handleExport(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
		t.Errorf("expected CSV content type")
	}

	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 2 { // header + 1 row
		t.Errorf("expected 2 lines (header+data), got %d", len(lines))
	}
}
