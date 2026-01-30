package xrayhq

import (
	"sync"
	"testing"
	"time"
)

func TestCollectorRecord(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BufferSize = 5
	c := NewCollector(cfg)

	for i := 0; i < 3; i++ {
		c.Record(&RequestTrace{
			ID:             generateID(),
			Method:         "GET",
			Path:           "/test",
			RoutePattern:   "/test",
			ResponseStatus: 200,
			Latency:        10 * time.Millisecond,
			StartTime:      time.Now(),
		})
	}

	if c.RequestCount() != 3 {
		t.Errorf("expected 3 requests, got %d", c.RequestCount())
	}

	recent := c.GetRecentRequests(10)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent requests, got %d", len(recent))
	}
}

func TestCollectorRingBufferOverflow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BufferSize = 3
	c := NewCollector(cfg)

	for i := 0; i < 10; i++ {
		c.Record(&RequestTrace{
			ID:             generateID(),
			Method:         "GET",
			Path:           "/test",
			RoutePattern:   "/test",
			ResponseStatus: 200,
			Latency:        time.Millisecond,
			StartTime:      time.Now(),
		})
	}

	if c.RequestCount() != 3 {
		t.Errorf("expected buffer capped at 3, got %d", c.RequestCount())
	}

	recent := c.GetRecentRequests(10)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent, got %d", len(recent))
	}
}

func TestCollectorConcurrency(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BufferSize = 100
	c := NewCollector(cfg)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				c.Record(&RequestTrace{
					ID:             generateID(),
					Method:         "GET",
					Path:           "/test",
					RoutePattern:   "/test",
					ResponseStatus: 200,
					Latency:        time.Millisecond,
					StartTime:      time.Now(),
				})
			}
		}()
	}
	wg.Wait()

	if c.RequestCount() != 100 {
		t.Errorf("expected 100 (capped), got %d", c.RequestCount())
	}
}

func TestCollectorGetRequestByID(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCollector(cfg)

	trace := &RequestTrace{
		ID:             "test-id-123",
		Method:         "POST",
		Path:           "/api/users",
		RoutePattern:   "/api/users",
		ResponseStatus: 201,
		Latency:        5 * time.Millisecond,
		StartTime:      time.Now(),
	}
	c.Record(trace)

	found := c.GetRequestByID("test-id-123")
	if found == nil {
		t.Fatal("expected to find request by ID")
	}
	if found.Method != "POST" {
		t.Errorf("expected POST, got %s", found.Method)
	}

	notFound := c.GetRequestByID("nonexistent")
	if notFound != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestCollectorRouteMetrics(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCollector(cfg)

	for i := 0; i < 20; i++ {
		status := 200
		if i%5 == 0 {
			status = 500
		}
		c.Record(&RequestTrace{
			ID:             generateID(),
			Method:         "GET",
			Path:           "/api/users",
			RoutePattern:   "/api/users",
			ResponseStatus: status,
			Latency:        time.Duration(10+i) * time.Millisecond,
			StartTime:      time.Now(),
		})
	}

	routes := c.GetRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	rm := routes[0]
	if rm.TotalRequests != 20 {
		t.Errorf("expected 20 requests, got %d", rm.TotalRequests)
	}
	if rm.ErrorCount != 4 {
		t.Errorf("expected 4 errors, got %d", rm.ErrorCount)
	}
	if rm.ErrorRate() != 20.0 {
		t.Errorf("expected 20%% error rate, got %.1f%%", rm.ErrorRate())
	}
}

func TestRouteMetricsPercentiles(t *testing.T) {
	rm := NewRouteMetrics("/test", "GET", 10000)
	for i := 1; i <= 100; i++ {
		rm.Record(&RequestTrace{
			Latency:        time.Duration(i) * time.Millisecond,
			ResponseStatus: 200,
			StartTime:      time.Now(),
		})
	}

	p50 := rm.P50()
	if p50 < 49*time.Millisecond || p50 > 51*time.Millisecond {
		t.Errorf("P50 expected ~50ms, got %v", p50)
	}

	p95 := rm.P95()
	if p95 < 94*time.Millisecond || p95 > 96*time.Millisecond {
		t.Errorf("P95 expected ~95ms, got %v", p95)
	}

	p99 := rm.P99()
	if p99 < 98*time.Millisecond || p99 > 100*time.Millisecond {
		t.Errorf("P99 expected ~99ms, got %v", p99)
	}
}
