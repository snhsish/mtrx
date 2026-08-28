package generic

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"mtrx/internal/agents"
	"mtrx/internal/events"
)

type Adapter struct{}

func (a *Adapter) ID() string   { return "generic" }
func (a *Adapter) Name() string { return "Generic" }
func (a *Adapter) Detect(ctx context.Context) (bool, error) { return true, nil }
func (a *Adapter) Sources() []agents.Source                 { return nil }
func (a *Adapter) Collect(ctx context.Context, src agents.Source) ([]events.Event, error) {
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
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		ev := toEvent(raw, line)
		out = append(out, ev)
	}
	return out, scanner.Err()
}
func (a *Adapter) Watch(ctx context.Context, src agents.Source, emit func(events.Event)) error {
	<-ctx.Done()
	return nil
}

func toEvent(raw map[string]any, line string) events.Event {
	typ, _ := raw["type"].(string)
	if typ == "" {
		typ = "unknown"
	}
	tsStr, _ := raw["timestamp"].(string)
	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		ts, _ = time.Parse(time.RFC3339Nano, tsStr)
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
	}
	agentID, _ := raw["agent"].(string)
	if m, ok := raw["agent"].(map[string]any); ok {
		agentID, _ = m["id"].(string)
	}
	if agentID == "" {
		agentID = "generic"
	}
	id, _ := raw["id"].(string)
	if id == "" {
		id = events.NewID()
	}
	var payload json.RawMessage
	if p, ok := raw["payload"]; ok {
		b, _ := json.Marshal(p)
		payload = b
	} else if p, ok := raw["usage"]; ok {
		b, _ := json.Marshal(p)
		payload = b
	}
	b, _ := json.Marshal(raw)
	_ = line
	return events.Event{
		ID:            id,
		Type:          events.EventType(typ),
		Timestamp:     ts,
		Agent:         events.AgentRef{ID: agentID},
		Payload:       payload,
		Raw:           b,
		SchemaVersion: 1,
	}
}
