package events

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	e := Event{ID: "1", Type: GenerationCompleted, Timestamp: time.Now(), Agent: AgentRef{ID: "a"}}
	if err := Validate(e); err != nil {
		t.Fatalf("unexpected %v", err)
	}
	e.ID = ""
	if err := Validate(e); err == nil {
		t.Fatal("expected error")
	}
}

func TestTokenTotal(t *testing.T) {
	u := TokenUsage{Input: 10, Output: 20, Reasoning: 5}
	u.ComputeTotal()
	if u.Total != 35 {
		t.Fatalf("got %d", u.Total)
	}
}

func TestIsValidType(t *testing.T) {
	if !IsValidType(ToolCompleted) {
		t.Fatal("expected valid")
	}
	if IsValidType("fake") {
		t.Fatal("expected invalid")
	}
}
