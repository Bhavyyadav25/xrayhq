package xrayhq

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func (ds *DashboardServer) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	requests := ds.collector.GetAllRequests()

	switch format {
	case "csv":
		ds.exportCSV(w, requests)
	default:
		ds.exportJSON(w, requests)
	}
}

func (ds *DashboardServer) exportJSON(w http.ResponseWriter, requests []*RequestTrace) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=xrayhq-export.json")
	json.NewEncoder(w).Encode(requests)
}

func (ds *DashboardServer) exportCSV(w http.ResponseWriter, requests []*RequestTrace) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=xrayhq-export.csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header
	writer.Write([]string{
		"ID", "Method", "Path", "RoutePattern", "Status",
		"Latency(ms)", "TTFB(ms)", "RequestSize", "ResponseSize",
		"DBQueries", "TotalDBTime(ms)", "ExternalCalls", "TotalExtTime(ms)",
		"ClientIP", "UserAgent", "Timestamp", "Panicked",
	})

	for _, req := range requests {
		writer.Write([]string{
			req.ID,
			req.Method,
			req.Path,
			req.RoutePattern,
			strconv.Itoa(req.ResponseStatus),
			fmt.Sprintf("%.2f", float64(req.Latency.Microseconds())/1000),
			fmt.Sprintf("%.2f", float64(req.TTFB.Microseconds())/1000),
			strconv.FormatInt(req.RequestSize, 10),
			strconv.FormatInt(req.ResponseSize, 10),
			strconv.Itoa(len(req.DBQueries)),
			fmt.Sprintf("%.2f", float64(req.TotalDBTime.Microseconds())/1000),
			strconv.Itoa(len(req.ExternalCalls)),
			fmt.Sprintf("%.2f", float64(req.TotalExtTime.Microseconds())/1000),
			req.ClientIP,
			req.UserAgent,
			req.StartTime.Format("2006-01-02T15:04:05.000Z07:00"),
			strconv.FormatBool(req.Panicked),
		})
	}
}
