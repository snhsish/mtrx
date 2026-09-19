package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mtrx/internal/agents"
)

func writeSession(t *testing.T, content string) agents.Source {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sess.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return agents.Source{Path: dir, Adapter: "codex"}
}

func TestCollectPicksBestTokenCount(t *testing.T) {
	src := writeSession(t, ""+
		`{"type":"session_meta","timestamp":"2026-04-01T08:00:00Z","payload":{"id":"abc"}}`+"\n"+
		`{"type":"event_msg","timestamp":"2026-04-01T08:01:00Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":60,"cached_input_tokens":0,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":100}}}}`+"\n"+
		`{"type":"event_msg","timestamp":"2026-04-01T08:02:00Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":150,"cached_input_tokens":5,"output_tokens":80,"reasoning_output_tokens":20,"total_tokens":250}}}}`+"\n"+
		`garbage line`+"\n")
	evs, err := (&Adapter{}).Collect(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	ev := evs[0]
	if ev.ID != "codex_sess_abc" {
		t.Fatalf("id = %q", ev.ID)
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["input"] != 150.0 || payload["total"] != 250.0 {
		t.Fatalf("payload = %v", payload)
	}
	if payload["output"] != 100.0 {
		t.Fatalf("output should include reasoning, payload = %v", payload)
	}
}

func TestCollectNoUsableSession(t *testing.T) {
	src := writeSession(t, `{"type":"session_meta","timestamp":"2026-04-01T08:00:00Z","payload":{"id":"empty"}}`+"\n")
	evs, err := (&Adapter{}).Collect(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 0 {
		t.Fatalf("got %d events, want 0", len(evs))
	}
}

func TestCollectEmptyDir(t *testing.T) {
	src := agents.Source{Path: t.TempDir(), Adapter: "codex"}
	evs, err := (&Adapter{}).Collect(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 0 {
		t.Fatalf("got %d events, want 0", len(evs))
	}
}
