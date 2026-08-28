package api

import (
	"database/sql"
	"net/http"

	"mtrx/internal/analytics"
	"mtrx/internal/database"
)

func LatencyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, _ := analytics.LatencyStats(db)
		writeJSON(w, s)
	}
}

func InternalMetricsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		overview, _ := analytics.OverviewStats(db)
		lat, _ := analytics.LatencyStats(db)
		errs, _ := analytics.ErrorCount(db)
		writeJSON(w, map[string]any{"overview": overview, "latency": lat, "errors": errs})
	}
}

func Paginate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") == "" {
			q := r.URL.Query()
			q.Set("limit", "100")
			r.URL.RawQuery = q.Encode()
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) { _ = database.WriteJSON(w, v) }
