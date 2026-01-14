package monitor

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/zerodice0/youtube-rtsp-proxy/internal/config"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/extractor"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/server"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/stream"
)

// Monitor handles health checking and automatic reconnection
type Monitor struct {
	mu sync.Mutex

	config        *config.MonitorConfig
	streamManager *stream.Manager
	server        *server.MediaMTXServer
	extractor     extractor.Extractor

	running  bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewMonitor creates a new monitor instance
func NewMonitor(
	cfg *config.MonitorConfig,
	manager *stream.Manager,
	srv *server.MediaMTXServer,
	ext extractor.Extractor,
) *Monitor {
	return &Monitor{
		config:        cfg,
		streamManager: manager,
		server:        srv,
		extractor:     ext,
	}
}

// Start starts the monitoring loop
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.run(monitorCtx)
	}()
}

// Stop stops the monitoring loop
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}

	if m.cancel != nil {
		m.cancel()
	}
	m.running = false
	m.mu.Unlock()

	m.wg.Wait()
}

// IsRunning returns whether the monitor is running
func (m *Monitor) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// run is the main monitoring loop
func (m *Monitor) run(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	log.Printf("[Monitor] Started with health check interval: %v", m.config.HealthCheckInterval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Monitor] Stopping...")
			return
		case <-ticker.C:
			m.runHealthChecks(ctx)
		}
	}
}

// runHealthChecks performs health checks on all streams
func (m *Monitor) runHealthChecks(ctx context.Context) {
	// Check MediaMTX server first
	if err := m.server.HealthCheck(); err != nil {
		log.Printf("[Monitor] MediaMTX server unhealthy: %v", err)
		m.handleServerFailure(ctx)
		return
	}

	// Check each stream
	streams := m.streamManager.GetAllStreams()
	for _, s := range streams {
		if s.GetState() != stream.StateRunning {
			continue
		}

		status := m.checkStreamHealth(s)
		if !status.Healthy {
			log.Printf("[Monitor] Stream '%s' unhealthy: %s", s.Name, status.Reason)
			go m.handleStreamFailure(ctx, s, status.Reason)
		} else {
			s.ResetConsecutiveErrors()
			s.SetLastChecked(time.Now())
		}
	}
}

// HealthStatus represents the health check result
type HealthStatus struct {
	Healthy bool
	Reason  string
}

// checkStreamHealth checks the health of a single stream
func (m *Monitor) checkStreamHealth(s *stream.Stream) HealthStatus {
	// 1. Check if FFmpeg process is alive
	pid := s.GetFFmpegPID()
	if pid <= 0 || !stream.IsProcessAlive(pid) {
		return HealthStatus{Healthy: false, Reason: "ffmpeg process not running"}
	}

	// 2. Check MediaMTX path status
	pathInfo, err := m.server.GetPathInfo(s.RTSPPath)
	if err != nil {
		return HealthStatus{Healthy: false, Reason: "path not found in MediaMTX"}
	}

	// 3. Check if data is flowing
	if !pathInfo.Ready {
		return HealthStatus{Healthy: false, Reason: "path not ready"}
	}

	// 4. Check for stalled stream (bytes not increasing)
	if !s.UpdateBytesReceived(pathInfo.BytesReceived) {
		stallCount := s.GetStallCount()
		if stallCount >= 3 {
			return HealthStatus{Healthy: false, Reason: "stream stalled (no data flow)"}
		}
	}

	return HealthStatus{Healthy: true}
}

// handleServerFailure handles MediaMTX server failure
func (m *Monitor) handleServerFailure(ctx context.Context) {
	log.Printf("[Monitor] Attempting to restart MediaMTX server...")

	if err := m.server.Restart(ctx); err != nil {
		log.Printf("[Monitor] Failed to restart MediaMTX: %v", err)
		return
	}

	log.Printf("[Monitor] MediaMTX restarted, restarting all streams...")

	// Restart all streams
	streams := m.streamManager.GetAllStreams()
	for _, s := range streams {
		go m.restartStream(ctx, s)
	}
}

