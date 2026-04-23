// Package service manages the background process for the ASK web server.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yeasy/ask/internal/filesystem"
)

const (
	pidFileName = "ask.pid"
	logFileName = "ask.log"
)

// Manager handles service process management
type Manager struct {
	homeDir string
}

// NewManager creates a new service manager
func NewManager(homeDir string) *Manager {
	return &Manager{
		homeDir: homeDir,
	}
}

// GetPIDFilePath returns the path to the PID file
func (m *Manager) GetPIDFilePath() string {
	return filepath.Join(m.homeDir, pidFileName)
}

// GetLogFilePath returns the path to the log file
func (m *Manager) GetLogFilePath() string {
	return filepath.Join(m.homeDir, logFileName)
}

// WritePID writes the current process ID to the PID file
func (m *Manager) WritePID(pid int) error {
	pidFile := m.GetPIDFilePath()
	return filesystem.AtomicWriteFile(pidFile, []byte(strconv.Itoa(pid)), 0600)
}

// ReadPID reads the PID from the PID file
func (m *Manager) ReadPID() (int, error) {
	pidFile := m.GetPIDFilePath()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	if pid <= 0 {
		return 0, fmt.Errorf("invalid PID: %d", pid)
	}
	return pid, nil
}

// ClearPID removes the PID file
func (m *Manager) ClearPID() error {
	return os.Remove(m.GetPIDFilePath())
}

// GetStatus returns the status of the service (pid, running, error)
func (m *Manager) GetStatus() (int, bool, error) {
	pid, err := m.ReadPID()
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	running := m.IsRunning(pid)
	if !running {
		// Stale PID file
		_ = m.ClearPID()
		return 0, false, nil
	}

	return pid, true, nil
}
