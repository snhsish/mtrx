package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mtrx/internal/agents"
)

func writeFixture(t *testing.T, name, content string) agents.Source {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return agents.Source{Path: p, Adapter: "claude"}
}

func TestCollectAssistantUsage(t *testing.T) {
	src := writeFixture(t, "sess.jsonl", ""+
		`{"type":"assistant","uuid":"u1","sessionId":"s1","timestamp":"2026-03-01T10:00:00Z","message":{"model":"claude-opus","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":2,"cache_creation_input_tokens":1}}}`+"\n"+
		`{"type":"user","uuid":"u2","sessionId":"s1","timestamp":"2026-03-01T10:01:00Z","message":{"content":"hi"}}`+"\n"+
		`{"type":"system","timestamp":"2026-03-01T10:02:00Z"}`+"\n"+
		`{"type":"assistant","uuid":"u3","sessionId":"s1","message":{}}`+"\n")
	evs, err := (&Adapter{}).Collect(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	ev := evs[0]
	if ev.ID != "claude_s1_u1" {
		t.Fatalf("id = %q", ev.ID)
	}
	if ev.Type != "generation.completed" || ev.Agent.ID != "claude" {
		t.Fatalf("ev = %+v", ev)
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["input"] != 10.0 || payload["output"] != 5.0 || payload["cached_input"] != 2.0 || payload["cache_write"] != 1.0 {
		t.Fatalf("payload = %v", payload)
	}
	if payload["model"] != "claude-opus" {
		t.Fatalf("model = %v", payload["model"])
	}
}

func TestCollectSessionFallbackToFilename(t *testing.T) {
	src := writeFixture(t, "fallback.jsonl",
		`{"type":"assistant","uuid":"u9","timestamp":"2026-03-01T10:00:00Z","message":{"usage":{"input_tokens":3,"output_tokens":4}}}`+"\n")
	evs, err := (&Adapter{}).Collect(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	if evs[0].Session == nil || evs[0].Session.ID != "fallback.jsonl" {
		t.Fatalf("session = %+v", evs[0].Session)
	}
}

func TestCollectMissingFile(t *testing.T) {
	_, err := (&Adapter{}).Collect(context.Background(), agents.Source{Path: filepath.Join(t.TempDir(), "nope.jsonl")})
	if err == nil {
		t.Fatal("expected error")
	}
}
