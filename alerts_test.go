package xrayhq

import (
	"testing"
	"time"
)

func TestAlertNPlusOne(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NPlusOneThreshold = 3
	c := NewCollector(cfg)
	engine := NewAlertEngine(c, cfg)

	trace := &RequestTrace{
		ID:           "test-1",
		Method:       "GET",
		Path:         "/api/orders",
		RoutePattern: "/api/orders",
		DBQueries: []DBQuery{
			{Query: "SELECT * FROM items WHERE order_id = 1"},
			{Query: "SELECT * FROM items WHERE order_id = 2"},
			{Query: "SELECT * FROM items WHERE order_id = 3"},
			{Query: "SELECT * FROM items WHERE order_id = 4"},
		},
	}

	engine.Evaluate(trace)

	if len(trace.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(trace.Alerts))
	}
	if trace.Alerts[0].Type != "n_plus_one" {
		t.Errorf("expected n_plus_one alert, got %s", trace.Alerts[0].Type)
	}
}

func TestAlertSlowQuery(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SlowQueryThreshold = 100 * time.Millisecond
	c := NewCollector(cfg)
	engine := NewAlertEngine(c, cfg)

	trace := &RequestTrace{
		ID:           "test-2",
		Method:       "GET",
		RoutePattern: "/test",
		DBQueries: []DBQuery{
			{Query: "SELECT * FROM big_table", Duration: 200 * time.Millisecond},
		},
	}

	engine.Evaluate(trace)

	found := false
	for _, a := range trace.Alerts {
		if a.Type == "slow_query" {
			found = true
		}
	}
	if !found {
		t.Error("expected slow_query alert")
	}
}

func TestAlertPanic(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCollector(cfg)
	engine := NewAlertEngine(c, cfg)

	trace := &RequestTrace{
		ID:           "test-3",
		Method:       "GET",
		Path:         "/crash",
		RoutePattern: "/crash",
		Panicked:     true,
		PanicValue:   "null pointer",
	}

	engine.Evaluate(trace)

	found := false
	for _, a := range trace.Alerts {
		if a.Type == "panic" {
			found = true
			if a.Severity != SeverityCritical {
				t.Errorf("expected critical severity for panic")
			}
		}
	}
	if !found {
		t.Error("expected panic alert")
	}
}

func TestAlertMemorySpike(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MemorySpikeBytes = 1000
	c := NewCollector(cfg)
	engine := NewAlertEngine(c, cfg)

	trace := &RequestTrace{
		ID:             "test-4",
		Method:         "GET",
		Path:           "/heavy",
		RoutePattern:   "/heavy",
		MemAllocBefore: 1000,
		MemAllocAfter:  5000,
	}

	engine.Evaluate(trace)

	found := false
	for _, a := range trace.Alerts {
		if a.Type == "memory_spike" {
			found = true
		}
	}
	if !found {
		t.Error("expected memory_spike alert")
	}
}

func TestNoFalseAlerts(t *testing.T) {
	cfg := DefaultConfig()
	c := NewCollector(cfg)
	engine := NewAlertEngine(c, cfg)

	trace := &RequestTrace{
		ID:             "test-5",
		Method:         "GET",
		Path:           "/healthy",
		RoutePattern:   "/healthy",
		ResponseStatus: 200,
		Latency:        5 * time.Millisecond,
		MemAllocBefore: 1000,
		MemAllocAfter:  1100,
	}

	engine.Evaluate(trace)

	if len(trace.Alerts) != 0 {
		t.Errorf("expected 0 alerts for healthy request, got %d", len(trace.Alerts))
	}
}
