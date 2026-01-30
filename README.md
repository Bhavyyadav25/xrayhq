# xrayhq

[![Go Reference](https://pkg.go.dev/badge/github.com/Bhavyyadav25/xrayhq.svg)](https://pkg.go.dev/github.com/Bhavyyadav25/xrayhq)
[![Go Report Card](https://goreportcard.com/badge/github.com/Bhavyyadav25/xrayhq)](https://goreportcard.com/report/github.com/Bhavyyadav25/xrayhq)

Lightweight, drop-in observability library for Go web applications. Request tracing, DB/Redis/MongoDB instrumentation, and a real-time dashboard — no external infrastructure required.

![Dashboard](https://img.shields.io/badge/dashboard-built--in-blue)
![Go](https://img.shields.io/badge/go-%3E%3D1.21-00ADD8)

## Features

- **Request tracing** — latency, TTFB, status codes, request/response bodies, headers, goroutine and memory deltas
- **Database instrumentation** — `database/sql`, GORM, Redis (go-redis), MongoDB (mongo-driver)
- **External call tracking** — wraps `http.Client` to record outbound requests
- **Real-time dashboard** — routes overview, per-route detail with latency histograms, request waterfall view, live tail with SSE, system stats
- **Automatic alerting** — N+1 queries, slow queries, slow routes (P95), high error rates, memory spikes, panics
- **Framework middleware** — net/http, Chi, Echo, Gin, Fiber
- **Data export** — JSON and CSV export of captured traces
- **Zero infrastructure** — everything runs in-process with a ring buffer, no databases or agents needed

## Installation

```bash
go get github.com/Bhavyyadav25/xrayhq
```

## Quick Start

```go
package main

import (
    "net/http"
    "github.com/Bhavyyadav25/xrayhq"
)

func main() {
    // Initialize — dashboard starts at localhost:9090
    xrayhq.Init(
        xrayhq.WithPort(":9090"),
        xrayhq.WithMode(xrayhq.ModeDev),
    )

    mux := http.NewServeMux()
    mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("hello"))
    })

    // Wrap your handler
    http.ListenAndServe(":8080", xrayhq.Wrap(mux))
}
```

Open `http://localhost:9090` to see the dashboard.

## Configuration

```go
xrayhq.Init(
    xrayhq.WithPort(":9090"),               // Dashboard port
    xrayhq.WithBufferSize(1000),             // Ring buffer size
    xrayhq.WithMode(xrayhq.ModeDev),         // ModeDev or ModeProd
    xrayhq.WithSamplingRate(1.0),             // 1.0 = capture all, 0.5 = 50%
    xrayhq.WithCaptureBody(true),             // Capture request/response bodies
    xrayhq.WithCaptureHeaders(true),          // Capture headers
    xrayhq.WithBasicAuth("admin", "secret"),  // Protect dashboard
    xrayhq.WithSlowQueryThreshold(500*time.Millisecond),
    xrayhq.WithSlowRouteThreshold(2*time.Second),
    xrayhq.WithHighErrorRate(10.0),           // Alert above 10% error rate
    xrayhq.WithNPlusOneThreshold(5),          // Alert on 5+ repeated queries
    xrayhq.WithMemorySpikeThreshold(10*1024*1024), // 10MB
    xrayhq.WithLatencyCap(10000),             // Max latencies stored per route
)
```

## Framework Integration

### Chi

```go
r := chi.NewRouter()
r.Use(xrayhq.ChiMiddleware)
```

### Echo

```go
e := echo.New()
e.Use(xrayhq.EchoMiddleware())
```

### Gin

```go
r := gin.Default()
r.Use(xrayhq.GinMiddleware())
```

### Fiber

```go
app := fiber.New()
app.Use(xrayhq.FiberMiddleware())

// Access trace in Fiber handlers
app.Get("/api/data", func(c *fiber.Ctx) error {
    xrayhq.FiberAddDBQuery(c, xrayhq.DBQuery{
        Query:    "SELECT * FROM users",
        Duration: 5 * time.Millisecond,
    })
    return c.SendString("ok")
})
```

## Database Instrumentation

### database/sql

```go
sqlDB, _ := sql.Open("postgres", dsn)
db := xrayhq.WrapDB(sqlDB)

// Use db.QueryContext, db.ExecContext, db.QueryRowContext
// All queries are automatically traced
rows, err := db.QueryContext(ctx, "SELECT * FROM users WHERE id = $1", 1)
```

### GORM

```go
db, _ := gorm.Open(sqlite.Open("app.db"), &gorm.Config{})
db.Use(xrayhq.NewGORMPlugin())

// All GORM operations are automatically traced
db.WithContext(ctx).Find(&users)
```

## Redis Instrumentation

```go
rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
rdb.AddHook(xrayhq.RedisHook())

// All Redis commands are automatically traced
rdb.Get(ctx, "user:123")
```

## MongoDB Instrumentation

```go
opts := options.Client().
    ApplyURI("mongodb://localhost:27017").
    SetMonitor(xrayhq.MongoMonitor())

client, _ := mongo.Connect(ctx, opts)

// All MongoDB operations are automatically traced
collection.FindOne(ctx, bson.M{"name": "Alice"})
```

## External HTTP Call Tracking

```go
client := xrayhq.WrapHTTPClient(&http.Client{Timeout: 5 * time.Second})

// Outbound calls are traced when the request carries a traced context
req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com/data", nil)
resp, err := client.Do(req)
```

## Manual Query Instrumentation

Add queries manually within any handler:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    // ... run your query ...

    xrayhq.AddDBQuery(r.Context(), xrayhq.DBQuery{
        Query:        "SELECT * FROM orders WHERE user_id = ?",
        Duration:     time.Since(start),
        RowsAffected: 42,
        Timestamp:    start,
    })
}
```

## Dashboard Pages

| Page | URL | Description |
|------|-----|-------------|
| Routes | `/` | All routes with hit counts, avg/P95/P99 latency, error rates |
| Route Detail | `/route/GET/api/users` | Per-route latency histogram, status distribution, slowest requests |
| Request Detail | `/request/{id}` | Full request waterfall: DB queries, external calls, Redis/Mongo ops |
| Live Tail | `/live` | Real-time request stream via Server-Sent Events |
| Alerts | `/alerts` | All triggered alerts (N+1, slow query, error rate, panics) |
| System | `/system` | Goroutines, memory, GC stats, uptime |

## Data Export

Export captured traces for offline analysis:

```
GET /xrayhq/export               → JSON
GET /xrayhq/export?format=csv    → CSV
```

## Alert Types

| Alert | Trigger | Severity |
|-------|---------|----------|
| N+1 Query | Same query pattern repeated > threshold times in one request | Warning |
| Slow Query | Individual query exceeds threshold | Warning |
| Slow Route | Route P95 exceeds threshold (after 10+ requests) | Warning |
| High Error Rate | Route 5xx rate exceeds threshold (after 10+ requests) | Critical |
| Memory Spike | Request allocates more than threshold bytes | Warning |
| Panic | Handler panics (recovered automatically) | Critical |

## Architecture

xrayhq runs entirely in-process:

```
┌─────────────────────────────────────────────┐
│  Your Application                           │
│                                             │
│  ┌───────────┐    ┌──────────────────────┐  │
│  │ Middleware │───>│     Collector        │  │
│  │ (per req) │    │  (ring buffer + map) │  │
│  └───────────┘    └──────────┬───────────┘  │
│                              │              │
│                   ┌──────────┴───────────┐  │
│                   │    Alert Engine       │  │
│                   └──────────┬───────────┘  │
│                              │              │
│                   ┌──────────┴───────────┐  │
│                   │  Dashboard Server    │  │
│                   │  (SSE + HTML + API)  │  │
│                   └──────────────────────┘  │
└─────────────────────────────────────────────┘
```

- **Ring buffer** stores the last N requests (configurable), zero GC pressure from old traces
- **Route metrics** aggregate latency percentiles, error rates, and status code distributions
- **SSE** streams new requests to the live tail view in real time
- **No goroutine leaks** — SSE clients are tracked and cleaned up on disconnect

## Production Usage

For production, reduce overhead with sampling and disable body capture:

```go
xrayhq.Init(
    xrayhq.WithMode(xrayhq.ModeProd),
    xrayhq.WithSamplingRate(0.1),        // Capture 10% of requests
    xrayhq.WithCaptureBody(false),
    xrayhq.WithCaptureHeaders(false),
    xrayhq.WithBasicAuth("admin", "strongpassword"),
)
```

## API Reference

Full documentation on [pkg.go.dev](https://pkg.go.dev/github.com/Bhavyyadav25/xrayhq).

## License

MIT
