package xrayhq

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GinMiddleware returns a Gin middleware that captures request data.
func GinMiddleware() gin.HandlerFunc {
	if defaultCollector == nil {
		Init()
	}
	return func(c *gin.Context) {
		// Create a wrapper handler that calls gin's Next
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Update gin's request with our context
			c.Request = r
			c.Next()
		})

		// Run through core middleware
		wrapped := coreMiddleware(defaultCollector, defaultConfig, handler)
		wrapped.ServeHTTP(c.Writer, c.Request)

		// Set route pattern after handler runs
		if t := TraceFromContext(c.Request.Context()); t != nil {
			t.RoutePattern = c.FullPath()
			if t.RoutePattern == "" {
				t.RoutePattern = c.Request.URL.Path
			}
		}
	}
}
