package xrayhq

import (
	"fmt"
	"strings"
	"time"
)

type AlertEngine struct {
	collector *Collector
	config    *Config
}

func NewAlertEngine(collector *Collector, config *Config) *AlertEngine {
	return &AlertEngine{collector: collector, config: config}
}

func (e *AlertEngine) Evaluate(trace *RequestTrace) {
	e.checkNPlusOne(trace)
	e.checkSlowQueries(trace)
	e.checkSlowRoute(trace)
	e.checkHighErrorRate(trace)
	e.checkMemorySpike(trace)
	e.checkPanic(trace)
}

func (e *AlertEngine) checkNPlusOne(trace *RequestTrace) {
	if len(trace.DBQueries) == 0 {
		return
	}
	patterns := make(map[string]int)
	for _, q := range trace.DBQueries {
		pattern := queryPattern(q.Query)
		patterns[pattern]++
	}
	for pattern, count := range patterns {
		if count > e.config.NPlusOneThreshold {
			alert := Alert{
				ID:           generateID(),
				Type:         "n_plus_one",
				Message:      fmt.Sprintf("N+1 query detected: pattern %q executed %d times", pattern, count),
				Severity:     SeverityWarning,
				RoutePattern: trace.RoutePattern,
				RequestID:    trace.ID,
				Timestamp:    time.Now(),
				Details:      map[string]interface{}{"pattern": pattern, "count": count},
			}
			trace.Alerts = append(trace.Alerts, alert)
			e.collector.AddAlert(alert)
		}
	}
}

func (e *AlertEngine) checkSlowQueries(trace *RequestTrace) {
	for _, q := range trace.DBQueries {
		if q.Duration > e.config.SlowQueryThreshold {
			alert := Alert{
				ID:           generateID(),
				Type:         "slow_query",
				Message:      fmt.Sprintf("Slow query: %s took %v", truncate(q.Query, 100), q.Duration),
				Severity:     SeverityWarning,
				RoutePattern: trace.RoutePattern,
				RequestID:    trace.ID,
				Timestamp:    time.Now(),
				Details:      map[string]interface{}{"query": q.Query, "duration_ms": q.Duration.Milliseconds()},
			}
			trace.Alerts = append(trace.Alerts, alert)
			e.collector.AddAlert(alert)
		}
	}
}

func (e *AlertEngine) checkSlowRoute(trace *RequestTrace) {
	rm := e.collector.GetRoute(trace.Method, trace.RoutePattern)
	if rm == nil || rm.TotalRequests < 10 {
		return
	}
	if rm.P95() > e.config.SlowRouteP95Threshold {
		alert := Alert{
			ID:           generateID(),
			Type:         "slow_route",
			Message:      fmt.Sprintf("Slow route: %s %s P95=%v", trace.Method, trace.RoutePattern, rm.P95()),
			Severity:     SeverityWarning,
			RoutePattern: trace.RoutePattern,
			RequestID:    trace.ID,
			Timestamp:    time.Now(),
			Details:      map[string]interface{}{"p95_ms": rm.P95().Milliseconds()},
		}
		trace.Alerts = append(trace.Alerts, alert)
		e.collector.AddAlert(alert)
	}
}

func (e *AlertEngine) checkHighErrorRate(trace *RequestTrace) {
	rm := e.collector.GetRoute(trace.Method, trace.RoutePattern)
	if rm == nil || rm.TotalRequests < 10 {
		return
	}
	if rm.ErrorRate() > e.config.HighErrorRatePercent {
		alert := Alert{
			ID:           generateID(),
			Type:         "high_error_rate",
			Message:      fmt.Sprintf("High error rate: %s %s at %.1f%%", trace.Method, trace.RoutePattern, rm.ErrorRate()),
			Severity:     SeverityCritical,
			RoutePattern: trace.RoutePattern,
			RequestID:    trace.ID,
			Timestamp:    time.Now(),
			Details:      map[string]interface{}{"error_rate": rm.ErrorRate()},
		}
		trace.Alerts = append(trace.Alerts, alert)
		e.collector.AddAlert(alert)
	}
}

func (e *AlertEngine) checkMemorySpike(trace *RequestTrace) {
	if trace.MemAllocAfter > trace.MemAllocBefore {
		delta := trace.MemAllocAfter - trace.MemAllocBefore
		if delta > e.config.MemorySpikeBytes {
			alert := Alert{
				ID:           generateID(),
				Type:         "memory_spike",
				Message:      fmt.Sprintf("Memory spike: %s %s allocated %s", trace.Method, trace.Path, formatBytes(delta)),
				Severity:     SeverityWarning,
				RoutePattern: trace.RoutePattern,
				RequestID:    trace.ID,
				Timestamp:    time.Now(),
				Details:      map[string]interface{}{"bytes_allocated": delta},
			}
			trace.Alerts = append(trace.Alerts, alert)
			e.collector.AddAlert(alert)
		}
	}
}

func (e *AlertEngine) checkPanic(trace *RequestTrace) {
	if trace.Panicked {
		alert := Alert{
			ID:           generateID(),
			Type:         "panic",
			Message:      fmt.Sprintf("Panic in %s %s: %v", trace.Method, trace.Path, trace.PanicValue),
			Severity:     SeverityCritical,
			RoutePattern: trace.RoutePattern,
			RequestID:    trace.ID,
			Timestamp:    time.Now(),
			Details:      map[string]interface{}{"panic_value": fmt.Sprintf("%v", trace.PanicValue)},
		}
		trace.Alerts = append(trace.Alerts, alert)
		e.collector.AddAlert(alert)
	}
}

func queryPattern(query string) string {
	query = strings.TrimSpace(query)
	parts := strings.Fields(query)
	if len(parts) < 2 {
		return query
	}
	// Return first word (SELECT/INSERT/etc) + table-like word
	op := strings.ToUpper(parts[0])
	table := ""
	for i, p := range parts {
		upper := strings.ToUpper(p)
		if upper == "FROM" || upper == "INTO" || upper == "UPDATE" || upper == "TABLE" {
			if i+1 < len(parts) {
				table = parts[i+1]
				break
			}
		}
	}
	if table == "" && len(parts) > 1 {
		table = parts[1]
	}
	return op + " " + table
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
