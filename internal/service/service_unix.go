//go:build !windows

package service

import (
	"os"
	"syscall"
)

// IsRunning checks if the process with the given PID is running
func (m *Manager) IsRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix systems, FindProcess always succeeds, so we send signal 0 to check existence
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
