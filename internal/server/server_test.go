package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mtrx/internal/config"
	"mtrx/internal/database"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Default()
	cfg.Storage.Path = t.TempDir()
	db, err := database.Open(config.DBPath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	seeds := []struct{ id, typ, ts, agent, session string }{
		{"e1", "generation.completed", "2026-01-01T10:00:00Z", "opencode", "s1"},
		{"e2", "generation.completed", "2026-01-02T10:00:00Z", "claude", "s2"},
	}
	for _, s := range seeds {
		_, err := db.Exec(`INSERT INTO events(id,type,timestamp,agent,session,payload,raw) VALUES(?,?,?,?,?,?,?)`,
			s.id, s.typ, s.ts, s.agent, s.session, `{"input":1,"output":2}`, "{}")
		if err != nil {
			t.Fatal(err)
		}
	}
	return New(cfg, db)
}

func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	rec := get(t, newTestServer(t), "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %v", body)
	}
}

func TestEventsFilterByAgent(t *testing.T) {
	rec := get(t, newTestServer(t), "/api/v1/events?agent=opencode")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0]["agent"] != "opencode" {
		t.Fatalf("body = %v", body)
	}
}

func TestAgentByIDNotFound(t *testing.T) {
	rec := get(t, newTestServer(t), "/api/agents/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestOverview(t *testing.T) {
	rec := get(t, newTestServer(t), "/api/v1/metrics/overview")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "events") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
