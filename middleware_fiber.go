package xrayhq

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
)

// FiberMiddleware returns a Fiber middleware that captures request data.
func FiberMiddleware() fiber.Handler {
	if defaultCollector == nil {
		Init()
	}
	return func(c *fiber.Ctx) error {
		cfg := defaultConfig

		// Sampling
		if cfg.SamplingRate < 1.0 {
			b := make([]byte, 1)
			_, _ = rand.Read(b)
			if float64(b[0])/255.0 > cfg.SamplingRate {
				return c.Next()
			}
		}

		start := time.Now()

		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)
		goroutinesBefore := runtime.NumGoroutine()

		// Capture request headers
		reqHeaders := make(map[string]string)
		if cfg.CaptureHeaders {
			c.Request().Header.VisitAll(func(k, v []byte) {
				reqHeaders[string(k)] = string(v)
			})
		}

		var reqBody []byte
		if cfg.CaptureBody {
			reqBody = make([]byte, len(c.Body()))
			copy(reqBody, c.Body())
		}

		idBytes := make([]byte, 16)
		_, _ = rand.Read(idBytes)

		trace := &RequestTrace{
			ID:               fmt.Sprintf("%x", idBytes),
			Method:           c.Method(),
			Path:             c.Path(),
			RoutePattern:     c.Route().Path,
			QueryParams:      string(c.Request().URI().QueryString()),
			RequestHeaders:   reqHeaders,
			RequestBody:      reqBody,
			RequestSize:      int64(len(c.Body())),
			ClientIP:         c.IP(),
			UserAgent:        c.Get("User-Agent"),
			StartTime:        start,
			GoroutinesBefore: goroutinesBefore,
			MemAllocBefore:   memBefore.TotalAlloc,
			DBQueries:        make([]DBQuery, 0),
			ExternalCalls:    make([]ExternalCall, 0),
			RedisOps:         make([]RedisOp, 0),
			MongoOps:         make([]MongoOp, 0),
		}

		// Store trace in Fiber locals for access by handlers
		c.Locals("xrayhq-trace", trace)

		var handlerErr error
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					trace.Panicked = true
					trace.PanicValue = rec
					buf := make([]byte, 4096)
					n := runtime.Stack(buf, false)
					trace.PanicStack = string(buf[:n])
					c.Status(500).SendString("Internal Server Error")
				}
			}()
			handlerErr = c.Next()
		}()

		end := time.Now()
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		trace.EndTime = end
		trace.Latency = end.Sub(start)
		trace.TTFB = trace.Latency // Fiber doesn't expose TTFB easily
		trace.HandlerTime = trace.Latency
		trace.ResponseStatus = c.Response().StatusCode()
		trace.ResponseSize = int64(len(c.Response().Body()))
		trace.GoroutinesAfter = runtime.NumGoroutine()
		trace.MemAllocAfter = memAfter.TotalAlloc

		if cfg.CaptureBody {
			body := c.Response().Body()
			trace.ResponseBody = make([]byte, len(body))
			copy(trace.ResponseBody, body)
		}

		if cfg.CaptureHeaders {
			respHeaders := make(map[string]string)
			c.Response().Header.VisitAll(func(k, v []byte) {
				respHeaders[string(k)] = string(v)
			})
			trace.ResponseHeaders = respHeaders
		}

		defaultCollector.Record(trace)

		return handlerErr
	}
}

// FiberTraceFromContext retrieves the trace from Fiber locals.
func FiberTraceFromContext(c *fiber.Ctx) *RequestTrace {
	if t, ok := c.Locals("xrayhq-trace").(*RequestTrace); ok {
		return t
	}
	return nil
}

// FiberAddDBQuery adds a DB query to the Fiber request trace.
func FiberAddDBQuery(c *fiber.Ctx, q DBQuery) {
	if t := FiberTraceFromContext(c); t != nil {
		t.DBQueries = append(t.DBQueries, q)
		t.TotalDBTime += q.Duration
	}
}

