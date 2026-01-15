package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zerodice0/youtube-rtsp-proxy/internal/config"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/extractor"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/logger"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/server"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/storage"
)

// Manager manages all streams
type Manager struct {
	mu sync.RWMutex

	streams   map[string]*Stream
	processes map[string]*FFmpegProcess

	config        *config.Config
	extractor     extractor.Extractor
	ffmpeg        *FFmpegManager
	server        *server.MediaMTXServer
	storage       *storage.FileStorage
	loggerManager *logger.LoggerManager
}

// NewManager creates a new stream manager
func NewManager(
	cfg *config.Config,
	ext extractor.Extractor,
	srv *server.MediaMTXServer,
	store *storage.FileStorage,
) *Manager {
	return &Manager{
		streams:       make(map[string]*Stream),
		processes:     make(map[string]*FFmpegProcess),
		config:        cfg,
		extractor:     ext,
		ffmpeg:        NewFFmpegManager(&cfg.FFmpeg),
		server:        srv,
		storage:       store,
		loggerManager: logger.NewLoggerManager(store.GetDataDir(), 100),
	}
}

// Start starts a new stream
func (m *Manager) Start(ctx context.Context, youtubeURL, name string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log := m.loggerManager.GetLogger(name)

	// Check if stream already exists
	if _, exists := m.streams[name]; exists {
		return fmt.Errorf("stream '%s' already exists", name)
	}

	// Use default port if not specified
	if port == 0 {
		port = m.config.Server.RTSPPort
	}

	// Create new stream
	stream := NewStream(name, youtubeURL, port)
	stream.SetState(StateStarting)
	log.Info("Starting stream from %s", youtubeURL)

	// Extract stream URL
	info, err := m.extractor.Extract(ctx, youtubeURL)
	if err != nil {
		log.Error("Failed to extract stream URL: %v", err)
		return fmt.Errorf("failed to extract stream URL: %w", err)
	}
	stream.SetStreamURL(info.URL)
	log.Info("Extracted stream URL successfully")

	// Start FFmpeg process
	proc, err := m.ffmpeg.Start(ctx, stream)
	if err != nil {
		log.Error("Failed to start FFmpeg: %v", err)
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Wait a bit for FFmpeg to initialize
	time.Sleep(2 * time.Second)

	// Verify process is running
	if !proc.IsRunning() {
		stderr := proc.GetStderr()
		log.Error("FFmpeg exited prematurely: %s", stderr)
		return fmt.Errorf("ffmpeg exited prematurely: %s", stderr)
	}

	stream.SetState(StateRunning)
	stream.SetStartedAt(time.Now())
	log.Info("Stream started successfully (PID: %d, RTSP: %s)", proc.GetPID(), stream.RTSPPath)

	// Store stream and process
	m.streams[name] = stream
	m.processes[name] = proc

	// Persist to storage
	m.saveStream(stream)

	return nil
}

// Stop stops a stream
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopStream(name)
}

// stopStream stops a stream (internal, must be called with lock held)
func (m *Manager) stopStream(name string) error {
	log := m.loggerManager.GetLogger(name)
	stream, exists := m.streams[name]
	if !exists {
		// Try to load from storage and kill by PID
		if data, err := m.storage.Load(name); err == nil && data.FFmpegPID > 0 {
			log.Info("Stopping orphaned stream (PID: %d)", data.FFmpegPID)
			KillByPID(data.FFmpegPID)
			m.storage.Delete(name)
			return nil
		}
		return fmt.Errorf("stream '%s' not found", name)
	}

	log.Info("Stopping stream")
	stream.SetState(StateStopping)

	// Stop FFmpeg process
	if proc, exists := m.processes[name]; exists {
		proc.Stop()
		delete(m.processes, name)
	}

	// Kill by PID if process reference is lost
	if pid := stream.GetFFmpegPID(); pid > 0 {
		KillByPID(pid)
	}

	// Clean up
	delete(m.streams, name)
	m.storage.Delete(name)
	log.Info("Stream stopped")

	return nil
}

