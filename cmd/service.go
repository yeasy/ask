package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/service"
	"github.com/yeasy/ask/internal/ui"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the ASK background service (start, stop, status)",
	Long:  `Manage the ASK background service. Allows running the web server in the background.`,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the ASK service in background",
	Run:   runServiceStart,
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the ASK service",
	Run:   runServiceStop,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the ASK service",
	Run:   runServiceStatus,
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the ASK service",
	Run:   runServiceRestart,
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(startCmd)
	serviceCmd.AddCommand(stopCmd)
	serviceCmd.AddCommand(statusCmd)
	serviceCmd.AddCommand(restartCmd)
}

func getServiceManager() *service.Manager {
	// Find home directory config path
	home, err := os.UserHomeDir()
	if err != nil {
		ui.Error("Failed to get home directory: " + err.Error())
		os.Exit(1)
	}
	configDir := filepath.Join(home, ".ask")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		ui.Error("Failed to create config directory: " + err.Error())
		os.Exit(1)
	}
	return service.NewManager(configDir)
}

func runServiceStart(_ *cobra.Command, _ []string) {
	mgr := getServiceManager()

	pid, running, err := mgr.GetStatus()
	if err != nil {
		ui.Error("Failed to check service status: " + err.Error())
		return
	}

	if running {
		fmt.Printf("Service is already running (PID: %d)\n", pid)
		return
	}

	fmt.Printf("Starting ASK service...\n")

	// Prepare the command to run "ask serve"
	// We use os.Executable to find the current binary
	exe, err := os.Executable()
	if err != nil {
		ui.Error("Failed to find executable: " + err.Error())
		return
	}

	launchArgs := []string{"serve", "--no-open"}
	// Pass through flags if needed, for now we presume default config or flags set via files

	bgCmd := exec.Command(exe, launchArgs...)

	// Open log file
	logPath := mgr.GetLogFilePath()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		ui.Error("Failed to open log file: " + err.Error())
		return
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		_ = logFile.Close()
		ui.Error("Failed to open /dev/null: " + err.Error())
		return
	}
	bgCmd.Stdin = devNull
	bgCmd.Stdout = logFile
	bgCmd.Stderr = logFile

	// Create new process group to ensure it survives parent exit (Unix only)
	bgCmd.SysProcAttr = sysProcAttr()

	if err := bgCmd.Start(); err != nil {
		_ = devNull.Close()
		_ = logFile.Close()
		ui.Error("Failed to start service: " + err.Error())
		return
	}
	// Close file handles in the parent process now that the child has inherited them
	_ = devNull.Close()
	_ = logFile.Close()

	if err := mgr.WritePID(bgCmd.Process.Pid); err != nil {
		ui.Error("Failed to write PID file: " + err.Error())
		// Try to kill the process since we failed to track it
		_ = bgCmd.Process.Kill()
		_ = bgCmd.Wait()
		return
	}

	fmt.Printf("Service started successfully (PID: %d)\n", bgCmd.Process.Pid)
	fmt.Printf("Logs available at: %s\n", logPath)
}

func runServiceStop(_ *cobra.Command, _ []string) {
	mgr := getServiceManager()
	pid, running, err := mgr.GetStatus()
	if err != nil {
		ui.Error("Error checking status: " + err.Error())
		return
	}

	if !running {
		fmt.Printf("Service is not running.\n")
		return
	}

	fmt.Printf("Stopping service (PID: %d)...\n", pid)

	process, err := os.FindProcess(pid)
	if err == nil {
		// Try graceful shutdown
		_ = signalTerm(pid)

		// Poll until process exits or timeout (server uses a 5s shutdown internally)
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) && mgr.IsRunning(pid) {
			time.Sleep(200 * time.Millisecond)
		}
		if mgr.IsRunning(pid) {
			_ = process.Kill()
			_, _ = process.Wait()
		}
	}

	_ = mgr.ClearPID()
	fmt.Printf("Service stopped.\n")
}

func runServiceStatus(_ *cobra.Command, _ []string) {
	mgr := getServiceManager()
	pid, running, err := mgr.GetStatus()
	if err != nil {
		ui.Error("Error checking status: " + err.Error())
		return
	}

	if running {
		fmt.Printf("Service is running (PID: %d)\n", pid)
		fmt.Printf("Log file: %s\n", mgr.GetLogFilePath())
	} else {
		fmt.Printf("Service is not running.\n")
	}
}

func runServiceRestart(cmd *cobra.Command, args []string) {
	runServiceStop(cmd, args)
	time.Sleep(1 * time.Second) // Give it a moment to release ports
	runServiceStart(cmd, args)
}
