package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "none.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Host != "localhost" || cfg.Server.Port != 6767 {
		t.Fatalf("cfg = %+v", cfg.Server)
	}
	if !cfg.Collectors.Enabled || cfg.Retention.Enabled {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadOverrides(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("server:\n  host: 127.0.0.1\n  port: 9999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9999 || cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("cfg = %+v", cfg.Server)
	}
}

func TestPaths(t *testing.T) {
	cfg := Default()
	cfg.Storage.Path = "/tmp/mtrx-test"
	if DBPath(cfg) != "/tmp/mtrx-test/mtrx.db" {
		t.Fatalf("db = %q", DBPath(cfg))
	}
	if !strings.HasSuffix(PIDPath(cfg), "mtrx.pid") {
		t.Fatalf("pid = %q", PIDPath(cfg))
	}
	empty := Config{}
	if StoragePath(empty) != Default().Storage.Path {
		t.Fatal("expected default storage fallback")
	}
}

func TestShouldPrune(t *testing.T) {
	if (RetentionConfig{Enabled: true, Days: 90}).ShouldPrune() != true {
		t.Fatal("expected prune")
	}
	if (RetentionConfig{Enabled: true}).ShouldPrune() {
		t.Fatal("days=0 should not prune")
	}
	if (RetentionConfig{Enabled: false, Days: 90}).ShouldPrune() {
		t.Fatal("disabled should not prune")
	}
}
