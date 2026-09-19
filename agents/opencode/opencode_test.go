package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mtrx/internal/agents"
)

func writeFixture(t *testing.T, content string) agents.Source {
	t.Helper()
	p := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return agents.Source{Path: p, Adapter: "opencode"}
}

func TestCollectJSONL(t *testing.T) {
	src := writeFixture(t, ""+
		`{"id":"evt1","type":"generation.completed","timestamp":"2026-01-02T03:04:05Z","session_id":"sess1","tokens":{"input":100,"output":50},"model":"gpt-4"}`+"\n"+
		`{"id":"evt2","tool":"bash","timestamp":"2026-01-02T03:05:00Z"}`+"\n"+
		`{"credential":"secret-should-be-skipped"}`+"\n"+
		`not json at all`+"\n"+
		"\n")
	evs, err := (&Adapter{}).Collect(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2", len(evs))
	}
	gen := evs[0]
	if gen.Type != "generation.completed" {
		t.Fatalf("type = %q", gen.Type)
	}
	if gen.Agent.ID != "opencode" {
		t.Fatalf("agent = %q", gen.Agent.ID)
	}
	if gen.Session == nil || gen.Session.ID != "sess1" {
		t.Fatalf("session = %+v", gen.Session)
	}
	var payload map[string]any
	if err := json.Unmarshal(gen.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["input"] != 100.0 || payload["output"] != 50.0 {
		t.Fatalf("payload = %v", payload)
	}
	if evs[1].Type != "tool.completed" {
		t.Fatalf("type = %q", evs[1].Type)
	}
}

func TestCollectMissingFile(t *testing.T) {
	_, err := (&Adapter{}).Collect(context.Background(), agents.Source{Path: filepath.Join(t.TempDir(), "nope.jsonl")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAdapterIdentity(t *testing.T) {
	a := &Adapter{}
	if a.ID() != "opencode" || a.Name() == "" {
		t.Fatalf("id=%q name=%q", a.ID(), a.Name())
	}
}
