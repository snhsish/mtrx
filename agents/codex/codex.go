package codex

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

func (a *Adapter) ID() string   { return "codex" }
func (a *Adapter) Name() string { return "Codex" }

func (a *Adapter) Detect(ctx context.Context) (bool, error) {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".config", "codex"),
	} {
		if _, err := os.Stat(p); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func (a *Adapter) Sources() []agents.Source {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".codex", "sessions")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	return []agents.Source{{Path: dir, Adapter: "codex"}}
}

func (a *Adapter) Collect(ctx context.Context, src agents.Source) ([]events.Event, error) {
	modelMap := loadModels(filepath.Join(homeDir(), ".codex", "state_5.sqlite"))
	var out []events.Event
	err := filepath.Walk(src.Path, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		evs, e := parseSession(p, modelMap)
		if e == nil {
			out = append(out, evs...)
		}
		return nil
	})
	return out, err
}

func (a *Adapter) Watch(ctx context.Context, src agents.Source, emit func(events.Event)) error {
	<-ctx.Done()
	return nil
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func loadModels(dbPath string) map[string]string {
	m := map[string]string{}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return m
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, model, model_provider FROM threads`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var id, model, provider sql.NullString
		if err := rows.Scan(&id, &model, &provider); err != nil {
			continue
		}
		name := model.String
		if name == "" {
			name = provider.String
		}
		if name != "" {
			m[id.String] = name
		}
	}
	return m
}

func parseSession(path string, modelMap map[string]string) ([]events.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var sessionID string
	var sessionTS time.Time
	var best Input
	var bestTS time.Time
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
		ts, _ := time.Parse(time.RFC3339, asString(raw["timestamp"]))
		if ts.IsZero() {
			ts, _ = time.Parse(time.RFC3339Nano, asString(raw["timestamp"]))
		}
		p, _ := raw["payload"].(map[string]any)
		if p == nil {
			continue
		}
		switch asString(raw["type"]) {
		case "session_meta":
			if id, _ := p["id"].(string); id != "" {
				sessionID = id
			}
			if !ts.IsZero() {
				sessionTS = ts
			}
		case "event_msg":
			if asString(p["type"]) == "token_count" {
				info, _ := p["info"].(map[string]any)
				if info == nil {
					continue
				}
				tu, _ := info["total_token_usage"].(map[string]any)
				if tu == nil {
					continue
				}
				cur := Input{
					Input:     toI(tu["input_tokens"]),
					Cached:    toI(tu["cached_input_tokens"]),
					Output:    toI(tu["output_tokens"]),
					Reasoning: toI(tu["reasoning_output_tokens"]),
					Total:     toI(tu["total_tokens"]),
				}
				if cur.Total > best.Total {
					best = cur
					bestTS = ts
				}
			}
		}
	}
	if sessionID == "" || best.Total == 0 {
		return nil, nil
	}
	model := modelMap[sessionID]
	if model == "" {
		model = "codex"
	}
	ts := bestTS
	if ts.IsZero() {
		ts = sessionTS
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	payload := map[string]any{
		"input":        best.Input,
		"output":       best.Output + best.Reasoning,
		"cached_input": best.Cached,
		"reasoning":    best.Reasoning,
		"total":        best.Total,
		"model":        model,
		"cost":         0,
	}
	pb, _ := json.Marshal(payload)
	rawMap := map[string]any{"session_id": sessionID, "model": model, "tokens": payload}
	rawB, _ := json.Marshal(rawMap)
	return []events.Event{{
		ID:            "codex_sess_" + sessionID,
		Type:          "generation.completed",
		Timestamp:     ts,
		Agent:         events.AgentRef{ID: "codex", Name: "Codex"},
		Session:       &events.SessionRef{ID: sessionID},
		Payload:       pb,
		Source:        &events.SourceRef{Adapter: "codex", Path: path},
		Raw:           rawB,
		SchemaVersion: 1,
	}}, nil
}

type Input struct {
	Input, Cached, Output, Reasoning, Total int64
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