// StopAll stops all streams
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name := range m.streams {
		if err := m.stopStream(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// List returns information about all streams
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []Info
	for _, stream := range m.streams {
		infos = append(infos, stream.GetInfo())
	}

	// Also check storage for streams from previous sessions
	stored, err := m.storage.List()
	if err == nil {
		for _, data := range stored {
			// Skip if already in memory
			if _, exists := m.streams[data.Name]; exists {
				continue
			}

			// Check if process is still running
			if data.FFmpegPID > 0 && IsProcessAlive(data.FFmpegPID) {
				infos = append(infos, Info{
					ID:             data.ID,
					Name:           data.Name,
					YouTubeURL:     data.YouTubeURL,
					RTSPPath:       data.RTSPPath,
					Port:           data.Port,
					State:          StateRunning,
					StateString:    "running",
					FFmpegPID:      data.FFmpegPID,
					CreatedAt:      data.CreatedAt,
					StartedAt:      data.StartedAt,
					LastURLRefresh: data.LastURLRefresh,
				})
			}
		}
	}

	return infos
}

// Status returns information about a specific stream
func (m *Manager) Status(name string) (*Info, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if stream, exists := m.streams[name]; exists {
		info := stream.GetInfo()
		return &info, nil
	}

	// Try storage
	data, err := m.storage.Load(name)
	if err != nil {
		return nil, fmt.Errorf("stream '%s' not found", name)
	}

	state := StateError
	stateStr := "error"
	if data.FFmpegPID > 0 && IsProcessAlive(data.FFmpegPID) {
		state = StateRunning
		stateStr = "running"
	}

	return &Info{
		ID:             data.ID,
		Name:           data.Name,
		YouTubeURL:     data.YouTubeURL,
		RTSPPath:       data.RTSPPath,
		Port:           data.Port,
		State:          state,
		StateString:    stateStr,
		FFmpegPID:      data.FFmpegPID,
		CreatedAt:      data.CreatedAt,
		StartedAt:      data.StartedAt,
		LastURLRefresh: data.LastURLRefresh,
	}, nil
}

// GetStream returns a stream by name (for monitor access)
func (m *Manager) GetStream(name string) *Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.streams[name]
}

// GetProcess returns an FFmpeg process by stream name
func (m *Manager) GetProcess(name string) *FFmpegProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.processes[name]
}

// RestartStream restarts a stream (for reconnection)
func (m *Manager) RestartStream(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log := m.loggerManager.GetLogger(name)
	stream, exists := m.streams[name]
	if !exists {
		return fmt.Errorf("stream '%s' not found", name)
	}

	log.Warn("Restarting stream")
	youtubeURL := stream.YouTubeURL
	port := stream.Port

	// Stop existing stream
	m.stopStream(name)

	// Release lock temporarily for start
	m.mu.Unlock()
	err := m.Start(ctx, youtubeURL, name, port)
	m.mu.Lock()

	if err != nil {
		log.Error("Restart failed: %v", err)
	}
	return err
}

// RefreshURL extracts a new stream URL for a stream
func (m *Manager) RefreshURL(ctx context.Context, name string) error {
	m.mu.Lock()
	log := m.loggerManager.GetLogger(name)
	stream, exists := m.streams[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("stream '%s' not found", name)
	}

	log.Info("Refreshing stream URL")
	stream.SetState(StateReconnecting)
	youtubeURL := stream.YouTubeURL
	m.mu.Unlock()

	// Extract new URL
	info, err := m.extractor.Extract(ctx, youtubeURL)
	if err != nil {
		log.Error("Failed to refresh URL: %v", err)
		return fmt.Errorf("failed to extract new URL: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stream.SetStreamURL(info.URL)
	log.Info("URL refreshed successfully")
	return nil
}

// RecoverStreams attempts to recover streams from storage
func (m *Manager) RecoverStreams() {
	m.mu.Lock()
	defer m.mu.Unlock()

	stored, err := m.storage.List()
	if err != nil {
		return
	}

	for _, data := range stored {
		// Skip if already in memory
		if _, exists := m.streams[data.Name]; exists {
			continue
		}

		// Check if process is still running
		if data.FFmpegPID > 0 && IsProcessAlive(data.FFmpegPID) {
			stream := &Stream{
				ID:             data.ID,
				Name:           data.Name,
				YouTubeURL:     data.YouTubeURL,
				RTSPPath:       data.RTSPPath,
				Port:           data.Port,
				State:          StateRunning,
				FFmpegPID:      data.FFmpegPID,
				CreatedAt:      data.CreatedAt,
				StartedAt:      data.StartedAt,
				LastURLRefresh: data.LastURLRefresh,
			}
			m.streams[data.Name] = stream
		} else {
			// Clean up orphaned storage entry
			m.storage.Delete(data.Name)
		}
	}
}

// saveStream persists stream data to storage
func (m *Manager) saveStream(stream *Stream) {
	data := &storage.StreamData{
		ID:             stream.ID,
		Name:           stream.Name,
		YouTubeURL:     stream.YouTubeURL,
		RTSPPath:       stream.RTSPPath,
		Port:           stream.Port,
		FFmpegPID:      stream.GetFFmpegPID(),
		CreatedAt:      stream.CreatedAt,
		StartedAt:      stream.StartedAt,
		LastURLRefresh: stream.GetLastURLRefresh(),
	}
	m.storage.Save(data)
}

// UpdateStreamPID updates the PID in storage
func (m *Manager) UpdateStreamPID(name string, pid int) {
	m.storage.UpdatePID(name, pid)
}

// GetAllStreams returns all stream objects (for monitor access)
func (m *Manager) GetAllStreams() []*Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*Stream, 0, len(m.streams))
	for _, s := range m.streams {
		streams = append(streams, s)
	}
	return streams
}

// GetLoggerManager returns the logger manager (for monitor access)
func (m *Manager) GetLoggerManager() *logger.LoggerManager {
	return m.loggerManager
}
