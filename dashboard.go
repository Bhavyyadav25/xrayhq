package xrayhq

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"
)

//go:embed dashboard/templates/*.html dashboard/static/*
var dashboardFS embed.FS

var funcMap = template.FuncMap{
	"formatDuration": func(d time.Duration) string {
		if d < time.Microsecond {
			return fmt.Sprintf("%dns", d.Nanoseconds())
		}
		if d < time.Millisecond {
			return fmt.Sprintf("%.1fμs", float64(d.Nanoseconds())/1000)
		}
		if d < time.Second {
			return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
		}
		return fmt.Sprintf("%.2fs", d.Seconds())
	},
	"formatTime": func(t time.Time) string {
		return t.Format("15:04:05.000")
	},
	"formatDateTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	},
	"formatBytes": func(b int64) string {
		const unit = 1024
		if b < unit {
			return fmt.Sprintf("%d B", b)
		}
		div, exp := int64(unit), 0
		for n := b / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
	},
	"formatBytesUint": func(b uint64) string {
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
	},
	"statusClass": func(code int) string {
		switch {
		case code >= 500:
			return "status-error"
		case code >= 400:
			return "status-warn"
		case code >= 300:
			return "status-info"
		default:
			return "status-ok"
		}
	},
	"healthClass": func(status string) string {
		switch status {
		case "critical":
			return "health-critical"
		case "warning":
			return "health-warning"
		default:
			return "health-ok"
		}
	},
	"severityClass": func(s Severity) string {
		switch s {
		case SeverityCritical:
			return "severity-critical"
		case SeverityWarning:
			return "severity-warning"
		default:
			return "severity-info"
		}
	},
	"formatFloat": func(f float64) string {
		return fmt.Sprintf("%.1f", f)
	},
	"formatPercent": func(f float64) string {
		return fmt.Sprintf("%.1f%%", f)
	},
	"json": func(v interface{}) (template.JS, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return template.JS(b), nil
	},
	"sub": func(a, b int) int { return a - b },
	"add": func(a, b int) int { return a + b },
	"mul": func(a, b int) int { return a * b },
	"dict": func(values ...interface{}) map[string]interface{} {
		d := make(map[string]interface{})
		for i := 0; i < len(values)-1; i += 2 {
			d[values[i].(string)] = values[i+1]
		}
		return d
	},
	"memDelta": func(before, after uint64) string {
		if after >= before {
			delta := after - before
			return formatBytes(delta)
		}
		return "0 B"
	},
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "..."
	},
	"timelinePercent": func(opStart, reqStart time.Time, totalDuration time.Duration) float64 {
		if totalDuration == 0 {
			return 0
		}
		offset := opStart.Sub(reqStart)
		return float64(offset) / float64(totalDuration) * 100
	},
	"durationPercent": func(d, total time.Duration) float64 {
		if total == 0 {
			return 0
		}
		return float64(d) / float64(total) * 100
	},
	"uptimeFormat": func(d time.Duration) string {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		seconds := int(d.Seconds()) % 60
		if hours > 0 {
			return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
		}
		if minutes > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%ds", seconds)
	},
}

type DashboardServer struct {
	collector *Collector
	config    *Config
	templates *template.Template
	mux       *http.ServeMux
}

