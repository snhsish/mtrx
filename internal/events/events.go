package events

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	AgentStarted        EventType = "agent.started"
	AgentStopped        EventType = "agent.stopped"
	SessionStarted      EventType = "session.started"
	SessionEnded        EventType = "session.ended"
	MessageUser         EventType = "message.user"
	MessageAssistant    EventType = "message.assistant"
	MessageSystem       EventType = "message.system"
	GenerationStarted   EventType = "generation.started"
	GenerationCompleted EventType = "generation.completed"
	ToolStarted         EventType = "tool.started"
	ToolCompleted       EventType = "tool.completed"
	FileRead            EventType = "file.read"
	FileWrite           EventType = "file.write"
	FileCreate          EventType = "file.create"
	FileDelete          EventType = "file.delete"
	GitCommit           EventType = "git.commit"
	GitCheckout         EventType = "git.checkout"
	GitBranch           EventType = "git.branch"
	CommandStarted      EventType = "command.started"
	CommandCompleted    EventType = "command.completed"
	TestStarted         EventType = "test.started"
	TestCompleted       EventType = "test.completed"
	BuildStarted        EventType = "build.started"
	BuildCompleted      EventType = "build.completed"
	Error               EventType = "error"
	Retry               EventType = "retry"
	ContextUpdated      EventType = "context.updated"
	ModelChanged        EventType = "model.changed"
)

var validTypes = map[EventType]struct{}{
	AgentStarted: {}, AgentStopped: {}, SessionStarted: {}, SessionEnded: {},
	MessageUser: {}, MessageAssistant: {}, MessageSystem: {},
	GenerationStarted: {}, GenerationCompleted: {}, ToolStarted: {}, ToolCompleted: {},
	FileRead: {}, FileWrite: {}, FileCreate: {}, FileDelete: {},
	GitCommit: {}, GitCheckout: {}, GitBranch: {}, CommandStarted: {}, CommandCompleted: {},
	TestStarted: {}, TestCompleted: {}, BuildStarted: {}, BuildCompleted: {},
	Error: {}, Retry: {}, ContextUpdated: {}, ModelChanged: {},
}

func IsValidType(t EventType) bool {
	if _, ok := validTypes[t]; ok {
		return true
	}
	return false
}

type AgentRef struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Version  string `json:"version,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type SessionRef struct {
	ID string `json:"id"`
}

type ProjectRef struct {
	ID   string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
}

type SourceRef struct {
	Adapter string `json:"adapter"`
	Path    string `json:"path,omitempty"`
}

type Event struct {
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	Agent         AgentRef        `json:"agent"`
	Session       *SessionRef     `json:"session,omitempty"`
	Project       *ProjectRef     `json:"project,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Source        *SourceRef      `json:"source,omitempty"`
	Raw           json.RawMessage `json:"raw,omitempty"`
	SchemaVersion int             `json:"schema_version"`
}

type TokenUsage struct {
	Input       int64 `json:"input"`
	Output      int64 `json:"output"`
	CachedInput int64 `json:"cached_input"`
	CacheWrite  int64 `json:"cache_write"`
	Reasoning   int64 `json:"reasoning"`
	Total       int64 `json:"total"`
}

func (t *TokenUsage) ComputeTotal() {
	if t.Total == 0 {
		t.Total = t.Input + t.Output + t.CachedInput + t.CacheWrite + t.Reasoning
	}
}

func Validate(e Event) error {
	if e.ID == "" {
		return fmt.Errorf("missing id")
	}
	if e.Type == "" {
		return fmt.Errorf("missing type")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("missing timestamp")
	}
	if e.Agent.ID == "" {
		return fmt.Errorf("missing agent id")
	}
	return nil
}

func DeterministicID(source, ts, payload string) string {
	h := sha256.Sum256([]byte(source + "|" + ts + "|" + payload))
	return fmt.Sprintf("%x", h[:16])
}

func NewID() string { return uuid.NewString() }
