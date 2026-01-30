package xrayhq

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"
)

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// coreMiddleware is the shared middleware logic used by all framework adapters.
func coreMiddleware(collector *Collector, cfg *Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sampling
		if cfg.SamplingRate < 1.0 {
			b := make([]byte, 1)
			_, _ = rand.Read(b)
			if float64(b[0])/255.0 > cfg.SamplingRate {
				next.ServeHTTP(w, r)
				return
			}
		}

		start := time.Now()

		// Runtime stats before
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)
		goroutinesBefore := runtime.NumGoroutine()

		// Read request body if configured
		var reqBody []byte
		if cfg.CaptureBody && r.Body != nil {
			var err error
			reqBody, err = io.ReadAll(r.Body)
			if err != nil {
				reqBody = nil
			}
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// Capture request headers
		reqHeaders := make(map[string]string)
		if cfg.CaptureHeaders {
			for k, v := range r.Header {
				reqHeaders[k] = strings.Join(v, ", ")
			}
		}

		// Create trace and attach to context
		trace := &RequestTrace{
			ID:               generateID(),
			Method:           r.Method,
			Path:             r.URL.Path,
			QueryParams:      r.URL.RawQuery,
			RequestHeaders:   reqHeaders,
			RequestBody:      reqBody,
			RequestSize:      r.ContentLength,
			ClientIP:         clientIP(r),
			UserAgent:        r.UserAgent(),
			StartTime:        start,
			GoroutinesBefore: goroutinesBefore,
			MemAllocBefore:   memBefore.TotalAlloc,
			DBQueries:        make([]DBQuery, 0),
			ExternalCalls:    make([]ExternalCall, 0),
			RedisOps:         make([]RedisOp, 0),
			MongoOps:         make([]MongoOp, 0),
		}

		ctx := withTrace(r.Context(), trace)
		r = r.WithContext(ctx)

		// Wrap response writer
		rw := newResponseWriter(w, start, cfg.CaptureBody)

		// Panic recovery
		defer func() {
			if rec := recover(); rec != nil {
				trace.Panicked = true
				trace.PanicValue = rec
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				trace.PanicStack = string(buf[:n])

				rw.WriteHeader(http.StatusInternalServerError)
				rw.Write([]byte("Internal Server Error"))
			}

			// Finalize trace
			end := time.Now()
			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)

			trace.EndTime = end
			trace.Latency = end.Sub(start)
			trace.TTFB = rw.ttfb
			trace.HandlerTime = trace.Latency
			trace.ResponseStatus = rw.statusCode
			trace.ResponseSize = rw.size
			trace.GoroutinesAfter = runtime.NumGoroutine()
			trace.MemAllocAfter = memAfter.TotalAlloc

			if cfg.CaptureBody {
				trace.ResponseBody = rw.body.Bytes()
			}

			if cfg.CaptureHeaders {
				respHeaders := make(map[string]string)
				for k, v := range rw.Header() {
					respHeaders[k] = strings.Join(v, ", ")
				}
				trace.ResponseHeaders = respHeaders
			}

			// Record to collector
			collector.Record(trace)
		}()

		next.ServeHTTP(rw, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
