package xrayhq

import (
	"net/http"
	"time"
)

type instrumentedTransport struct {
	base http.RoundTripper
}

// WrapHTTPClient returns a new http.Client with an instrumented transport
// that records external calls to the request trace.
func WrapHTTPClient(client *http.Client) *http.Client {
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	return &http.Client{
		Transport:     &instrumentedTransport{base: base},
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       client.Timeout,
	}
}

func (t *instrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	duration := time.Since(start)

	call := ExternalCall{
		URL:       req.URL.String(),
		Method:    req.Method,
		Duration:  duration,
		Timestamp: start,
	}
	if err != nil {
		call.Error = err.Error()
	}
	if resp != nil {
		call.StatusCode = resp.StatusCode
	}

	AddExternalCall(req.Context(), call)
	return resp, err
}
