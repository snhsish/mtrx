package database

import "testing"

func TestOpen(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Verify(db); err != nil {
		t.Fatal(err)
	}
}

func TestQueryFiltered(t *testing.T) {
	db, _ := Open(t.TempDir() + "/q.db")
	defer db.Close()
	db.Exec(`INSERT INTO events(id,type,timestamp,agent) VALUES('1','tool.completed','2026-01-01T00:00:00Z','opencode')`)
	rows, err := QueryEventsFiltered(db, EventFilter{Agent: "opencode", Limit: 10})
	if err != nil || len(rows) != 1 {
		t.Fatalf("query %v %v", err, rows)
	}
}
