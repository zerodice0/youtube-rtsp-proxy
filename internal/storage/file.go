package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StreamData represents persisted stream information
type StreamData struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	YouTubeURL     string    `json:"youtube_url"`
	RTSPPath       string    `json:"rtsp_path"`
	Port           int       `json:"port"`
	FFmpegPID      int       `json:"ffmpeg_pid"`
	CreatedAt      time.Time `json:"created_at"`
	StartedAt      time.Time `json:"started_at"`
	LastURLRefresh time.Time `json:"last_url_refresh"`
}

// Storage defines the interface for stream state persistence
type Storage interface {
	Save(data *StreamData) error
	Load(name string) (*StreamData, error)
	Delete(name string) error
	List() ([]*StreamData, error)
	GetDataDir() string
}

// FileStorage implements file-based stream state storage
type FileStorage struct {
	mu      sync.RWMutex
	dataDir string
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(dataDir string) (*FileStorage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &FileStorage{
		dataDir: dataDir,
	}, nil
}

// Save persists stream data to file
func (s *FileStorage) Save(data *StreamData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Save info file (JSON)
	infoPath := filepath.Join(s.dataDir, data.Name+".json")
	infoData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stream data: %w", err)
	}

	if err := os.WriteFile(infoPath, infoData, 0644); err != nil {
		return fmt.Errorf("failed to write info file: %w", err)
	}

	// Save PID file separately for quick access
	if data.FFmpegPID > 0 {
		pidPath := filepath.Join(s.dataDir, data.Name+".pid")
		if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", data.FFmpegPID)), 0644); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
	}

	return nil
}

// Load retrieves stream data from file
func (s *FileStorage) Load(name string) (*StreamData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	infoPath := filepath.Join(s.dataDir, name+".json")
	infoData, err := os.ReadFile(infoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("stream not found: %s", name)
		}
		return nil, fmt.Errorf("failed to read info file: %w", err)
	}

	var data StreamData
	if err := json.Unmarshal(infoData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream data: %w", err)
	}

	return &data, nil
}

// Delete removes stream data files
func (s *FileStorage) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove info file
	infoPath := filepath.Join(s.dataDir, name+".json")
	if err := os.Remove(infoPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove info file: %w", err)
	}

	// Remove PID file
	pidPath := filepath.Join(s.dataDir, name+".pid")
	os.Remove(pidPath) // Ignore errors

	// Remove log file
	logPath := filepath.Join(s.dataDir, name+".log")
	os.Remove(logPath) // Ignore errors

	return nil
}

// List returns all stored stream data
func (s *FileStorage) List() ([]*StreamData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pattern := filepath.Join(s.dataDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list stream files: %w", err)
	}

	var streams []*StreamData
	for _, match := range matches {
		// Skip mediamtx config if stored as json
		if filepath.Base(match) == "mediamtx.json" {
			continue
		}

		data, err := os.ReadFile(match)
		if err != nil {
			continue
		}

		var stream StreamData
		if err := json.Unmarshal(data, &stream); err != nil {
			continue
		}

		streams = append(streams, &stream)
	}

	return streams, nil
}

// GetDataDir returns the data directory path
func (s *FileStorage) GetDataDir() string {
	return s.dataDir
}

// GetPID retrieves just the PID for a stream
func (s *FileStorage) GetPID(name string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pidPath := filepath.Join(s.dataDir, name+".pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, err
	}

	return pid, nil
}

// UpdatePID updates just the PID for a stream
func (s *FileStorage) UpdatePID(name string, pid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update PID file
	pidPath := filepath.Join(s.dataDir, name+".pid")
	if pid > 0 {
		if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
	} else {
		os.Remove(pidPath)
	}

	// Also update JSON file
	infoPath := filepath.Join(s.dataDir, name+".json")
	infoData, err := os.ReadFile(infoPath)
	if err != nil {
		return nil // JSON file might not exist yet
	}

	var data StreamData
	if err := json.Unmarshal(infoData, &data); err != nil {
		return nil
	}

	data.FFmpegPID = pid
	newData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil
	}

	return os.WriteFile(infoPath, newData, 0644)
}

// GetLogPath returns the log file path for a stream
func (s *FileStorage) GetLogPath(name string) string {
	return filepath.Join(s.dataDir, name+".log")
}

// Cleanup removes orphaned files (streams that are no longer running)
func (s *FileStorage) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	streams, err := s.List()
	if err != nil {
		return err
	}

	for _, stream := range streams {
		if stream.FFmpegPID > 0 {
			// Check if process is still running
			if process, err := os.FindProcess(stream.FFmpegPID); err == nil {
				if err := process.Signal(os.Signal(nil)); err != nil {
					// Process is not running, clean up
					s.Delete(stream.Name)
				}
			} else {
				s.Delete(stream.Name)
			}
		}
	}

	return nil
}
