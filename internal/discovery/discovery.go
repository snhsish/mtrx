package discovery

import (
	"os"
	"os/exec"
	"path/filepath"
)

type AgentInfo struct {
	ID          string
	Name        string
	Detected    bool
	Reason      string
	Binary      string
	ConfigPaths []string
}

func KnownAgents() []AgentInfo {
	home, _ := os.UserHomeDir()
	return []AgentInfo{
		{ID: "opencode", Name: "OpenCode", Binary: "opencode", ConfigPaths: []string{filepath.Join(home, ".config", "opencode"), filepath.Join(home, ".local", "share", "opencode")}},
		{ID: "claude", Name: "Claude Code", Binary: "claude", ConfigPaths: []string{filepath.Join(home, ".claude"), filepath.Join(home, ".config", "claude")}},
		{ID: "codex", Name: "Codex", Binary: "codex", ConfigPaths: []string{filepath.Join(home, ".codex"), filepath.Join(home, ".config", "codex")}},
		{ID: "aider", Name: "Aider", Binary: "aider", ConfigPaths: []string{filepath.Join(home, ".aider")}},
		{ID: "gemini", Name: "Gemini CLI", Binary: "gemini", ConfigPaths: []string{filepath.Join(home, ".gemini"), filepath.Join(home, ".config", "gemini")}},
		{ID: "cursor", Name: "Cursor", Binary: "cursor", ConfigPaths: []string{filepath.Join(home, ".cursor")}},
	}
}

func Detect() []AgentInfo {
	agents := KnownAgents()
	for i := range agents {
		detected, reason := detectOne(agents[i])
		agents[i].Detected = detected
		agents[i].Reason = reason
	}
	return agents
}

func detectOne(a AgentInfo) (bool, string) {
	if _, err := exec.LookPath(a.Binary); err == nil {
		return true, "binary found: " + a.Binary
	}
	for _, p := range a.ConfigPaths {
		if _, err := os.Stat(p); err == nil {
			return true, "config found: " + p
		}
	}
	return false, "not found"
}
