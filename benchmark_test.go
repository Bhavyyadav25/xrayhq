package xrayhq

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkMiddleware(b *testing.B) {
	cfg := DefaultConfig()
	cfg.CaptureBody = false
	cfg.CaptureHeaders = false
	c := NewCollector(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	wrapped := coreMiddleware(c, cfg, handler)

	req := httptest.NewRequest("GET", "/api/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

func BenchmarkMiddlewareWithBodyCapture(b *testing.B) {
	cfg := DefaultConfig()
	cfg.CaptureBody = true
	cfg.CaptureHeaders = true
	c := NewCollector(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	})
	wrapped := coreMiddleware(c, cfg, handler)

	req := httptest.NewRequest("GET", "/api/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

func BenchmarkCollectorRecord(b *testing.B) {
	cfg := DefaultConfig()
	c := NewCollector(cfg)

	trace := &RequestTrace{
		ID:             "bench",
		Method:         "GET",
		Path:           "/test",
		RoutePattern:   "/test",
		ResponseStatus: 200,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Record(trace)
	}
}

func BenchmarkCollectorRecordParallel(b *testing.B) {
	cfg := DefaultConfig()
	c := NewCollector(cfg)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		trace := &RequestTrace{
			ID:             generateID(),
			Method:         "GET",
			Path:           "/test",
			RoutePattern:   "/test",
			ResponseStatus: 200,
		}
		for pb.Next() {
			c.Record(trace)
		}
	})
}
