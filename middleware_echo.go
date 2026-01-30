package xrayhq

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// EchoMiddleware returns an Echo middleware that captures request data.
func EchoMiddleware() echo.MiddlewareFunc {
	if defaultCollector == nil {
		Init()
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var echoErr error

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.SetRequest(r)
				echoErr = next(c)
			})

			wrapped := coreMiddleware(defaultCollector, defaultConfig, handler)
			wrapped.ServeHTTP(c.Response().Writer, c.Request())

			// Set route pattern
			if t := TraceFromContext(c.Request().Context()); t != nil {
				t.RoutePattern = c.Path()
				if t.RoutePattern == "" {
					t.RoutePattern = c.Request().URL.Path
				}
			}

			return echoErr
		}
	}
}
