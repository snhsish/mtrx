package system

import (
	"fmt"
	"os"
	"strconv"
)

func WritePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

func ReadPID(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(b))
}

func RemovePID(path string) error {
	return os.Remove(path)
}

func Status(pidPath string) (bool, int) {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return false, 0
	}
	if IsRunning(pid) {
		return true, pid
	}
	return false, pid
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
