package xrayhq

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ChiMiddleware is a Chi-compatible middleware that captures request data.
func ChiMiddleware(next http.Handler) http.Handler {
	if defaultCollector == nil {
		Init()
	}
	wrapped := coreMiddleware(defaultCollector, defaultConfig, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		// Set route pattern from chi's route context
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if t := TraceFromContext(r.Context()); t != nil {
				t.RoutePattern = rctx.RoutePattern()
				if t.RoutePattern == "" {
					t.RoutePattern = r.URL.Path
				}
			}
		}
	}))
	return wrapped
}
