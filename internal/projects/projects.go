package projects

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Project struct {
	ID     string
	Path   string
	Name   string
	Remote string
	Branch string
}

func Detect(path string) Project {
	abs, _ := filepath.Abs(path)
	name := filepath.Base(abs)
	id := hash(abs)
	remote, branch := gitInfo(abs)
	return Project{ID: id, Path: abs, Name: name, Remote: remote, Branch: branch}
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func gitInfo(dir string) (string, string) {
	remote := ""
	if out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output(); err == nil {
		remote = strings.TrimSpace(string(out))
	}
	branch := ""
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return remote, branch
	}
	return remote, branch
}
