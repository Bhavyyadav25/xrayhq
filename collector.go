package xrayhq

import (
	"sort"
	"sync"
	"time"
)

type Collector struct {
	mu         sync.RWMutex
	buffer     []*RequestTrace
	bufferSize int
	head       int
	count      int

	routes     map[string]*RouteMetrics
	alerts     []Alert
	startTime  time.Time

	config      *Config
	alertEngine *AlertEngine
	sseClients  map[chan *RequestTrace]struct{}
	sseMu       sync.Mutex
}

func NewCollector(cfg *Config) *Collector {
	c := &Collector{
		buffer:     make([]*RequestTrace, cfg.BufferSize),
		bufferSize: cfg.BufferSize,
		routes:     make(map[string]*RouteMetrics),
		alerts:     make([]Alert, 0),
		startTime:  time.Now(),
		config:     cfg,
		sseClients: make(map[chan *RequestTrace]struct{}),
	}
	c.alertEngine = NewAlertEngine(c, cfg)
	return c
}

func (c *Collector) Record(trace *RequestTrace) {
	c.mu.Lock()
	c.buffer[c.head] = trace
	c.head = (c.head + 1) % c.bufferSize
	if c.count < c.bufferSize {
		c.count++
	}

	key := trace.Method + " " + trace.RoutePattern
	rm, ok := c.routes[key]
	if !ok {
		rm = NewRouteMetrics(trace.RoutePattern, trace.Method, c.config.LatencyCap)
		c.routes[key] = rm
	}
	rm.Record(trace)
	c.mu.Unlock()

	// Evaluate alert rules
	c.alertEngine.Evaluate(trace)

	// Notify SSE clients
	c.sseMu.Lock()
	for ch := range c.sseClients {
		select {
		case ch <- trace:
		default: // drop if slow consumer
		}
	}
	c.sseMu.Unlock()
}

func (c *Collector) AddAlert(a Alert) {
	c.mu.Lock()
	c.alerts = append(c.alerts, a)
	c.mu.Unlock()
}

func (c *Collector) GetAlerts() []Alert {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Alert, len(c.alerts))
	copy(out, c.alerts)
	return out
}

func (c *Collector) GetRecentRequests(limit int) []*RequestTrace {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > c.count {
		limit = c.count
	}

	result := make([]*RequestTrace, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (c.head - 1 - i + c.bufferSize) % c.bufferSize
		if c.buffer[idx] != nil {
			result = append(result, c.buffer[idx])
		}
	}
	return result
}

func (c *Collector) GetAllRequests() []*RequestTrace {
	return c.GetRecentRequests(c.count)
}

func (c *Collector) GetRequestByID(id string) *RequestTrace {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := 0; i < c.count; i++ {
		idx := (c.head - 1 - i + c.bufferSize) % c.bufferSize
		if c.buffer[idx] != nil && c.buffer[idx].ID == id {
			return c.buffer[idx]
		}
	}
	return nil
}

func (c *Collector) GetRoutes() []*RouteMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*RouteMetrics, 0, len(c.routes))
	for _, rm := range c.routes {
		result = append(result, rm.Snapshot())
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalRequests > result[j].TotalRequests
	})
	return result
}

func (c *Collector) GetRoute(method, pattern string) *RouteMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := method + " " + pattern
	if rm, ok := c.routes[key]; ok {
		return rm.Snapshot()
	}
	return nil
}

func (c *Collector) GetRequestsForRoute(method, pattern string, limit int) []*RequestTrace {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*RequestTrace, 0)
	for i := 0; i < c.count; i++ {
		idx := (c.head - 1 - i + c.bufferSize) % c.bufferSize
		t := c.buffer[idx]
		if t != nil && t.Method == method && t.RoutePattern == pattern {
			result = append(result, t)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

func (c *Collector) SubscribeSSE() chan *RequestTrace {
	ch := make(chan *RequestTrace, 64)
	c.sseMu.Lock()
	c.sseClients[ch] = struct{}{}
	c.sseMu.Unlock()
	return ch
}

func (c *Collector) UnsubscribeSSE(ch chan *RequestTrace) {
	c.sseMu.Lock()
	delete(c.sseClients, ch)
	c.sseMu.Unlock()
	close(ch)
}

func (c *Collector) Uptime() time.Duration {
	return time.Since(c.startTime)
}

func (c *Collector) RequestCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.count
}