func NewDashboardServer(collector *Collector, config *Config) *http.Server {
	ds := &DashboardServer{
		collector: collector,
		config:    config,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(dashboardFS, "dashboard/templates/*.html")
	if err != nil {
		panic(fmt.Sprintf("xrayhq: failed to parse templates: %v", err))
	}
	ds.templates = tmpl

	mux := http.NewServeMux()
	ds.mux = mux

	// Static files
	mux.Handle("/static/", http.FileServer(http.FS(dashboardFS)))
	// Wrap with Handle for proper path
	mux.Handle("/dashboard/static/", http.FileServer(http.FS(dashboardFS)))

	// Pages
	mux.HandleFunc("/", ds.handleRoutes)
	mux.HandleFunc("/route/", ds.handleRouteDetail)
	mux.HandleFunc("/request/", ds.handleRequestDetail)
	mux.HandleFunc("/live", ds.handleLiveTail)
	mux.HandleFunc("/alerts", ds.handleAlerts)
	mux.HandleFunc("/system", ds.handleSystem)

	// API endpoints
	mux.HandleFunc("/events", ds.handleSSE)
	mux.HandleFunc("/xrayhq/export", ds.handleExport)

	var handler http.Handler = mux
	if config.BasicAuthUser != "" && config.BasicAuthPass != "" {
		handler = basicAuth(config.BasicAuthUser, config.BasicAuthPass, mux)
	}

	return &http.Server{
		Addr:    config.Port,
		Handler: handler,
	}
}

func basicAuth(user, pass string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="xrayhq"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (ds *DashboardServer) handleRoutes(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "hits"
	}

	routes := ds.collector.GetRoutes()

	switch sortBy {
	case "route":
		sort.Slice(routes, func(i, j int) bool { return routes[i].Pattern < routes[j].Pattern })
	case "method":
		sort.Slice(routes, func(i, j int) bool { return routes[i].Method < routes[j].Method })
	case "hits":
		sort.Slice(routes, func(i, j int) bool { return routes[i].TotalRequests > routes[j].TotalRequests })
	case "avg":
		sort.Slice(routes, func(i, j int) bool { return routes[i].AvgLatency() > routes[j].AvgLatency() })
	case "p95":
		sort.Slice(routes, func(i, j int) bool { return routes[i].P95() > routes[j].P95() })
	case "p99":
		sort.Slice(routes, func(i, j int) bool { return routes[i].P99() > routes[j].P99() })
	case "errors":
		sort.Slice(routes, func(i, j int) bool { return routes[i].ErrorRate() > routes[j].ErrorRate() })
	}

	alerts := ds.collector.GetAlerts()
	activeAlerts := len(alerts)

	data := map[string]interface{}{
		"Routes":       routes,
		"Sort":         sortBy,
		"ActiveAlerts": activeAlerts,
		"RequestCount": ds.collector.RequestCount(),
		"Page":         "routes",
	}
	ds.render(w, "routes.html", data)
}

func (ds *DashboardServer) handleRouteDetail(w http.ResponseWriter, r *http.Request) {
	// Parse /route/METHOD/pattern
	path := strings.TrimPrefix(r.URL.Path, "/route/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	method := parts[0]
	pattern := "/" + parts[1]

	rm := ds.collector.GetRoute(method, pattern)
	if rm == nil {
		http.NotFound(w, r)
		return
	}

	requests := ds.collector.GetRequestsForRoute(method, pattern, 50)

	// Find slowest requests
	slowest := make([]*RequestTrace, len(requests))
	copy(slowest, requests)
	sort.Slice(slowest, func(i, j int) bool { return slowest[i].Latency > slowest[j].Latency })
	if len(slowest) > 10 {
		slowest = slowest[:10]
	}

	// Status code distribution
	statusDist := make([]map[string]interface{}, 0)
	for code, count := range rm.StatusCodes {
		statusDist = append(statusDist, map[string]interface{}{
			"code":  code,
			"count": count,
		})
	}
	sort.Slice(statusDist, func(i, j int) bool {
		return statusDist[i]["code"].(int) < statusDist[j]["code"].(int)
	})

	// Latency distribution for histogram
	latencyBuckets := computeLatencyBuckets(rm.Latencies)

	data := map[string]interface{}{
		"Route":           rm,
		"Requests":        requests,
		"SlowestRequests": slowest,
		"StatusDist":      statusDist,
		"LatencyBuckets":  latencyBuckets,
		"Page":            "route_detail",
	}
	ds.render(w, "route_detail.html", data)
}

func computeLatencyBuckets(latencies []time.Duration) []map[string]interface{} {
	if len(latencies) == 0 {
		return nil
	}
	bucketLabels := []string{"<1ms", "1-5ms", "5-10ms", "10-50ms", "50-100ms", "100-500ms", "500ms-1s", ">1s"}
	bucketThresholds := []time.Duration{
		time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}
	counts := make([]int, len(bucketLabels))
	for _, l := range latencies {
		placed := false
		for i, threshold := range bucketThresholds {
			if l < threshold {
				counts[i]++
				placed = true
				break
			}
		}
		if !placed {
			counts[len(counts)-1]++
		}
	}
	result := make([]map[string]interface{}, len(bucketLabels))
	for i, label := range bucketLabels {
		result[i] = map[string]interface{}{
			"label": label,
			"count": counts[i],
		}
	}
	return result
}

func (ds *DashboardServer) handleRequestDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/request/")
	trace := ds.collector.GetRequestByID(id)
	if trace == nil {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Trace": trace,
		"Page":  "request_detail",
	}
	ds.render(w, "request_detail.html", data)
}

func (ds *DashboardServer) handleLiveTail(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Page": "live",
	}
	ds.render(w, "live_tail.html", data)
}

func (ds *DashboardServer) handleAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := ds.collector.GetAlerts()
	// Reverse to show newest first
	for i, j := 0, len(alerts)-1; i < j; i, j = i+1, j-1 {
		alerts[i], alerts[j] = alerts[j], alerts[i]
	}

	data := map[string]interface{}{
		"Alerts": alerts,
		"Page":   "alerts",
	}
	ds.render(w, "alerts.html", data)
}

func (ds *DashboardServer) handleSystem(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	data := map[string]interface{}{
		"Goroutines":    runtime.NumGoroutine(),
		"MemAlloc":      mem.Alloc,
		"MemTotalAlloc": mem.TotalAlloc,
		"MemSys":        mem.Sys,
		"NumGC":         mem.NumGC,
		"LastGC":        time.Unix(0, int64(mem.LastGC)),
		"Uptime":        ds.collector.Uptime(),
		"RequestCount":  ds.collector.RequestCount(),
		"RouteCount":    len(ds.collector.GetRoutes()),
		"BufferSize":    ds.config.BufferSize,
		"Mode":          ds.config.Mode,
		"Page":          "system",
	}
	ds.render(w, "system.html", data)
}

func (ds *DashboardServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := ds.collector.SubscribeSSE()
	defer ds.collector.UnsubscribeSSE(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case trace, ok := <-ch:
			if !ok {
				return
			}
			data := map[string]interface{}{
				"id":          trace.ID,
				"method":      trace.Method,
				"path":        trace.Path,
				"status":      trace.ResponseStatus,
				"latency":     trace.Latency.Milliseconds(),
				"latencyFmt":  sseFormatDuration(trace.Latency),
				"dbQueries":   len(trace.DBQueries),
				"timestamp":   trace.StartTime.Format("15:04:05.000"),
				"statusClass": sseStatusClass(trace.ResponseStatus),
			}
			jsonData, err := json.Marshal(data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}

func (ds *DashboardServer) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ds.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

func sseFormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fμs", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func sseStatusClass(code int) string {
	switch {
	case code >= 500:
		return "status-error"
	case code >= 400:
		return "status-warn"
	case code >= 300:
		return "status-info"
	default:
		return "status-ok"
	}
}
