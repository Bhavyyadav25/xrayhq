package xrayhq

import (
	"bytes"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	body        bytes.Buffer
	size        int64
	wroteHeader bool
	ttfb        time.Duration
	startTime   time.Time
	captureBody bool
}

func newResponseWriter(w http.ResponseWriter, start time.Time, captureBody bool) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		startTime:      start,
		captureBody:    captureBody,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ttfb = time.Since(rw.startTime)
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)
	if rw.captureBody {
		rw.body.Write(b[:n])
	}
	return n, err
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap supports http.ResponseController
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}
