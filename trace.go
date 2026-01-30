package xrayhq

import (
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type RequestTrace struct {
	ID            string
	Method        string
	Path          string
	RoutePattern  string
	QueryParams   string
	RequestHeaders  map[string]string
	RequestBody     []byte
	ResponseStatus  int
	ResponseHeaders map[string]string
	ResponseBody    []byte
	RequestSize     int64
	ResponseSize    int64
	ClientIP        string
	UserAgent       string

	StartTime     time.Time
	EndTime       time.Time
	Latency       time.Duration
	TTFB          time.Duration
	HandlerTime   time.Duration

	GoroutinesBefore int
	GoroutinesAfter  int
	MemAllocBefore   uint64
	MemAllocAfter    uint64

	DBQueries      []DBQuery
	TotalDBTime    time.Duration
	ExternalCalls  []ExternalCall
	TotalExtTime   time.Duration
	RedisOps       []RedisOp
	TotalRedisTime time.Duration
	MongoOps       []MongoOp
	TotalMongoTime time.Duration

	Panicked   bool
	PanicValue interface{}
	PanicStack string

	Alerts []Alert
}

type DBQuery struct {
	Query       string
	Duration    time.Duration
	RowsAffected int64
	Error       string
	Timestamp   time.Time
}

type ExternalCall struct {
	URL        string
	Method     string
	StatusCode int
	Duration   time.Duration
	Error      string
	Timestamp  time.Time
}

type RedisOp struct {
	Command   string
	Key       string
	Duration  time.Duration
	Error     string
	Timestamp time.Time
}

type MongoOp struct {
	Collection string
	Operation  string
	Filter     string
	Duration   time.Duration
	Error      string
	Timestamp  time.Time
}

type Alert struct {
	ID        string
	Type      string
	Message   string
	Severity  Severity
	RoutePattern string
	RequestID    string
	Timestamp    time.Time
	Details      map[string]interface{}
}

type DBPoolStats struct {
	OpenConnections int
	IdleConnections int
	InUseConnections int
	WaitCount       int64
	WaitDuration    time.Duration
}
