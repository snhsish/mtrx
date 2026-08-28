package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"mtrx/agents/aider"
	"mtrx/agents/claude"
	"mtrx/agents/codex"
	"mtrx/agents/cursor"
	"mtrx/agents/gemini"
	"mtrx/agents/generic"
	"mtrx/agents/opencode"
	"mtrx/internal/agents"
	"mtrx/internal/collectors"
	"mtrx/internal/config"
	"mtrx/internal/database"
	"mtrx/internal/discovery"
	"mtrx/internal/server"
	"mtrx/internal/system"
)

var (
	version  = "0.1.0"
	cfgFile  string
	logLevel string
	storage  string
	cfg      config.Config
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cfg = config.Default()
	root := &cobra.Command{
		Use:   "mtrx",
		Short: "Local-first telemetry for AI agents",
		Long:  `mtrx - local-first, unlimited, open-source telemetry and analytics for AI agents.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(cmd, args)
		},
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (error|warn|info|debug|trace)")
	root.PersistentFlags().StringVar(&storage, "storage", "", "storage path override")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		loaded, err := config.Load(cfgFile)
		if err != nil {
			return err
		}
		cfg = loaded
		if storage != "" {
			cfg.Storage.Path = storage
		}
		return config.EnsureDirs(cfg)
	}

	root.AddCommand(newStartCmd())
	root.AddCommand(newStopCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newAgentsCmd())
	root.AddCommand(newSessionsCmd())
	root.AddCommand(newEventsCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newDatabaseCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newOpenCmd())

	return root
}

func openDB() (*sql.DB, error) {
	return database.Open(config.DBPath(cfg))
}

func newStartCmd() *cobra.Command {
	var port int
	var host string
	var foreground bool
	c := &cobra.Command{
		Use:   "start",
		Short: "Start the local dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			if port != 0 {
				cfg.Server.Port = port
			}
			if host != "" {
				cfg.Server.Host = host
			}
			if foreground {
				return serveForeground()
			}
			return runStart(cmd, args)
		},
	}
	c.Flags().IntVar(&port, "port", 0, "port to listen on")
	c.Flags().StringVar(&host, "host", "", "host to bind")
	c.Flags().BoolVarP(&foreground, "foreground", "f", false, "run in foreground")
	return c
}

func runStart(cmd *cobra.Command, args []string) error {
	pidPath := config.PIDPath(cfg)
	if ok, pid := system.Status(pidPath); ok {
		fmt.Printf("mtrx already running (pid %d) at http://%s:%d\n", pid, cfg.Server.Host, cfg.Server.Port)
		return nil
	}
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()

	srv := server.New(cfg, db)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}

	if isDaemonized() {
		fmt.Printf("Starting mtrx at http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
		return httpSrv.ListenAndServe()
	}

	exe, _ := os.Executable()
	daemon := exec.Command(exe, "start", "--foreground")
	daemon.Env = os.Environ()
	daemon.Stdout = os.Stdout
	daemon.Stderr = os.Stderr
	if err := daemon.Start(); err != nil {
		return serveForegroundWithDB(db, addr, srv)
	}
	_ = system.WritePID(pidPath, daemon.Process.Pid)
	time.Sleep(400 * time.Millisecond)
	if ok, _ := system.Status(pidPath); ok {
		fmt.Printf("mtrx started at http://%s:%d (pid %d)\n", cfg.Server.Host, cfg.Server.Port, daemon.Process.Pid)
	} else {
		fmt.Printf("mtrx started at http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	}
	return nil
}

func isDaemonized() bool {
	for _, a := range os.Args {
		if a == "--foreground" || a == "-f" {
			return true
		}
	}
	return false
}

func serveForeground() error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	srv := server.New(cfg, db)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	return serveForegroundWithDB(db, addr, srv)
}

func serveForegroundWithDB(db *sql.DB, addr string, srv *server.Server) error {
	_ = system.WritePID(config.PIDPath(cfg), os.Getpid())
	defer os.Remove(config.PIDPath(cfg))
	if cfg.Collectors.Enabled {
		adapters := []agents.Adapter{
			&opencode.Adapter{},
			&claude.Adapter{},
			&codex.Adapter{},
			&cursor.Adapter{},
			&aider.Adapter{},
			&gemini.Adapter{},
			&generic.Adapter{},
		}
		col := collectors.New(db, adapters)
		go func() {
			if err := col.Run(context.Background()); err != nil && err != context.Canceled {
				fmt.Fprintf(os.Stderr, "collector stopped: %v\n", err)
			}
		}()
	}
	fmt.Printf("mtrx listening on http://%s\n", addr)
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}
	return httpSrv.ListenAndServe()
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			pidPath := config.PIDPath(cfg)
			ok, pid := system.Status(pidPath)
			if !ok {
				fmt.Println("mtrx not running")
				return nil
			}
			if err := system.Kill(pid); err != nil {
				return err
			}
			for i := 0; i < 10; i++ {
				time.Sleep(200 * time.Millisecond)
				if ok, _ := system.Status(pidPath); !ok {
					break
				}
			}
			_ = system.RemovePID(pidPath)
			fmt.Println("mtrx stopped")
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := config.DBPath(cfg)
			pidPath := config.PIDPath(cfg)
			ok, pid := system.Status(pidPath)
			status := "stopped"
			if ok {
				status = "running"
			}
			db, err := database.Open(dbPath)
			var stats database.Stats
			if err == nil {
				stats, _ = database.GetStats(db, dbPath)
				db.Close()
			}
			agents := discovery.Detect()
			fmt.Println("mtrx")
			fmt.Println("──────────────────────────────")
			fmt.Printf("%-12s %s\n", "Status", status)
			if ok {
				fmt.Printf("%-12s %d\n", "PID", pid)
			}
			fmt.Printf("%-12s http://%s:%d\n", "Dashboard", cfg.Server.Host, cfg.Server.Port)
			fmt.Printf("%-12s %s\n", "Database", system.FormatBytes(stats.DBSize))
			fmt.Printf("%-12s %d\n", "Events", stats.EventCount)
			fmt.Printf("%-12s %d\n", "Sessions", stats.SessionCount)
			fmt.Println()
			fmt.Println("Collectors")
			fmt.Println()
			for _, a := range agents {
				mark := "✗"
				if a.Detected {
					mark = "✓"
				}
				fmt.Printf("%s %s\n", mark, a.Name)
			}
			return nil
		},
	}
}

func newAgentsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "agents",
		Aliases: []string{"agent"},
		Short:   "Manage agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listAgents()
		},
	}
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List detected agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listAgents()
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "detect",
		Short: "Detect installed agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listAgents()
		},
	})
	return c
}

func listAgents() error {
	agents := discovery.Detect()
	fmt.Printf("%-12s %-10s %s\n", "ID", "DETECTED", "NAME")
	for _, a := range agents {
		det := "no"
		if a.Detected {
			det = "yes"
		}
		fmt.Printf("%-12s %-10s %s  (%s)\n", a.ID, det, a.Name, a.Reason)
	}
	return nil
}

func newSessionsCmd() *cobra.Command {
	var limit int
	var jsonOut bool
	c := &cobra.Command{
		Use:   "sessions",
		Short: "List sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(`SELECT id, agent_id, project_id, started_at, ended_at FROM sessions ORDER BY started_at DESC LIMIT ?`, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				ID, Agent, Project, Started, Ended string
			}
			var out []row
			for rows.Next() {
				var id, agent, proj, started, ended sql.NullString
				_ = rows.Scan(&id, &agent, &proj, &started, &ended)
				out = append(out, row{id.String, agent.String, proj.String, started.String, ended.String})
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if len(out) == 0 {
				fmt.Println("No sessions found")
				return nil
			}
			fmt.Printf("%-36s %-12s %-20s %s\n", "ID", "AGENT", "STARTED", "PROJECT")
			for _, r := range out {
				fmt.Printf("%-36s %-12s %-20s %s\n", r.ID, r.Agent, r.Started, r.Project)
			}
			return nil
		},
	}
	c.Flags().IntVar(&limit, "limit", 50, "limit")
	c.Flags().BoolVar(&jsonOut, "json", false, "json output")
	return c
}

func newEventsCmd() *cobra.Command {
	var limit int
	var agent, session, typ, jsonOut string
	c := &cobra.Command{
		Use:   "events",
		Short: "Inspect stored events",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = jsonOut
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			events, err := database.QueryEvents(db, limit, agent, session, typ)
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(events)
			}
			if len(events) == 0 {
				fmt.Println("No events found")
				return nil
			}
			fmt.Printf("%-36s %-22s %-16s %s\n", "ID", "TIMESTAMP", "TYPE", "AGENT")
			for _, e := range events {
				fmt.Printf("%-36s %-22v %-22v %v\n", e["id"], e["timestamp"], e["type"], e["agent"])
			}
			return nil
		},
	}
	c.Flags().IntVar(&limit, "limit", 50, "limit")
	c.Flags().StringVar(&agent, "agent", "", "filter by agent")
	c.Flags().StringVar(&session, "session", "", "filter by session")
	c.Flags().StringVar(&typ, "type", "", "filter by event type")
	c.Flags().Bool("json", false, "json output")
	return c
}

func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <path>",
		Short: "Import historical agent data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			fi, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("path not found: %s", path)
			}
			var files []string
			if fi.IsDir() {
				_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if !info.IsDir() && (strings.HasSuffix(p, ".jsonl") || strings.HasSuffix(p, ".json")) {
						files = append(files, p)
					}
					return nil
				})
				if len(files) == 0 {
					fmt.Printf("No importable files found in %s\n", path)
					return nil
				}
			} else {
				files = []string{path}
			}
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			total := 0
			for _, f := range files {
				n, err := importFile(db, f)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: %s: %v\n", f, err)
					continue
				}
				total += n
				fmt.Printf("Imported %d events from %s\n", n, f)
			}
			fmt.Printf("Done. %d events total.\n", total)
			return nil
		},
	}
}

func importFile(db *sql.DB, path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	count := 0
	tx, _ := db.Begin()
	stmt, _ := tx.Prepare(`INSERT OR IGNORE INTO events(id,type,timestamp,agent,session,project,payload,raw) VALUES(?,?,?,?,?,?,?,?)`)
	if stmt == nil {
		return 0, fmt.Errorf("prepare failed")
	}
	defer stmt.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	isJSONL := strings.HasSuffix(path, ".jsonl")
	if isJSONL {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var raw map[string]any
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}
			if err := insertEvent(stmt, raw, line); err == nil {
				count++
			}
		}
	} else {
		var data any
		b, _ := os.ReadFile(path)
		if err := json.Unmarshal(b, &data); err != nil {
			return 0, err
		}
		switch v := data.(type) {
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					raw, _ := json.Marshal(m)
					if err := insertEvent(stmt, m, string(raw)); err == nil {
						count++
					}
				}
			}
		case map[string]any:
			raw, _ := json.Marshal(v)
			if err := insertEvent(stmt, v, string(raw)); err == nil {
				count++
			}
		}
	}
	_ = tx.Commit()
	return count, scanner.Err()
}

func insertEvent(stmt *sql.Stmt, raw map[string]any, rawStr string) error {
	id, _ := raw["id"].(string)
	if id == "" {
		if v, ok := raw["ID"].(string); ok {
			id = v
		}
	}
	typ, _ := raw["type"].(string)
	if typ == "" {
		if v, ok := raw["event_type"].(string); ok {
			typ = v
		} else {
			typ = "unknown"
		}
	}
	ts, _ := raw["timestamp"].(string)
	if ts == "" {
		if v, ok := raw["time"].(string); ok {
			ts = v
		}
	}
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		if t2, err2 := time.Parse(time.RFC3339Nano, ts); err2 == nil {
			ts = t2.Format(time.RFC3339)
		} else {
			ts = time.Now().UTC().Format(time.RFC3339)
		}
	}
	agent := ""
	if a, ok := raw["agent"].(string); ok {
		agent = a
	} else if a, ok := raw["agent"].(map[string]any); ok {
		agent, _ = a["id"].(string)
	}
	if agent == "" {
		agent = "unknown"
	}
	session := ""
	if s, ok := raw["session"].(string); ok {
		session = s
	} else if s, ok := raw["session"].(map[string]any); ok {
		session, _ = s["id"].(string)
	}
	project := ""
	if p, ok := raw["project"].(string); ok {
		project = p
	} else if p, ok := raw["project"].(map[string]any); ok {
		project, _ = p["id"].(string)
	}
	if id == "" {
		h := sha256.Sum256([]byte(typ + "|" + ts + "|" + agent + "|" + session + "|" + rawStr))
		id = fmt.Sprintf("%x", h[:16])
		if id == "" {
			id = uuid.NewString()
		}
	}
	payload := ""
	if p, ok := raw["payload"]; ok {
		b, _ := json.Marshal(p)
		payload = string(b)
	} else if p, ok := raw["usage"]; ok {
		b, _ := json.Marshal(p)
		payload = string(b)
	} else if p, ok := raw["tokens"]; ok {
		b, _ := json.Marshal(p)
		payload = string(b)
	}
	_, err := stmt.Exec(id, typ, ts, agent, session, project, payload, rawStr)
	return err
}

func newExportCmd() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "export [path]",
		Short: "Export telemetry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			events, err := database.QueryEvents(db, 1000000, "", "", "")
			if err != nil {
				return err
			}
			var out *os.File
			if len(args) == 1 && args[0] != "-" {
				out, err = os.Create(args[0])
				if err != nil {
					return err
				}
				defer out.Close()
			} else {
				out = os.Stdout
			}
			switch format {
			case "json":
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(events)
			case "csv":
				w := csv.NewWriter(out)
				_ = w.Write([]string{"id", "type", "timestamp", "agent", "session", "project"})
				for _, e := range events {
					_ = w.Write([]string{fmt.Sprint(e["id"]), fmt.Sprint(e["type"]), fmt.Sprint(e["timestamp"]), fmt.Sprint(e["agent"]), fmt.Sprint(e["session"]), fmt.Sprint(e["project"])})
				}
				w.Flush()
				return w.Error()
			default:
				for _, e := range events {
					b, _ := json.Marshal(e)
					fmt.Fprintln(out, string(b))
				}
				return nil
			}
		},
	}
	c.Flags().StringVar(&format, "format", "jsonl", "format: json|jsonl|csv")
	return c
}

func newDatabaseCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "database",
		Short: "Database operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return databaseStats()
		},
	}
	c.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Show database stats",
		RunE:  func(cmd *cobra.Command, args []string) error { return databaseStats() },
	})
	c.AddCommand(&cobra.Command{
		Use:   "vacuum",
		Short: "Vacuum database",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			fmt.Println("Vacuuming...")
			if err := database.Vacuum(db); err != nil {
				return err
			}
			fmt.Println("Done.")
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "backup <path>",
		Short: "Backup database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			dest := args[0]
			if err := database.Backup(db, config.DBPath(cfg), dest); err != nil {
				return err
			}
			fmt.Printf("Backup written to %s\n", dest)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "verify",
		Short: "Verify database integrity",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := database.Verify(db); err != nil {
				return err
			}
			fmt.Println("Database integrity: ok")
			return nil
		},
	})
	return c
}

func databaseStats() error {
	dbPath := config.DBPath(cfg)
	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	stats, _ := database.GetStats(db, dbPath)
	fmt.Printf("%-16s %s\n", "Database", dbPath)
	fmt.Printf("%-16s %s\n", "Size", system.FormatBytes(stats.DBSize))
	fmt.Printf("%-16s %d\n", "Events", stats.EventCount)
	fmt.Printf("%-16s %d\n", "Sessions", stats.SessionCount)
	fmt.Printf("%-16s %d\n", "Projects", stats.ProjectCount)
	fmt.Printf("%-16s %d\n", "Agents", stats.AgentCount)
	if stats.OldestEvent.Valid {
		fmt.Printf("%-16s %s\n", "Oldest", stats.OldestEvent.String)
	}
	if stats.NewestEvent.Valid {
		fmt.Printf("%-16s %s\n", "Newest", stats.NewestEvent.String)
	}
	return nil
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("mtrx Doctor")
			fmt.Println()
			dbPath := config.DBPath(cfg)
			db, err := database.Open(dbPath)
			if err != nil {
				fmt.Printf("✗ Database: %v\n", err)
			} else {
				defer db.Close()
				var wal string
				_ = db.QueryRow(`PRAGMA journal_mode`).Scan(&wal)
				if strings.ToLower(wal) == "wal" {
					fmt.Println("✓ SQLite WAL enabled")
				} else {
					fmt.Printf("! SQLite journal_mode=%s (expected WAL)\n", wal)
				}
				if err := database.Verify(db); err != nil {
					fmt.Printf("✗ Database integrity: %v\n", err)
				} else {
					fmt.Println("✓ Database writable")
				}
				stats, _ := database.GetStats(db, dbPath)
				fmt.Printf("✓ Database accessible (%d events)\n", stats.EventCount)
			}
			addr := fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
			ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "GET", addr+"/api/health", nil)
			if _, err := http.DefaultClient.Do(req); err == nil {
				fmt.Println("✓ Dashboard accessible")
			} else {
				fmt.Println("! Dashboard not reachable (is mtrx running?)")
			}
			fmt.Println()
			fmt.Println("Agents")
			fmt.Println()
			for _, a := range discovery.Detect() {
				if a.Detected {
					fmt.Printf("✓ %s detected (%s)\n", a.Name, a.Reason)
				} else {
					fmt.Printf("✗ %s not detected\n", a.Name)
				}
			}
			fmt.Println()
			fmt.Println("Storage")
			fmt.Println()
			if db != nil {
				stats, _ := database.GetStats(db, dbPath)
				fmt.Printf("Events: %d\n", stats.EventCount)
				fmt.Printf("Database: %s\n", system.FormatBytes(stats.DBSize))
			}
			var free string
			if avail := diskFree(filepath.Dir(dbPath)); avail > 0 {
				free = system.FormatBytes(int64(avail))
			} else {
				free = "unknown"
			}
			fmt.Printf("Free disk: %s\n", free)
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("mtrx %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open dashboard in browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
			var c *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				c = exec.Command("open", url)
			case "windows":
				c = exec.Command("cmd", "/c", "start", url)
			default:
				c = exec.Command("xdg-open", url)
			}
			if err := c.Start(); err != nil {
				fmt.Println(url)
				return nil
			}
			fmt.Printf("Opening %s\n", url)
			return nil
		},
	}
}

// ensure strconv import used
var _ = strconv.Itoa
