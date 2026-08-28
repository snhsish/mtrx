package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mtrx/internal/agents"
	"mtrx/internal/events"
)

type Adapter struct{}

func (a *Adapter) ID() string   { return "claude" }
func (a *Adapter) Name() string { return "Claude Code" }

func (a *Adapter) Detect(ctx context.Context) (bool, error) {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".config", "claude"),
	} {
		if _, err := os.Stat(p); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func (a *Adapter) Sources() []agents.Source {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".claude", "projects")
	var out []agents.Source
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".jsonl") {
			out = append(out, agents.Source{Path: p, Adapter: "claude"})
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return out
}

func (a *Adapter) Collect(ctx context.Context, src agents.Source) ([]events.Event, error) {
	f, err := os.Open(src.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []events.Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		typ, _ := raw["type"].(string)
		if typ != "assistant" && typ != "user" {
			continue
		}
		msg, _ := raw["message"].(map[string]any)
		if msg == nil {
			continue
		}
		usage, _ := msg["usage"].(map[string]any)
		if usage == nil {
			continue
		}
		sessionID, _ := raw["sessionId"].(string)
		if sessionID == "" {
			sessionID = filepath.Base(src.Path)
		}
		model, _ := msg["model"].(string)
		if model == "" {
			model = "claude"
		}
		ts, _ := time.Parse(time.RFC3339, asString(raw["timestamp"]))
		if ts.IsZero() {
			ts, _ = time.Parse(time.RFC3339Nano, asString(raw["timestamp"]))
		}
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		uuid, _ := raw["uuid"].(string)
		if uuid == "" {
			uuid = sessionID
		}
		payload := map[string]any{
			"input":        toF(usage["input_tokens"]),
			"output":       toF(usage["output_tokens"]),
			"cached_input": toF(usage["cache_read_input_tokens"]),
			"cache_write":  toF(usage["cache_creation_input_tokens"]),
			"model":        model,
			"cost":         0,
		}
		pb, _ := json.Marshal(payload)
		rawMap := map[string]any{"session_id": sessionID, "model": model, "usage": usage}
		rawB, _ := json.Marshal(rawMap)
		out = append(out, events.Event{
			ID:            "claude_" + sessionID + "_" + uuid,
			Type:          "generation.completed",
			Timestamp:     ts,
			Agent:         events.AgentRef{ID: "claude", Name: "Claude Code"},
			Session:       &events.SessionRef{ID: sessionID},
			Payload:       pb,
			Source:        &events.SourceRef{Adapter: "claude", Path: src.Path},
			Raw:           rawB,
			SchemaVersion: 1,
		})
	}
	return out, scanner.Err()
}

func (a *Adapter) Watch(ctx context.Context, src agents.Source, emit func(events.Event)) error {
	<-ctx.Done()
	return nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func toF(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}
