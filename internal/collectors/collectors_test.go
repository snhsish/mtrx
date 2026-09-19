package collectors

import (
	"context"
	"testing"
	"time"

	"mtrx/internal/agents"
	"mtrx/internal/database"
	"mtrx/internal/events"
)

type fakeAdapter struct {
	evs []events.Event
}

func (f *fakeAdapter) ID() string   { return "fake" }
func (f *fakeAdapter) Name() string { return "Fake" }
func (f *fakeAdapter) Detect(ctx context.Context) (bool, error) {
	return true, nil
}
func (f *fakeAdapter) Sources() []agents.Source {
	return []agents.Source{{Path: "fake", Adapter: "fake"}}
}
func (f *fakeAdapter) Collect(ctx context.Context, _ agents.Source) ([]events.Event, error) {
	return f.evs, nil
}
func (f *fakeAdapter) Watch(ctx context.Context, _ agents.Source, _ func(events.Event)) error {
	<-ctx.Done()
	return nil
}

func validEvent(id string) events.Event {
	return events.Event{
		ID:        id,
		Type:      events.GenerationCompleted,
		Timestamp: time.Now().UTC(),
		Agent:     events.AgentRef{ID: "fake"},
	}
}

func eventCount(t *testing.T, c *Collector) int {
	t.Helper()
	var n int
	if err := c.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCollectOnceStoresValidEvents(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/c.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := New(db, []agents.Adapter{&fakeAdapter{evs: []events.Event{validEvent("a"), validEvent("b")}}})
	if err := c.collectOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := eventCount(t, c); n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
	if err := c.collectOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := eventCount(t, c); n != 2 {
		t.Fatalf("rerun should be idempotent, rows = %d", n)
	}
}

func TestCollectOnceSkipsInvalid(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/c.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := New(db, []agents.Adapter{&fakeAdapter{evs: []events.Event{{}, validEvent("ok")}}})
	if err := c.collectOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := eventCount(t, c); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}
