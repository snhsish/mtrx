package analytics

import (
	"database/sql"
	"testing"

	"mtrx/internal/database"
)

func seedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/a.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	rows := []struct{ id, typ, ts, agent, session, payload string }{
		{"e1", "generation.completed", "2026-01-01T10:00:00Z", "opencode", "s1", `{"input":100,"output":50}`},
		{"e2", "generation.completed", "2026-01-01T11:00:00Z", "opencode", "s1", `{"input":10,"output":5}`},
		{"e3", "generation.completed", "2026-01-02T10:00:00Z", "claude", "s2", `{"input":7,"output":3}`},
		{"e4", "tool.completed", "2026-01-02T11:00:00Z", "claude", "s2", ``},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO events(id,type,timestamp,agent,session,payload,raw) VALUES(?,?,?,?,?,?,?)`,
			r.id, r.typ, r.ts, r.agent, r.session, r.payload, "{}"); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestOverviewStatsFiltered(t *testing.T) {
	db := seedDB(t)
	o, err := OverviewStatsFiltered(db, Filter{Agent: "opencode"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Events != 2 || o.Sessions != 1 || o.Agents != 1 {
		t.Fatalf("overview = %+v", o)
	}
}

func TestTokensByDayFiltered(t *testing.T) {
	db := seedDB(t)
	buckets, err := TokensByDayFiltered(db, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 {
		t.Fatalf("buckets = %+v", buckets)
	}
	if buckets[0].Bucket != "2026-01-01" || buckets[0].Input != 110 || buckets[0].Output != 55 || buckets[0].Count != 2 {
		t.Fatalf("day1 = %+v", buckets[0])
	}
	if buckets[1].Bucket != "2026-01-02" || buckets[1].Input != 7 || buckets[1].Count != 1 {
		t.Fatalf("day2 = %+v", buckets[1])
	}
}

func TestActivityByDayFiltered(t *testing.T) {
	db := seedDB(t)
	buckets, err := ActivityByDayFiltered(db, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 2 || buckets[0].Count != 2 || buckets[1].Count != 2 {
		t.Fatalf("buckets = %+v", buckets)
	}
}

func TestTokensByDayEmpty(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/empty.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	buckets, err := TokensByDayFiltered(db, Filter{Agent: "nobody"})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 0 {
		t.Fatalf("buckets = %+v", buckets)
	}
}
