package collectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mtrx/internal/agents"
	"mtrx/internal/events"
)

type Collector struct {
	db       *sql.DB
	adapters []agents.Adapter
	interval time.Duration
}

func New(db *sql.DB, adapters []agents.Adapter) *Collector {
	return &Collector{db: db, adapters: adapters, interval: 5 * time.Second}
}

func (c *Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	if err := c.collectOnce(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "collector: %v\n", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.collectOnce(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "collector: %v\n", err)
			}
		}
	}
}

func (c *Collector) collectOnce(ctx context.Context) error {
	for _, a := range c.adapters {
		if ok, _ := a.Detect(ctx); !ok {
			continue
		}
		for _, src := range a.Sources() {
			evs, err := a.Collect(ctx, src)
			if err != nil {
				continue
			}
			for _, ev := range evs {
				if err := events.Validate(ev); err != nil {
					continue
				}
				raw, _ := json.Marshal(ev)
				payload := string(ev.Payload)
				agent := ev.Agent.ID
				sess := ""
				if ev.Session != nil {
					sess = ev.Session.ID
				}
				proj := ""
				if ev.Project != nil {
					proj = ev.Project.Path
				}
				ts := ev.Timestamp.UTC().Format(time.RFC3339)
				_, _ = c.db.Exec(`INSERT INTO events(id,type,timestamp,agent,session,project,payload,raw) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET type=excluded.type, timestamp=excluded.timestamp, agent=excluded.agent, session=excluded.session, project=excluded.project, payload=excluded.payload, raw=excluded.raw`,
					ev.ID, string(ev.Type), ts, agent, sess, proj, payload, string(raw))
			}
		}
	}
	return nil
}
