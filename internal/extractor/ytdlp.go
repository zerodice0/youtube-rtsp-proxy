package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StreamInfo contains extracted stream information
type StreamInfo struct {
	URL        string
	Format     string
	Resolution string
	IsLive     bool
	Title      string
}

// Extractor defines the interface for URL extraction
type Extractor interface {
	Extract(ctx context.Context, youtubeURL string) (*StreamInfo, error)
	IsLiveStream(ctx context.Context, youtubeURL string) (bool, error)
}

// YtdlpExtractor implements URL extraction using yt-dlp
type YtdlpExtractor struct {
	BinaryPath string
	Timeout    time.Duration
	Format     string
}

// NewYtdlpExtractor creates a new yt-dlp extractor
func NewYtdlpExtractor(binaryPath string, timeout time.Duration, format string) *YtdlpExtractor {
	if binaryPath == "" {
		binaryPath = "yt-dlp"
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if format == "" {
		format = "best[protocol=https]/best"
	}
	return &YtdlpExtractor{
		BinaryPath: binaryPath,
		Timeout:    timeout,
		Format:     format,
	}
}

// Extract extracts the direct stream URL from a YouTube URL
func (e *YtdlpExtractor) Extract(ctx context.Context, youtubeURL string) (*StreamInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	// Get stream URL
	urlCmd := exec.CommandContext(ctx, e.BinaryPath,
		"-f", e.Format,
		"-g",
		"--no-warnings",
		youtubeURL,
	)

	urlOutput, err := urlCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to extract URL: %w", err)
	}

	streamURL := strings.TrimSpace(string(urlOutput))
	if streamURL == "" {
		return nil, fmt.Errorf("empty stream URL returned")
	}

	// Get video info (title, live status, etc.)
	info, err := e.getVideoInfo(ctx, youtubeURL)
	if err != nil {
		// Return basic info even if metadata fetch fails
		return &StreamInfo{
			URL: streamURL,
		}, nil
	}

	info.URL = streamURL
	return info, nil
}

// getVideoInfo retrieves video metadata
func (e *YtdlpExtractor) getVideoInfo(ctx context.Context, youtubeURL string) (*StreamInfo, error) {
	cmd := exec.CommandContext(ctx, e.BinaryPath,
		"-j",
		"--no-warnings",
		youtubeURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	var data struct {
		Title       string `json:"title"`
		IsLive      bool   `json:"is_live"`
		Format      string `json:"format"`
		Resolution  string `json:"resolution"`
		FormatNote  string `json:"format_note"`
		Height      int    `json:"height"`
		Width       int    `json:"width"`
	}

	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("failed to parse video info: %w", err)
	}

	resolution := data.Resolution
	if resolution == "" && data.Height > 0 {
		resolution = fmt.Sprintf("%dx%d", data.Width, data.Height)
	}

	return &StreamInfo{
		Title:      data.Title,
		IsLive:     data.IsLive,
		Format:     data.Format,
		Resolution: resolution,
	}, nil
}

// IsLiveStream checks if the URL is a live stream
func (e *YtdlpExtractor) IsLiveStream(ctx context.Context, youtubeURL string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.BinaryPath,
		"-j",
		"--no-warnings",
		youtubeURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check live status: %w", err)
	}

	var data struct {
		IsLive bool `json:"is_live"`
	}

	if err := json.Unmarshal(output, &data); err != nil {
		return false, fmt.Errorf("failed to parse live status: %w", err)
	}

	return data.IsLive, nil
}

// CheckBinary verifies that yt-dlp binary exists and is executable
func (e *YtdlpExtractor) CheckBinary() error {
	cmd := exec.Command(e.BinaryPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp not found or not executable: %w", err)
	}
	return nil
}
