package stream

import (
	"sync"
	"time"
)

// State represents the current state of a stream
type State int

const (
	StateIdle State = iota
	StateStarting
	StateRunning
	StateReconnecting
	StateStopping
	StateError
)

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateReconnecting:
		return "reconnecting"
	case StateStopping:
		return "stopping"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// Stream represents a single YouTube to RTSP proxy stream
type Stream struct {
	mu sync.RWMutex

	ID         string
	Name       string
	YouTubeURL string
	StreamURL  string // Extracted direct stream URL
	RTSPPath   string // RTSP path (e.g., /stream1)
	Port       int

	State         State
	FFmpegPID     int
	FFmpegCmd     interface{} // *exec.Cmd, stored as interface to avoid import cycle
	CreatedAt     time.Time
	StartedAt     time.Time
	LastChecked   time.Time
	LastURLRefresh time.Time

	// Health tracking
	ErrorCount         int
	ConsecutiveErrors  int
	LastError          string
	LastBytesReceived  int64
	StallCount         int
}

// NewStream creates a new stream instance
func NewStream(name, youtubeURL string, port int) *Stream {
	return &Stream{
		ID:         generateID(),
		Name:       name,
		YouTubeURL: youtubeURL,
		RTSPPath:   "/" + name,
		Port:       port,
		State:      StateIdle,
		CreatedAt:  time.Now(),
	}
}

// Info returns a copy of stream information (thread-safe)
type Info struct {
	ID                string
	Name              string
	YouTubeURL        string
	RTSPPath          string
	Port              int
	State             State
	StateString       string
	FFmpegPID         int
	CreatedAt         time.Time
	StartedAt         time.Time
	LastChecked       time.Time
	LastURLRefresh    time.Time
	ErrorCount        int
	ConsecutiveErrors int
	LastError         string
}

// GetInfo returns stream information
func (s *Stream) GetInfo() Info {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Info{
		ID:                s.ID,
		Name:              s.Name,
		YouTubeURL:        s.YouTubeURL,
		RTSPPath:          s.RTSPPath,
		Port:              s.Port,
		State:             s.State,
		StateString:       s.State.String(),
		FFmpegPID:         s.FFmpegPID,
		CreatedAt:         s.CreatedAt,
		StartedAt:         s.StartedAt,
		LastChecked:       s.LastChecked,
		LastURLRefresh:    s.LastURLRefresh,
		ErrorCount:        s.ErrorCount,
		ConsecutiveErrors: s.ConsecutiveErrors,
		LastError:         s.LastError,
	}
}

// SetState updates the stream state
func (s *Stream) SetState(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

// GetState returns the current state
func (s *Stream) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// SetStreamURL updates the stream URL
func (s *Stream) SetStreamURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StreamURL = url
	s.LastURLRefresh = time.Now()
}

// GetStreamURL returns the current stream URL
func (s *Stream) GetStreamURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.StreamURL
}

// SetFFmpegPID updates the FFmpeg process ID
func (s *Stream) SetFFmpegPID(pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FFmpegPID = pid
}

// GetFFmpegPID returns the FFmpeg process ID
func (s *Stream) GetFFmpegPID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.FFmpegPID
}

// SetStartedAt updates the started time
func (s *Stream) SetStartedAt(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StartedAt = t
}

// IncrementErrorCount increments the error count
func (s *Stream) IncrementErrorCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCount++
	s.ConsecutiveErrors++
}

// ResetConsecutiveErrors resets the consecutive error count
func (s *Stream) ResetConsecutiveErrors() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConsecutiveErrors = 0
}

// SetLastError sets the last error message
func (s *Stream) SetLastError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
}

// SetLastChecked updates the last checked time
func (s *Stream) SetLastChecked(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastChecked = t
}

// GetLastURLRefresh returns the last URL refresh time
func (s *Stream) GetLastURLRefresh() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastURLRefresh
}

// GetConsecutiveErrors returns the consecutive error count
func (s *Stream) GetConsecutiveErrors() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConsecutiveErrors
}

// GetLastError returns the last error message
func (s *Stream) GetLastError() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastError
}

// UpdateBytesReceived updates bytes received and returns true if data is flowing
func (s *Stream) UpdateBytesReceived(bytes int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if bytes == s.LastBytesReceived {
		s.StallCount++
		return false
	}

	s.LastBytesReceived = bytes
	s.StallCount = 0
	return true
}

// GetStallCount returns the stall count
func (s *Stream) GetStallCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.StallCount
}

// generateID generates a unique stream ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

// randomString generates a random string of given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
