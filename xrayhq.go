package xrayhq

import (
	"log"
	"net/http"
)

var defaultCollector *Collector
var defaultConfig *Config

// Init initializes xrayhq with the given options and starts the dashboard server.
func Init(opts ...Option) {
	cfg := DefaultConfig()
	for _, o := range opts {
		o(cfg)
	}
	defaultConfig = cfg
	defaultCollector = NewCollector(cfg)

	// Start dashboard server in a separate goroutine
	go func() {
		srv := NewDashboardServer(defaultCollector, cfg)
		log.Printf("[xrayhq] Dashboard available at http://localhost%s\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[xrayhq] Dashboard server error: %v\n", err)
		}
	}()
}

// Wrap wraps an http.Handler with xrayhq middleware.
func Wrap(handler http.Handler) http.Handler {
	if defaultCollector == nil {
		Init()
	}
	return coreMiddleware(defaultCollector, defaultConfig, handler)
}

// WrapFunc wraps an http.HandlerFunc with xrayhq middleware.
func WrapFunc(handler http.HandlerFunc) http.Handler {
	return Wrap(handler)
}

// GetCollector returns the default collector (for advanced usage).
func GetCollector() *Collector {
	return defaultCollector
}

// GetConfig returns the default config (for advanced usage).
func GetConfig() *Config {
	return defaultConfig
}

// SetRoutePattern sets the route pattern on the trace in context.
// Framework adapters call this to record the matched route pattern.
func SetRoutePattern(r *http.Request, pattern string) {
	if t := TraceFromContext(r.Context()); t != nil {
		t.RoutePattern = pattern
	}
}
