package xrayhq

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestCollector() (*Collector, *Config) {
	cfg := DefaultConfig()
	cfg.CaptureBody = true
	cfg.CaptureHeaders = true
	c := NewCollector(cfg)
	return c, cfg
}

func TestMiddlewareCapturesBasicData(t *testing.T) {
	c, cfg := setupTestCollector()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	wrapped := coreMiddleware(c, cfg, handler)

	req := httptest.NewRequest("GET", "/api/test?foo=bar", nil)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	time.Sleep(10 * time.Millisecond) // let recording finish

	recent := c.GetRecentRequests(1)
	if len(recent) != 1 {
		t.Fatalf("expected 1 request, got %d", len(recent))
	}

	trace := recent[0]
	if trace.Method != "GET" {
		t.Errorf("expected GET, got %s", trace.Method)
	}
	if trace.Path != "/api/test" {
		t.Errorf("expected /api/test, got %s", trace.Path)
	}
	if trace.QueryParams != "foo=bar" {
		t.Errorf("expected foo=bar, got %s", trace.QueryParams)
	}
	if trace.ResponseStatus != 200 {
		t.Errorf("expected status 200, got %d", trace.ResponseStatus)
	}
	if trace.UserAgent != "test-agent" {
		t.Errorf("expected test-agent, got %s", trace.UserAgent)
	}
	if trace.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestMiddlewareCapturesRequestBody(t *testing.T) {
	c, cfg := setupTestCollector()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		w.Write(buf.Bytes())
	})

	wrapped := coreMiddleware(c, cfg, handler)

	body := []byte(`{"name":"test"}`)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	recent := c.GetRecentRequests(1)
	if len(recent) != 1 {
		t.Fatalf("expected 1 request")
	}

	trace := recent[0]
	if string(trace.RequestBody) != `{"name":"test"}` {
		t.Errorf("expected request body captured, got %s", string(trace.RequestBody))
	}
}

func TestMiddlewarePanicRecovery(t *testing.T) {
	c, cfg := setupTestCollector()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic!")
	})

	wrapped := coreMiddleware(c, cfg, handler)

	req := httptest.NewRequest("GET", "/api/panic", nil)
	rec := httptest.NewRecorder()

	// Should not panic
	wrapped.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("expected 500 after panic, got %d", rec.Code)
	}

	recent := c.GetRecentRequests(1)
	if len(recent) != 1 {
		t.Fatalf("expected 1 request")
	}

	trace := recent[0]
	if !trace.Panicked {
		t.Error("expected Panicked to be true")
	}
	if trace.PanicStack == "" {
		t.Error("expected panic stack trace")
	}
}

func TestMiddlewareDBQueryCapture(t *testing.T) {
	c, cfg := setupTestCollector()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		AddDBQuery(r.Context(), DBQuery{
			Query:     "SELECT * FROM users",
			Duration:  5 * time.Millisecond,
			Timestamp: time.Now(),
		})
		AddDBQuery(r.Context(), DBQuery{
			Query:     "SELECT * FROM orders",
			Duration:  3 * time.Millisecond,
			Timestamp: time.Now(),
		})
		w.Write([]byte("ok"))
	})

	wrapped := coreMiddleware(c, cfg, handler)
	req := httptest.NewRequest("GET", "/api/data", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	recent := c.GetRecentRequests(1)
	trace := recent[0]

	if len(trace.DBQueries) != 2 {
		t.Errorf("expected 2 DB queries, got %d", len(trace.DBQueries))
	}
	if trace.TotalDBTime != 8*time.Millisecond {
		t.Errorf("expected 8ms total DB time, got %v", trace.TotalDBTime)
	}
}

func TestResponseWriterCapture(t *testing.T) {
	start := time.Now()
	rw := newResponseWriter(httptest.NewRecorder(), start, true)

	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte("hello"))
	rw.Write([]byte(" world"))

	if rw.statusCode != 201 {
		t.Errorf("expected 201, got %d", rw.statusCode)
	}
	if rw.size != 11 {
		t.Errorf("expected 11 bytes, got %d", rw.size)
	}
	if rw.body.String() != "hello world" {
		t.Errorf("expected 'hello world', got %s", rw.body.String())
	}
	if rw.ttfb <= 0 {
		t.Error("expected positive TTFB")
	}
}
