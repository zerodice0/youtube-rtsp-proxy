package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zerodice0/youtube-rtsp-proxy/internal/config"
)

// MediaMTXServer manages the MediaMTX RTSP server process
type MediaMTXServer struct {
	mu sync.Mutex

	config     *config.MediaMTXConfig
	serverCfg  *config.ServerConfig
	dataDir    string
	cmd        *exec.Cmd
	pid        int
	pidFile    string
	running    bool
	cancel     context.CancelFunc
}

// NewMediaMTXServer creates a new MediaMTX server manager
func NewMediaMTXServer(cfg *config.MediaMTXConfig, serverCfg *config.ServerConfig, dataDir string) *MediaMTXServer {
	return &MediaMTXServer{
		config:    cfg,
		serverCfg: serverCfg,
		dataDir:   dataDir,
		pidFile:   filepath.Join(dataDir, "mediamtx.pid"),
	}
}

// Start starts the MediaMTX server
func (s *MediaMTXServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Check if already running from previous session
	if s.isAlreadyRunning() {
		s.running = true
		return nil
	}

	// Ensure data directory exists
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create MediaMTX config file if needed
	configPath := s.getConfigPath()
	if err := s.ensureConfig(configPath); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	// Create cancellable context
	procCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Build command
	args := []string{}
	if configPath != "" {
		args = append(args, configPath)
	}

	cmd := exec.CommandContext(procCtx, s.config.BinaryPath, args...)

	// Log file
	logFile, err := os.OpenFile(
		filepath.Join(s.dataDir, "mediamtx.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Ensure process gets its own process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		logFile.Close()
		return fmt.Errorf("failed to start mediamtx: %w", err)
	}

	s.cmd = cmd
	s.pid = cmd.Process.Pid
	s.running = true

	// Save PID file
	if err := os.WriteFile(s.pidFile, []byte(fmt.Sprintf("%d", s.pid)), 0644); err != nil {
		// Non-fatal error
		fmt.Fprintf(os.Stderr, "warning: failed to write PID file: %v\n", err)
	}

	// Wait for server to be ready
	if err := s.waitForReady(5 * time.Second); err != nil {
		s.stopLocked() // Use stopLocked to avoid mutex deadlock
		return fmt.Errorf("mediamtx failed to start: %w", err)
	}

	// Monitor process in background
	go func() {
		cmd.Wait()
		logFile.Close()
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	return nil
}

// Stop stops the MediaMTX server
func (s *MediaMTXServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.stopLocked()
}

// stopLocked performs the actual stop logic without acquiring the mutex.
// Must be called while holding s.mu.
func (s *MediaMTXServer) stopLocked() error {
	if !s.running {
		return nil
	}

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Try graceful shutdown
	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			if !strings.Contains(err.Error(), "process already finished") {
				s.cmd.Process.Kill()
			}
		}

		// Wait for process to exit
		done := make(chan struct{})
		go func() {
			s.cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			s.cmd.Process.Kill()
		}
	}

	// Also kill by PID if needed (for processes from previous sessions)
	if s.pid > 0 {
		if process, err := os.FindProcess(s.pid); err == nil {
			process.Signal(syscall.SIGTERM)
			time.Sleep(500 * time.Millisecond)
			process.Kill()
		}
	}

	// Remove PID file
	os.Remove(s.pidFile)

	s.running = false
	s.pid = 0
	s.cmd = nil

	return nil
}

// Restart restarts the MediaMTX server
func (s *MediaMTXServer) Restart(ctx context.Context) error {
	if err := s.Stop(); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return s.Start(ctx)
}

// IsRunning checks if the server is running
func (s *MediaMTXServer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return false
	}

	// Verify process is actually running
	if s.pid > 0 {
		process, err := os.FindProcess(s.pid)
		if err != nil {
			s.running = false
			return false
		}
		if err := process.Signal(syscall.Signal(0)); err != nil {
			s.running = false
			return false
		}
	}

	return s.running
}

// HealthCheck performs a health check on the MediaMTX API
func (s *MediaMTXServer) HealthCheck() error {
	url := fmt.Sprintf("http://localhost:%d/v3/config/global/get", s.serverCfg.APIPort)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// GetPID returns the server process ID
func (s *MediaMTXServer) GetPID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pid
}

// PathInfo represents information about a MediaMTX path
type PathInfo struct {
	Name          string `json:"name"`
	Ready         bool   `json:"ready"`
	ReadyTime     string `json:"readyTime"`
	BytesReceived int64  `json:"bytesReceived"`
	BytesSent     int64  `json:"bytesSent"`
}

// GetPathInfo retrieves information about a specific path
func (s *MediaMTXServer) GetPathInfo(path string) (*PathInfo, error) {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	url := fmt.Sprintf("http://localhost:%d/v3/paths/get/%s", s.serverCfg.APIPort, path)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get path info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("path not found: %s", path)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var info PathInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &info, nil
}

// ListPaths lists all active paths
func (s *MediaMTXServer) ListPaths() ([]PathInfo, error) {
	url := fmt.Sprintf("http://localhost:%d/v3/paths/list", s.serverCfg.APIPort)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list paths: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Items []PathInfo `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Items, nil
}

// getConfigPath returns the MediaMTX config file path
func (s *MediaMTXServer) getConfigPath() string {
	if s.config.ConfigPath != "" {
		return s.config.ConfigPath
	}
	return filepath.Join(s.dataDir, "mediamtx.yml")
}

// ensureConfig ensures MediaMTX config file exists
func (s *MediaMTXServer) ensureConfig(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		return nil // Config already exists
	}

	// Create minimal config
	config := fmt.Sprintf(`# MediaMTX configuration for youtube-rtsp-proxy
api: yes
apiAddress: :%d
rtspAddress: :%d
logLevel: %s

paths:
  all:
    # Allow any path
`, s.serverCfg.APIPort, s.serverCfg.RTSPPort, s.config.LogLevel)

	return os.WriteFile(configPath, []byte(config), 0644)
}

// waitForReady waits for the server to be ready
func (s *MediaMTXServer) waitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if err := s.HealthCheck(); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for server to be ready")
}

// isAlreadyRunning checks if MediaMTX is already running
func (s *MediaMTXServer) isAlreadyRunning() bool {
	// Check PID file
	if data, err := os.ReadFile(s.pidFile); err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err == nil && pid > 0 {
			if process, err := os.FindProcess(pid); err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
					s.pid = pid
					// Verify it's actually MediaMTX by checking API
					if s.HealthCheck() == nil {
						return true
					}
				}
			}
		}
	}

	// Check if MediaMTX API is responding (might be started externally)
	if s.HealthCheck() == nil {
		return true
	}

	return false
}

// CheckBinary verifies that mediamtx binary exists and is executable
func (s *MediaMTXServer) CheckBinary() error {
	cmd := exec.Command(s.config.BinaryPath, "--help")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mediamtx not found or not executable: %w", err)
	}
	return nil
}