// handleStreamFailure handles a single stream failure
func (m *Monitor) handleStreamFailure(ctx context.Context, s *stream.Stream, reason string) {
	s.IncrementErrorCount()
	s.SetLastError(reason)
	s.SetState(stream.StateReconnecting)

	// Check if we should refresh URL
	if m.shouldRefreshURL(s, reason) {
		log.Printf("[Monitor] Refreshing URL for stream '%s'", s.Name)
		if err := m.refreshStreamURL(ctx, s); err != nil {
			log.Printf("[Monitor] Failed to refresh URL: %v", err)
		}
	}

	// Attempt reconnection
	m.reconnectStream(ctx, s)
}

// shouldRefreshURL determines if URL should be refreshed
func (m *Monitor) shouldRefreshURL(s *stream.Stream, reason string) bool {
	// Condition 1: Periodic refresh
	if time.Since(s.GetLastURLRefresh()) > m.config.URLRefreshInterval {
		return true
	}

	// Condition 2: Consecutive errors
	if s.GetConsecutiveErrors() >= m.config.MaxConsecutiveErrors {
		return true
	}

	// Condition 3: URL-related error patterns
	if m.hasURLExpiredError(reason) {
		return true
	}

	return false
}

// hasURLExpiredError checks for URL expiration error patterns
func (m *Monitor) hasURLExpiredError(errMsg string) bool {
	patterns := []string{
		"403",
		"404",
		"forbidden",
		"not found",
		"connection refused",
		"timeout",
		"expired",
	}

	errLower := strings.ToLower(errMsg)
	for _, pattern := range patterns {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}
	return false
}

// refreshStreamURL extracts a new URL for the stream
func (m *Monitor) refreshStreamURL(ctx context.Context, s *stream.Stream) error {
	info, err := m.extractor.Extract(ctx, s.YouTubeURL)
	if err != nil {
		return err
	}

	s.SetStreamURL(info.URL)
	return nil
}

// reconnectStream attempts to reconnect a stream with exponential backoff
func (m *Monitor) reconnectStream(ctx context.Context, s *stream.Stream) {
	backoff := m.config.Reconnect.InitialDelay

	for attempt := 1; attempt <= m.config.Reconnect.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("[Monitor] Reconnect attempt %d/%d for stream '%s' (delay: %v)",
			attempt, m.config.Reconnect.MaxAttempts, s.Name, backoff)

		// Stop existing process
		if pid := s.GetFFmpegPID(); pid > 0 {
			stream.KillByPID(pid)
			time.Sleep(500 * time.Millisecond)
		}

		// Restart stream
		if err := m.streamManager.RestartStream(ctx, s.Name); err != nil {
			log.Printf("[Monitor] Reconnect failed: %v", err)

			// Wait before next attempt
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff = m.nextBackoff(backoff)
			continue
		}

		// Success
		log.Printf("[Monitor] Stream '%s' reconnected successfully", s.Name)
		s.ResetConsecutiveErrors()
		s.SetState(stream.StateRunning)
		return
	}

	// Max attempts reached
	log.Printf("[Monitor] Max reconnect attempts reached for stream '%s'", s.Name)
	s.SetState(stream.StateError)
}

// restartStream restarts a stream after server recovery
func (m *Monitor) restartStream(ctx context.Context, s *stream.Stream) {
	log.Printf("[Monitor] Restarting stream '%s' after server recovery", s.Name)

	// Refresh URL first
	if err := m.refreshStreamURL(ctx, s); err != nil {
		log.Printf("[Monitor] Failed to refresh URL for stream '%s': %v", s.Name, err)
	}

	// Restart
	if err := m.streamManager.RestartStream(ctx, s.Name); err != nil {
		log.Printf("[Monitor] Failed to restart stream '%s': %v", s.Name, err)
		m.reconnectStream(ctx, s)
	}
}

// nextBackoff calculates the next backoff duration
func (m *Monitor) nextBackoff(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * m.config.Reconnect.Multiplier)
	if next > m.config.Reconnect.MaxDelay {
		return m.config.Reconnect.MaxDelay
	}
	return next
}

// TriggerHealthCheck manually triggers a health check
func (m *Monitor) TriggerHealthCheck(ctx context.Context) {
	m.runHealthChecks(ctx)
}

// ForceReconnect forces a reconnection for a specific stream
func (m *Monitor) ForceReconnect(ctx context.Context, name string) error {
	s := m.streamManager.GetStream(name)
	if s == nil {
		return nil
	}

	go m.handleStreamFailure(ctx, s, "forced reconnection")
	return nil
}
