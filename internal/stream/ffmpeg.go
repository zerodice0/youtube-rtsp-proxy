package stream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zerodice0/youtube-rtsp-proxy/internal/config"
)

// FFmpegProcess manages an FFmpeg process for a stream
type FFmpegProcess struct {
	mu sync.Mutex

	cmd       *exec.Cmd
	pid       int
	inputURL  string
	outputURL string
	startTime time.Time
	stderr    *bytes.Buffer
	cancel    context.CancelFunc
	done      chan struct{}
}

// FFmpegManager handles FFmpeg process lifecycle
type FFmpegManager struct {
	config *config.FFmpegConfig
}

// NewFFmpegManager creates a new FFmpeg manager
func NewFFmpegManager(cfg *config.FFmpegConfig) *FFmpegManager {
	return &FFmpegManager{
		config: cfg,
	}
}

// Start starts an FFmpeg process for streaming
func (m *FFmpegManager) Start(ctx context.Context, stream *Stream) (*FFmpegProcess, error) {
	streamURL := stream.GetStreamURL()
	if streamURL == "" {
		return nil, fmt.Errorf("stream URL is empty")
	}

	rtspOutput := fmt.Sprintf("rtsp://localhost:%d%s", stream.Port, stream.RTSPPath)

	// Build FFmpeg arguments
	args := m.buildArgs(streamURL, rtspOutput)

	// Create cancellable context
	procCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(procCtx, m.config.BinaryPath, args...)

	// Capture stderr for error analysis
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	cmd.Stdout = io.Discard

	// Ensure process gets its own process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	proc := &FFmpegProcess{
		cmd:       cmd,
		inputURL:  streamURL,
		outputURL: rtspOutput,
		stderr:    stderr,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	proc.pid = cmd.Process.Pid
	proc.startTime = time.Now()

	// Update stream with FFmpeg info
	stream.SetFFmpegPID(proc.pid)
	stream.FFmpegCmd = cmd

	// Start goroutine to wait for process exit
	go func() {
		cmd.Wait()
		close(proc.done)
	}()

	return proc, nil
}

// buildArgs constructs FFmpeg command line arguments
func (m *FFmpegManager) buildArgs(inputURL, outputURL string) []string {
	args := []string{
		"-re", // Read input at native frame rate
	}

	// Add input options (reconnect settings, etc.)
	args = append(args, m.config.InputOptions...)

	// Input URL
	args = append(args, "-i", inputURL)

	// Output options (codec settings)
	args = append(args, m.config.OutputOptions...)

	// RTSP transport
	args = append(args, "-rtsp_transport", "tcp")

	// Output URL
	args = append(args, outputURL)

	return args
}

// Stop stops the FFmpeg process
func (p *FFmpegProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Cancel the context first
	if p.cancel != nil {
		p.cancel()
	}

	// Try graceful shutdown with SIGTERM
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		if !strings.Contains(err.Error(), "process already finished") {
			// Force kill
			p.cmd.Process.Kill()
		}
	}

	// Wait for process to exit with timeout
	select {
	case <-p.done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Force kill after timeout
		p.cmd.Process.Kill()
		<-p.done
	}

	return nil
}

// IsRunning checks if the FFmpeg process is still running
func (p *FFmpegProcess) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(p.pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetPID returns the process ID
func (p *FFmpegProcess) GetPID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pid
}

// GetStderr returns captured stderr output
func (p *FFmpegProcess) GetStderr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stderr.String()
}

// GetStartTime returns when the process was started
func (p *FFmpegProcess) GetStartTime() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.startTime
}

// Done returns a channel that's closed when the process exits
func (p *FFmpegProcess) Done() <-chan struct{} {
	return p.done
}

// CheckBinary verifies that ffmpeg binary exists and is executable
func (m *FFmpegManager) CheckBinary() error {
	cmd := exec.Command(m.config.BinaryPath, "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg not found or not executable: %w", err)
	}
	return nil
}

// KillByPID kills an FFmpeg process by PID
func KillByPID(pid int) error {
	if pid <= 0 {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return nil // Process doesn't exist
	}

	// Try SIGTERM first
	if err := process.Signal(syscall.SIGTERM); err != nil {
		if !strings.Contains(err.Error(), "process already finished") {
			// Force kill
			process.Kill()
		}
	}

	// Wait a bit for graceful shutdown
	time.Sleep(500 * time.Millisecond)

	// Check if still alive and force kill
	if err := process.Signal(syscall.Signal(0)); err == nil {
		process.Kill()
	}

	return nil
}

// IsProcessAlive checks if a process with given PID is alive
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}
