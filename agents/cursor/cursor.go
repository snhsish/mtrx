package cursor

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"mtrx/internal/agents"
	"mtrx/internal/events"
)

type Adapter struct{}

func (a *Adapter) ID() string   { return "cursor" }
func (a *Adapter) Name() string { return "Cursor" }

func (a *Adapter) Detect(ctx context.Context) (bool, error) {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".config", "Cursor", "User", "globalStorage", "state.vscdb"),
		filepath.Join(home, ".cursor"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func (a *Adapter) Sources() []agents.Source {
	home, _ := os.UserHomeDir()
	db := filepath.Join(home, ".config", "Cursor", "User", "globalStorage", "state.vscdb")
	if fi, err := os.Stat(db); err != nil || fi.IsDir() {
		return nil
	}
	return []agents.Source{{Path: db, Adapter: "cursor"}}
}

func (a *Adapter) Collect(ctx context.Context, src agents.Source) ([]events.Event, error) {
	db, err := sql.Open("sqlite", "file:"+src.Path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT key, value FROM cursorDiskKV WHERE key LIKE 'bubbleId:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type conv struct {
		calls  int64
		model  string
		lastTS time.Time
	}
	convs := map[string]*conv{}
	for rows.Next() {
		var key, value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		parts := strings.Split(key.String, ":")
		if len(parts) < 3 {
			continue
		}
		composerID := parts[1]
		var b map[string]any
		if err := json.Unmarshal([]byte(value.String), &b); err != nil {
			continue
		}
		if toI(b["type"]) != 2 {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, asString(b["createdAt"]))
		if ts.IsZero() {
			ts, _ = time.Parse(time.RFC3339Nano, asString(b["createdAt"]))
		}
		c := convs[composerID]
		if c == nil {
			c = &conv{}
			convs[composerID] = c
		}
		c.calls++
		if mi, ok := b["modelInfo"].(map[string]any); ok {
			if mn, _ := mi["modelName"].(string); mn != "" && mn != "default" {
				c.model = mn
			}
		}
		if ts.After(c.lastTS) {
			c.lastTS = ts
		}
	}
	var out []events.Event
	for composerID, c := range convs {
		model := c.model
		if model == "" {
			model = "cursor"
		}
		ts := c.lastTS
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		payload := map[string]any{
			"input":  0,
			"output": 0,
			"model":  model,
			"calls":  c.calls,
			"cost":   0,
		}
		pb, _ := json.Marshal(payload)
		rawMap := map[string]any{"conversation_id": composerID, "model": model, "calls": c.calls}
		rawB, _ := json.Marshal(rawMap)
		out = append(out, events.Event{
			ID:            "cursor_conv_" + composerID,
			Type:          "generation.completed",
			Timestamp:     ts,
			Agent:         events.AgentRef{ID: "cursor", Name: "Cursor"},
			Session:       &events.SessionRef{ID: composerID},
			Payload:       pb,
			Source:        &events.SourceRef{Adapter: "cursor", Path: src.Path},
			Raw:           rawB,
			SchemaVersion: 1,
		})
	}
	return out, nil
}

func (a *Adapter) Watch(ctx context.Context, src agents.Source, emit func(events.Event)) error {
	<-ctx.Done()
	return nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func toI(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	}
	return 0
}
