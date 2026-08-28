package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func WriteJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			agent TEXT NOT NULL,
			session TEXT,
			project TEXT,
			payload TEXT,
			source TEXT,
			raw TEXT,
			schema_version INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_events_agent ON events(agent)`,
		`CREATE INDEX IF NOT EXISTS idx_events_session ON events(session)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_agent_ts ON events(agent, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_events_project_ts ON events(project, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type_ts ON events(type, timestamp)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			agent_id TEXT,
			project_id TEXT,
			started_at TEXT,
			ended_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			path TEXT,
			name TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT,
			version TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tool_calls (
			id TEXT PRIMARY KEY,
			event_id TEXT REFERENCES events(id),
			tool TEXT NOT NULL,
			status TEXT,
			duration_ms INTEGER,
			timestamp TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_calls_tool ON tool_calls(tool)`,
		`CREATE TABLE IF NOT EXISTS generations (
			id TEXT PRIMARY KEY,
			event_id TEXT REFERENCES events(id),
			model TEXT,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			cached_tokens INTEGER DEFAULT 0,
			reasoning_tokens INTEGER DEFAULT 0,
			latency_ms INTEGER,
			timestamp TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			event_id TEXT REFERENCES events(id),
			role TEXT NOT NULL,
			timestamp TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS file_changes (
			id TEXT PRIMARY KEY,
			event_id TEXT REFERENCES events(id),
			path TEXT NOT NULL,
			op TEXT NOT NULL,
			timestamp TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS git_events (
			id TEXT PRIMARY KEY,
			event_id TEXT REFERENCES events(id),
			op TEXT NOT NULL,
			branch TEXT,
			timestamp TEXT
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	_, _ = db.Exec(`INSERT OR IGNORE INTO schema_version(version, applied_at) VALUES(1, datetime('now'))`)
	_, _ = db.Exec(`UPDATE events SET agent='opencode' WHERE agent IN ('build','explore','general','plan')`)
	return nil
}

type Stats struct {
	EventCount   int64
	SessionCount int64
	ProjectCount int64
	AgentCount   int64
	OldestEvent  sql.NullString
	NewestEvent  sql.NullString
	DBSize       int64
}

func GetStats(db *sql.DB, dbPath string) (Stats, error) {
	var s Stats
	_ = db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&s.EventCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&s.SessionCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&s.ProjectCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&s.AgentCount)
	_ = db.QueryRow(`SELECT MIN(timestamp), MAX(timestamp) FROM events`).Scan(&s.OldestEvent, &s.NewestEvent)
	if fi, err := os.Stat(dbPath); err == nil {
		s.DBSize = fi.Size()
		if wal, err := os.Stat(dbPath + "-wal"); err == nil {
			s.DBSize += wal.Size()
		}
		if shm, err := os.Stat(dbPath + "-shm"); err == nil {
			s.DBSize += shm.Size()
		}
	}
	return s, nil
}

func Vacuum(db *sql.DB) error {
	_, err := db.Exec(`VACUUM`)
	return err
}

func Backup(db *sql.DB, srcPath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf(`VACUUM INTO '%s'`, destPath)); err == nil {
		return nil
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func PruneRetention(db *sql.DB, days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	res, err := db.Exec(`DELETE FROM events WHERE timestamp < datetime('now', ?)`, fmt.Sprintf("-%d days", days))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func Verify(db *sql.DB) error {
	var result string
	err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result)
	if err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity_check: %s", result)
	}
	return nil
}

func QueryEvents(db *sql.DB, limit int, agent, session, eventType string) ([]map[string]any, error) {
	return QueryEventsFiltered(db, EventFilter{Limit: limit, Agent: agent, Session: session, Type: eventType})
}

type EventFilter struct {
	Limit   int
	Agent   string
	Session string
	Type    string
	Project string
	Model   string
	From    string
	To      string
}

func QueryEventsFiltered(db *sql.DB, f EventFilter) ([]map[string]any, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	q := `SELECT id, type, timestamp, agent, session, project, payload FROM events WHERE 1=1`
	args := []any{}
	if f.Agent != "" {
		q += ` AND agent = ?`
		args = append(args, f.Agent)
	}
	if f.Session != "" {
		q += ` AND session = ?`
		args = append(args, f.Session)
	}
	if f.Type != "" {
		q += ` AND type = ?`
		args = append(args, f.Type)
	}
	if f.Project != "" {
		q += ` AND project = ?`
		args = append(args, f.Project)
	}
	if f.Model != "" {
		q += ` AND COALESCE(json_extract(payload,'$.model'), json_extract(payload,'$.model_name'), '') = ?`
		args = append(args, f.Model)
	}
	if f.From != "" {
		q += ` AND timestamp >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		q += ` AND timestamp <= ?`
		args = append(args, f.To)
	}
	q += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, f.Limit)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, typ, ts, agentVal sql.NullString
		var sess, proj, payload sql.NullString
		if err := rows.Scan(&id, &typ, &ts, &agentVal, &sess, &proj, &payload); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id.String, "type": typ.String, "timestamp": ts.String,
			"agent": agentVal.String, "session": sess.String, "project": proj.String, "payload": payload.String,
		})
	}
	return out, nil
}
