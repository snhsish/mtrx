package server

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"mtrx/internal/analytics"
	"mtrx/internal/config"
	"mtrx/internal/database"
	"mtrx/internal/discovery"
)

//go:embed static
var staticFS embed.FS

type Server struct {
	cfg config.Config
	db  *sql.DB
	mux *http.ServeMux
}

func New(cfg config.Config, db *sql.DB) *Server {
	s := &Server{cfg: cfg, db: db, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/health", s.handleHealth)
	s.mux.HandleFunc("/api/agents", s.handleAgents)
	s.mux.HandleFunc("/api/v1/agents", s.handleAgents)
	s.mux.HandleFunc("/api/agents/", s.handleAgentByID)
	s.mux.HandleFunc("/api/v1/agents/", s.handleAgentByID)
	s.mux.HandleFunc("/api/projects", s.handleProjects)
	s.mux.HandleFunc("/api/v1/projects", s.handleProjects)
	s.mux.HandleFunc("/api/projects/", s.handleProjectByID)
	s.mux.HandleFunc("/api/v1/projects/", s.handleProjectByID)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/v1/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/sessions/", s.handleSessionByID)
	s.mux.HandleFunc("/api/v1/sessions/", s.handleSessionByID)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/v1/events", s.handleEvents)
	s.mux.HandleFunc("/api/metrics/overview", s.handleOverview)
	s.mux.HandleFunc("/api/v1/metrics/overview", s.handleOverview)
	s.mux.HandleFunc("/api/metrics/tokens", s.handleTokens)
	s.mux.HandleFunc("/api/v1/metrics/tokens", s.handleTokens)
	s.mux.HandleFunc("/api/metrics/tokens/live", s.handleTokensLive)
	s.mux.HandleFunc("/api/v1/metrics/tokens/live", s.handleTokensLive)
	s.mux.HandleFunc("/api/metrics/activity", s.handleActivity)
	s.mux.HandleFunc("/api/v1/metrics/activity", s.handleActivity)
	s.mux.HandleFunc("/api/metrics/tools", s.handleTools)
	s.mux.HandleFunc("/api/v1/metrics/tools", s.handleTools)
	s.mux.HandleFunc("/api/metrics/models", s.handleModels)
	s.mux.HandleFunc("/api/v1/metrics/models", s.handleModels)
	s.mux.HandleFunc("/api/metrics/errors", s.handleErrors)
	s.mux.HandleFunc("/api/v1/metrics/errors", s.handleErrors)
	s.mux.HandleFunc("/api/metrics/latency", s.handleLatency)
	s.mux.HandleFunc("/api/v1/metrics/latency", s.handleLatency)
	s.mux.HandleFunc("/api/metrics/internal", s.handleInternal)
	s.mux.HandleFunc("/api/v1/metrics/internal", s.handleInternal)
	s.mux.HandleFunc("/api/meta/agents", s.handleMetaAgents)
	s.mux.HandleFunc("/api/v1/meta/agents", s.handleMetaAgents)
	s.mux.HandleFunc("/api/meta/models", s.handleMetaModels)
	s.mux.HandleFunc("/api/v1/meta/models", s.handleMetaModels)
	s.mux.HandleFunc("/api/live", s.handleLive)
	s.mux.HandleFunc("/api/v1/live", s.handleLive)
	s.mux.HandleFunc("/", s.handleFrontend)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats, _ := database.GetStats(s.db, config.DBPath(s.cfg))
	writeJSON(w, map[string]any{
		"status":   "ok",
		"database": map[string]any{"events": stats.EventCount, "sessions": stats.SessionCount},
	})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	agents := discovery.Detect()
	writeJSON(w, agents)
}

func (s *Server) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	id = strings.TrimPrefix(id, "/api/v1/agents/")
	id = strings.Trim(id, "/")
	for _, a := range discovery.Detect() {
		if a.ID == id {
			writeJSON(w, a)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	rows, _ := s.db.Query(`SELECT id, path, name FROM projects LIMIT 100`)
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()
	var out []map[string]any
	if rows != nil {
		for rows.Next() {
			var id, path, name sql.NullString
			_ = rows.Scan(&id, &path, &name)
			out = append(out, map[string]any{"id": id.String, "path": path.String, "name": name.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, out)
}

func (s *Server) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	id = strings.TrimPrefix(id, "/api/v1/projects/")
	id = strings.Trim(id, "/")
	var pid, path, name sql.NullString
	err := s.db.QueryRow(`SELECT id, path, name FROM projects WHERE id=?`, id).Scan(&pid, &path, &name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{"id": pid.String, "path": path.String, "name": name.String})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	rows, _ := s.db.Query(`SELECT id, agent_id, project_id, started_at, ended_at FROM sessions ORDER BY started_at DESC LIMIT 100`)
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()
	var out []map[string]any
	if rows != nil {
		for rows.Next() {
			var id, agentID, projID, started, ended sql.NullString
			_ = rows.Scan(&id, &agentID, &projID, &started, &ended)
			out = append(out, map[string]any{"id": id.String, "agent": agentID.String, "project": projID.String, "started_at": started.String, "ended_at": ended.String})
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	writeJSON(w, out)
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	id = strings.TrimPrefix(id, "/api/v1/sessions/")
	id = strings.Trim(id, "/")
	var sid, agentID, projID, started, ended sql.NullString
	err := s.db.QueryRow(`SELECT id, agent_id, project_id, started_at, ended_at FROM sessions WHERE id=?`, id).Scan(&sid, &agentID, &projID, &started, &ended)
	if err != nil {
		rows, _ := database.QueryEventsFiltered(s.db, database.EventFilter{Session: id, Limit: 100})
		if len(rows) > 0 {
			writeJSON(w, map[string]any{"id": id, "events": rows})
			return
		}
		http.NotFound(w, r)
		return
	}
	events, _ := database.QueryEventsFiltered(s.db, database.EventFilter{Session: id, Limit: 100})
	writeJSON(w, map[string]any{"id": sid.String, "agent": agentID.String, "project": projID.String, "started_at": started.String, "ended_at": ended.String, "events": events})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleIngest(w, r)
		return
	}
	q := r.URL.Query()
	f := database.EventFilter{
		Agent:   q.Get("agent"),
		Session: q.Get("session"),
		Type:    q.Get("type"),
		Project: q.Get("project"),
		Model:   q.Get("model"),
		From:    q.Get("from"),
		To:      q.Get("to"),
		Limit:   100,
	}
	if v := q.Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &f.Limit)
	}
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 100
	}
	events, err := database.QueryEventsFiltered(s.db, f)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if events == nil {
		events = []map[string]any{}
	}
	writeJSON(w, events)
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", 400)
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	id, _ := raw["id"].(string)
	if id == "" {
		id = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	typ, _ := raw["type"].(string)
	if typ == "" {
		typ = "unknown"
	}
	ts, _ := raw["timestamp"].(string)
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	agent := ""
	if a, ok := raw["agent"].(string); ok {
		agent = a
	} else if m, ok := raw["agent"].(map[string]any); ok {
		agent, _ = m["id"].(string)
	}
	if agent == "" {
		agent = "generic"
	}
	payload := ""
	if p, ok := raw["payload"]; ok {
		b, _ := json.Marshal(p)
		payload = string(b)
	} else if p, ok := raw["usage"]; ok {
		b, _ := json.Marshal(p)
		payload = string(b)
	}
	session := ""
	if s, ok := raw["session"].(string); ok {
		session = s
	}
	project := ""
	if p, ok := raw["project"].(string); ok {
		project = p
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO events(id,type,timestamp,agent,session,project,payload,raw) VALUES(?,?,?,?,?,?,?,?)`, id, typ, ts, agent, session, project, payload, string(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"id": id, "status": "ok"})
}

func parseFilter(r *http.Request) analytics.Filter {
	q := r.URL.Query()
	f := analytics.Filter{
		From:  q.Get("from"),
		To:    q.Get("to"),
		Agent: q.Get("agent"),
		Model: q.Get("model"),
	}
	if f.From == "" && f.To == "" {
		f.From = time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339)
	}
	if f.From != "" {
		if _, err := time.Parse(time.RFC3339, f.From); err != nil {
			if t, err2 := time.Parse("2006-01-02", f.From); err2 == nil {
				f.From = t.UTC().Format(time.RFC3339)
			}
		}
	}
	if f.To != "" {
		if _, err := time.Parse(time.RFC3339, f.To); err != nil {
			if t, err2 := time.Parse("2006-01-02", f.To); err2 == nil {
				f.To = t.Add(24*time.Hour - time.Nanosecond).UTC().Format(time.RFC3339)
			}
		}
	}
	return f
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	o, _ := analytics.OverviewStatsFiltered(s.db, f)
	writeJSON(w, o)
}

func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	data, _ := analytics.TokensByDayFiltered(s.db, f)
	writeJSON(w, data)
}

func (s *Server) handleTokensLive(w http.ResponseWriter, r *http.Request) {
	from := time.Now().Add(-5 * time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	f := analytics.Filter{From: from, To: to}
	data, _ := analytics.TokensByHourFiltered(s.db, f)
	writeJSON(w, data)
}

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	data, _ := analytics.ActivityByDayFiltered(s.db, f)
	writeJSON(w, data)
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	data, _ := analytics.ToolsStatsFiltered(s.db, f)
	writeJSON(w, data)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	data, _ := analytics.ModelsStatsFiltered(s.db, f)
	writeJSON(w, data)
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	n, _ := analytics.ErrorCountFiltered(s.db, f)
	writeJSON(w, map[string]any{"errors": n})
}

func (s *Server) handleLatency(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	ls, _ := analytics.LatencyStatsFiltered(s.db, f)
	writeJSON(w, ls)
}

func (s *Server) handleInternal(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	o, _ := analytics.OverviewStatsFiltered(s.db, f)
	ls, _ := analytics.LatencyStatsFiltered(s.db, f)
	errs, _ := analytics.ErrorCountFiltered(s.db, f)
	writeJSON(w, map[string]any{"overview": o, "latency": ls, "errors": errs})
}

func (s *Server) handleMetaAgents(w http.ResponseWriter, r *http.Request) {
	known := map[string]struct{}{}
	for _, a := range discovery.KnownAgents() {
		known[a.ID] = struct{}{}
	}
	known["generic"] = struct{}{}
	seen := map[string]struct{}{}
	var out []string
	for _, a := range discovery.Detect() {
		if a.Detected {
			if _, ok := seen[a.ID]; !ok {
				seen[a.ID] = struct{}{}
				out = append(out, a.ID)
			}
		}
	}
	data, _ := analytics.DistinctAgents(s.db)
	for _, v := range data {
		if _, ok := known[v]; ok {
			if _, dup := seen[v]; !dup {
				seen[v] = struct{}{}
				out = append(out, v)
			}
		}
	}
	if out == nil {
		out = []string{}
	}
	writeJSON(w, out)
}

func (s *Server) handleMetaModels(w http.ResponseWriter, r *http.Request) {
	data, _ := analytics.DistinctModels(s.db)
	writeJSON(w, data)
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected"}`)
		return
	}
	ctx := r.Context()
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected"}`)
	flusher.Flush()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/" || r.URL.Path == "" {
		b, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, fallbackHTML())
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(b)
		return
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}

func fallbackHTML() string {
	return `<!doctype html><html><head><meta charset="utf-8"><title>mtrx</title></head><body><h1>mtrx</h1><p>Local telemetry dashboard</p><p>API: <a href="/api/health">/api/health</a></p></body></html>`
}
