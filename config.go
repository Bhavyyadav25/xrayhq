package xrayhq

import "time"

type Mode string

const (
	ModeDev  Mode = "dev"
	ModeProd Mode = "prod"
)

type Config struct {
	Port          string
	BufferSize    int
	Mode          Mode
	SamplingRate  float64
	CaptureBody   bool
	CaptureHeaders bool
	BasicAuthUser string
	BasicAuthPass string

	SlowQueryThreshold    time.Duration
	SlowRouteP95Threshold time.Duration
	HighErrorRatePercent  float64
	NPlusOneThreshold     int
	MemorySpikeBytes      uint64
	LatencyCap            int
}

func DefaultConfig() *Config {
	return &Config{
		Port:                  ":9090",
		BufferSize:            1000,
		Mode:                  ModeDev,
		SamplingRate:          1.0,
		CaptureBody:          true,
		CaptureHeaders:       true,
		SlowQueryThreshold:    500 * time.Millisecond,
		SlowRouteP95Threshold: 2 * time.Second,
		HighErrorRatePercent:  10.0,
		NPlusOneThreshold:     5,
		MemorySpikeBytes:      10 * 1024 * 1024, // 10MB
		LatencyCap:            10000,
	}
}

type Option func(*Config)

func WithPort(port string) Option { return func(c *Config) { c.Port = port } }
func WithBufferSize(size int) Option { return func(c *Config) { c.BufferSize = size } }
func WithMode(mode Mode) Option { return func(c *Config) { c.Mode = mode } }
func WithSamplingRate(rate float64) Option { return func(c *Config) { c.SamplingRate = rate } }
func WithCaptureBody(capture bool) Option { return func(c *Config) { c.CaptureBody = capture } }
func WithCaptureHeaders(capture bool) Option { return func(c *Config) { c.CaptureHeaders = capture } }
func WithBasicAuth(user, pass string) Option {
	return func(c *Config) { c.BasicAuthUser = user; c.BasicAuthPass = pass }
}
func WithSlowQueryThreshold(d time.Duration) Option { return func(c *Config) { c.SlowQueryThreshold = d } }
func WithSlowRouteThreshold(d time.Duration) Option { return func(c *Config) { c.SlowRouteP95Threshold = d } }
func WithHighErrorRate(pct float64) Option { return func(c *Config) { c.HighErrorRatePercent = pct } }
func WithNPlusOneThreshold(n int) Option { return func(c *Config) { c.NPlusOneThreshold = n } }
func WithMemorySpikeThreshold(bytes uint64) Option { return func(c *Config) { c.MemorySpikeBytes = bytes } }
func WithLatencyCap(n int) Option                  { return func(c *Config) { c.LatencyCap = n } }
