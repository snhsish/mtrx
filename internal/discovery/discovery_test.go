package discovery

import "testing"

func TestKnownAgents(t *testing.T) {
	known := KnownAgents()
	if len(known) == 0 {
		t.Fatal("expected known agents")
	}
	seen := map[string]bool{}
	for _, a := range known {
		if a.ID == "" || a.Name == "" {
			t.Fatalf("agent = %+v", a)
		}
		if seen[a.ID] {
			t.Fatalf("duplicate id %q", a.ID)
		}
		seen[a.ID] = true
	}
}

func TestDetectCoversKnown(t *testing.T) {
	got := Detect()
	if len(got) != len(KnownAgents()) {
		t.Fatalf("got %d, want %d", len(got), len(KnownAgents()))
	}
	for _, a := range got {
		if a.Reason == "" {
			t.Fatalf("missing reason for %q", a.ID)
		}
	}
}
