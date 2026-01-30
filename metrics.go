package xrayhq

import (
	"sort"
	"time"
)

type RouteMetrics struct {
	Pattern       string
	Method        string
	TotalRequests int64
	ErrorCount    int64 // 5xx responses
	TotalLatency  time.Duration
	Latencies     []time.Duration // for percentile calculation - capped at 10000

	StatusCodes   map[int]int64
	AvgDBQueries  float64
	totalDBQueries int64

	MinLatency time.Duration
	MaxLatency time.Duration

	LastRequestTime time.Time
	latencyCap      int
}

func NewRouteMetrics(pattern, method string, latencyCap int) *RouteMetrics {
	if latencyCap <= 0 {
		latencyCap = 10000
	}
	return &RouteMetrics{
		Pattern:     pattern,
		Method:      method,
		StatusCodes: make(map[int]int64),
		Latencies:   make([]time.Duration, 0, 1000),
		MinLatency:  time.Duration(1<<63 - 1),
		latencyCap:  latencyCap,
	}
}

func (rm *RouteMetrics) Record(trace *RequestTrace) {
	rm.TotalRequests++
	rm.TotalLatency += trace.Latency
	rm.LastRequestTime = trace.StartTime

	if trace.Latency < rm.MinLatency {
		rm.MinLatency = trace.Latency
	}
	if trace.Latency > rm.MaxLatency {
		rm.MaxLatency = trace.Latency
	}

	if trace.ResponseStatus >= 500 {
		rm.ErrorCount++
	}

	rm.StatusCodes[trace.ResponseStatus]++
	rm.totalDBQueries += int64(len(trace.DBQueries))
	if rm.TotalRequests > 0 {
		rm.AvgDBQueries = float64(rm.totalDBQueries) / float64(rm.TotalRequests)
	}

	// Keep latencies for percentile calc
	if len(rm.Latencies) < rm.latencyCap {
		rm.Latencies = append(rm.Latencies, trace.Latency)
	}
}

func (rm *RouteMetrics) AvgLatency() time.Duration {
	if rm.TotalRequests == 0 {
		return 0
	}
	return time.Duration(int64(rm.TotalLatency) / rm.TotalRequests)
}

func (rm *RouteMetrics) ErrorRate() float64 {
	if rm.TotalRequests == 0 {
		return 0
	}
	return float64(rm.ErrorCount) / float64(rm.TotalRequests) * 100
}

func (rm *RouteMetrics) Percentile(p float64) time.Duration {
	if len(rm.Latencies) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(rm.Latencies))
	copy(sorted, rm.Latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(float64(len(sorted)-1) * p / 100.0)
	return sorted[idx]
}

func (rm *RouteMetrics) P50() time.Duration  { return rm.Percentile(50) }
func (rm *RouteMetrics) P95() time.Duration  { return rm.Percentile(95) }
func (rm *RouteMetrics) P99() time.Duration  { return rm.Percentile(99) }

func (rm *RouteMetrics) Status() string {
	errRate := rm.ErrorRate()
	p95 := rm.P95()
	if errRate > 10 || p95 > 2*time.Second {
		return "critical"
	}
	if errRate > 5 || p95 > 1*time.Second {
		return "warning"
	}
	return "healthy"
}

func (rm *RouteMetrics) Snapshot() *RouteMetrics {
	snap := &RouteMetrics{
		Pattern:        rm.Pattern,
		Method:         rm.Method,
		TotalRequests:  rm.TotalRequests,
		ErrorCount:     rm.ErrorCount,
		TotalLatency:   rm.TotalLatency,
		StatusCodes:    make(map[int]int64),
		AvgDBQueries:   rm.AvgDBQueries,
		totalDBQueries: rm.totalDBQueries,
		MinLatency:      rm.MinLatency,
		MaxLatency:      rm.MaxLatency,
		LastRequestTime: rm.LastRequestTime,
		latencyCap:      rm.latencyCap,
	}
	for k, v := range rm.StatusCodes {
		snap.StatusCodes[k] = v
	}
	snap.Latencies = make([]time.Duration, len(rm.Latencies))
	copy(snap.Latencies, rm.Latencies)
	return snap
}
