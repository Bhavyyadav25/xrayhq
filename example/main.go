package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/xrayhq/xrayhq"
)

func main() {
	// Initialize xrayhq with configuration options
	xrayhq.Init(
		xrayhq.WithPort(":9090"),
		xrayhq.WithBufferSize(1000),
		xrayhq.WithMode(xrayhq.ModeDev),
		xrayhq.WithCaptureBody(true),
		xrayhq.WithCaptureHeaders(true),
		xrayhq.WithSlowQueryThreshold(200*time.Millisecond),
		xrayhq.WithSlowRouteThreshold(1*time.Second),
		xrayhq.WithHighErrorRate(15.0),
		xrayhq.WithNPlusOneThreshold(5),
		xrayhq.WithMemorySpikeThreshold(5*1024*1024),
		xrayhq.WithSamplingRate(1.0),
		// xrayhq.WithBasicAuth("admin", "secret"),
	)

	// Wrap an HTTP client to track external calls
	client := xrayhq.WrapHTTPClient(&http.Client{Timeout: 5 * time.Second})

	mux := http.NewServeMux()

	// Basic endpoint with DB queries
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

		trace := xrayhq.TraceFromContext(r.Context())
		if trace != nil {
			for i := 0; i < rand.Intn(5)+1; i++ {
				xrayhq.AddDBQuery(r.Context(), xrayhq.DBQuery{
					Query:        fmt.Sprintf("SELECT * FROM users WHERE id = %d", rand.Intn(1000)),
					Duration:     time.Duration(rand.Intn(10)) * time.Millisecond,
					RowsAffected: 1,
					Timestamp:    time.Now(),
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []map[string]string{
				{"id": "1", "name": "Alice"},
				{"id": "2", "name": "Bob"},
			},
		})
	})

	// N+1 query example
	mux.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

		for i := 0; i < 8; i++ {
			xrayhq.AddDBQuery(r.Context(), xrayhq.DBQuery{
				Query:        "SELECT * FROM order_items WHERE order_id = ?",
				Duration:     time.Duration(rand.Intn(5)) * time.Millisecond,
				RowsAffected: int64(rand.Intn(10)),
				Timestamp:    time.Now(),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Health check
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// Slow endpoint
	mux.HandleFunc("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(500+rand.Intn(2000)) * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "slow but ok"})
	})

	// Error endpoint
	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		if rand.Float64() < 0.7 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "something went wrong"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Panic endpoint
	mux.HandleFunc("/api/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("something terrible happened!")
	})

	// External HTTP call example
	mux.HandleFunc("/api/external", func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequestWithContext(r.Context(), "GET", "https://httpbin.org/get", nil)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"external_status": resp.StatusCode,
		})
	})

	// Wrap with xrayhq middleware
	handler := xrayhq.Wrap(mux)

	log.Println("API server starting on :8080")
	log.Println("xrayhq dashboard at http://localhost:9090")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

// Additional integration examples (not wired into this demo server):
//
// Redis instrumentation:
//   rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//   rdb.AddHook(xrayhq.RedisHook())
//
// MongoDB instrumentation:
//   opts := options.Client().ApplyURI("mongodb://localhost:27017").
//       SetMonitor(xrayhq.MongoMonitor())
//   client, _ := mongo.Connect(ctx, opts)
//
// GORM instrumentation:
//   db, _ := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
//   db.Use(xrayhq.NewGORMPlugin())
//
// database/sql instrumentation:
//   sqlDB, _ := sql.Open("postgres", dsn)
//   wrappedDB := xrayhq.WrapDB(sqlDB)
//   wrappedDB.QueryContext(ctx, "SELECT * FROM users WHERE id = $1", 1)
//
// Chi middleware:
//   r := chi.NewRouter()
//   r.Use(xrayhq.ChiMiddleware)
//
// Echo middleware:
//   e := echo.New()
//   e.Use(xrayhq.EchoMiddleware())
//
// Gin middleware:
//   r := gin.Default()
//   r.Use(xrayhq.GinMiddleware())
//
// Fiber middleware:
//   app := fiber.New()
//   app.Use(xrayhq.FiberMiddleware())
