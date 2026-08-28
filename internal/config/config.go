package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Storage    StorageConfig    `yaml:"storage"`
	Collectors CollectorsConfig `yaml:"collectors"`
	Retention  RetentionConfig  `yaml:"retention"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type StorageConfig struct {
	Path string `yaml:"path"`
}

type CollectorsConfig struct {
	Enabled bool `yaml:"enabled"`
}

type RetentionConfig struct {
	Enabled bool `yaml:"enabled"`
	Days    int  `yaml:"days"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 6767,
		},
		Storage: StorageConfig{
			Path: filepath.Join(home, ".local", "share", "mtrx"),
		},
		Collectors: CollectorsConfig{
			Enabled: true,
		},
		Retention: RetentionConfig{
			Enabled: false,
		},
	}
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mtrx", "config.yaml")
	}
	return filepath.Join(home, ".config", "mtrx", "config.yaml")
}

func StoragePath(cfg Config) string {
	if cfg.Storage.Path != "" {
		return cfg.Storage.Path
	}
	return Default().Storage.Path
}

func DBPath(cfg Config) string {
	return filepath.Join(StoragePath(cfg), "mtrx.db")
}

func PIDPath(cfg Config) string {
	return filepath.Join(StoragePath(cfg), "mtrx.pid")
}

func LogPath(cfg Config) string {
	home, _ := os.UserHomeDir()
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "mtrx", "mtrx.log")
	}
	return filepath.Join(home, ".local", "state", "mtrx", "mtrx.log")
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (r RetentionConfig) ShouldPrune() bool { return r.Enabled && r.Days > 0 }

func EnsureDirs(cfg Config) error {
	for _, p := range []string{
		StoragePath(cfg),
		filepath.Dir(LogPath(cfg)),
		filepath.Dir(DefaultPath()),
	} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
	}
	return nil
}
