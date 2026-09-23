package opencode

import (
	"bufio"
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

func (a *Adapter) ID() string   { return "opencode" }
func (a *Adapter) Name() string { return "OpenCode" }

func (a *Adapter) Detect(ctx context.Context) (bool, error) {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".local", "share", "opencode"),
		filepath.Join(home, ".config", "opencode"),
	} {
		if _, err := os.Stat(p); err == nil {
			return true, nil
		}
	}
	return false, nil
}

var skipFiles = map[string]struct{}{"account.json": {}, "auth.json": {}, "kv.json": {}, "model.json": {}, "session.json": {}, "mcp-auth.json": {}, "opencode.jsonc": {}, "package.json": {}, "package-lock.json": {}, ".package-map.json": {}, "pnpm-lock.yaml": {}}

func (a *Adapter) Sources() []agents.Source {
	home, _ := os.UserHomeDir()
	var out []agents.Source
	for _, base := range []string{
		filepath.Join(home, ".local", "share", "opencode"),
		filepath.Join(home, ".config", "opencode"),
	} {
		_ = filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.Contains(p, "node_modules") || strings.Contains(p, ".pnpm") {
				return nil
			}
			if _, ok := skipFiles[filepath.Base(p)]; ok {
				return nil
			}
			if strings.HasSuffix(p, ".db") {
				if filepath.Base(p) == "opencode-stable.db" {
					out = append(out, agents.Source{Path: p, Adapter: "opencode"})
				}
				return nil
			}
			if info.Size() > 5*1024*1024 {
				return nil
			}
			if strings.HasSuffix(p, ".jsonl") || strings.HasSuffix(p, ".json") {
				out = append(out, agents.Source{Path: p, Adapter: "opencode"})
			}
			return nil
		})
	}
	return out
}

func (a *Adapter) Collect(ctx context.Context, src agents.Source) ([]events.Event, error) {
	if strings.HasSuffix(src.Path, ".db") {
		return collectDB(src.Path)
	}
	f, err := os.Open(src.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []events.Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.Contains(line, `"credential"`) || strings.Contains(line, `"accounts"`) || strings.Contains(line, `"serviceID"`) {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if _, ok := raw["accounts"]; ok {
			continue
		}
		if _, ok := raw["credential"]; ok {
			continue
		}
		ev := normalize(raw, src.Path)
		out = append(out, ev)
	}
	return out, scanner.Err()
}

func toTime(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	if v > 1e14 {
		return time.Unix(0, v*int64(time.Millisecond)).UTC()
	}
	if v > 1e12 {
		return time.UnixMilli(v).UTC()
	}
	if v > 1e9 {
		return time.Unix(v, 0).UTC()
	}
	return time.Unix(v, 0).UTC()
}

func normalizeModel(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "{") {
		return s
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return s
	}
	for _, k := range []string{"id", "model", "model_name", "name"} {
		if v, _ := m[k].(string); v != "" {
			return strings.TrimSpace(v)
		}
	}
	return s
}

func collectDB(path string) ([]events.Event, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, time_created, time_updated, agent, model, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write, cost FROM session WHERE tokens_input>0 OR tokens_output>0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []events.Event
	for rows.Next() {
		var id, agent, model sql.NullString
		var tCreated, tUpdated sql.NullInt64
		var tin, tout, treason, tread, twrite sql.NullInt64
		var cost sql.NullFloat64
		if err := rows.Scan(&id, &tCreated, &tUpdated, &agent, &model, &tin, &tout, &treason, &tread, &twrite, &cost); err != nil {
			continue
		}
		ts := toTime(tUpdated.Int64)
		if ts.IsZero() {
			ts = toTime(tCreated.Int64)
		}
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		normalized := normalizeModel(model.String)
		if normalized == "" {
			normalized = "unknown"
		}
		payloadMap := map[string]any{
			"input": tin.Int64, "output": tout.Int64, "input_tokens": tin.Int64, "output_tokens": tout.Int64,
			"reasoning": treason.Int64, "reasoning_tokens": treason.Int64,
			"cached_input": tread.Int64, "cache_read": tread.Int64, "cache_write": twrite.Int64,
			"model": normalized, "model_raw": model.String, "cost": cost.Float64,
		}
		pb, _ := json.Marshal(payloadMap)
		rawMap := map[string]any{"session_id": id.String, "tokens": payloadMap, "model": normalized, "model_raw": model.String}
		rawB, _ := json.Marshal(rawMap)
		evID := "opencode_sess_" + id.String
		out = append(out, events.Event{
			ID:            evID,
			Type:          "generation.completed",
			Timestamp:     ts,
			Agent:         events.AgentRef{ID: "opencode", Name: "OpenCode"},
			Session:       &events.SessionRef{ID: id.String},
			Payload:       pb,
			Source:        &events.SourceRef{Adapter: "opencode", Path: path},
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

func normalize(raw map[string]any, source string) events.Event {
	typ, _ := raw["type"].(string)
	if typ == "" {
		if _, ok := raw["tool"]; ok {
			typ = "tool.completed"
		} else if _, ok := raw["tokens"]; ok {
			typ = "generation.completed"
		} else if _, ok := raw["model"]; ok {
			typ = "generation.completed"
		} else if _, ok := raw["usage"]; ok {
			typ = "generation.completed"
		} else {
			typ = "unknown"
		}
	}
	tsStr, _ := raw["timestamp"].(string)
	if tsStr == "" {
		tsStr, _ = raw["time"].(string)
	}
	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		ts, _ = time.Parse(time.RFC3339Nano, tsStr)
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
	}
	id, _ := raw["id"].(string)
	if id == "" {
		id = events.NewID()
	}
	var payload json.RawMessage
	if t, ok := raw["tokens"]; ok {
		b, _ := json.Marshal(t)
		payload = b
	} else if p, ok := raw["payload"]; ok {
		b, _ := json.Marshal(p)
		payload = b
	}
	rawB, _ := json.Marshal(raw)
	sessID, _ := raw["session_id"].(string)
	if sessID == "" {
		sessID, _ = raw["session"].(string)
	}
	var sess *events.SessionRef
	if sessID != "" {
		sess = &events.SessionRef{ID: sessID}
	}
	return events.Event{
		ID:            id,
		Type:          events.EventType(typ),
		Timestamp:     ts,
		Agent:         events.AgentRef{ID: "opencode", Name: "OpenCode"},
		Session:       sess,
		Payload:       payload,
		Source:        &events.SourceRef{Adapter: "opencode", Path: source},
		Raw:           rawB,
		SchemaVersion: 1,
	}
}
