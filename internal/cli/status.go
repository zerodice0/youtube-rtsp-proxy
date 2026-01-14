package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [stream-name]",
	Short: "Show status of a stream or the proxy server",
	Long: `Show detailed status information.

Without arguments, shows server status.
With a stream name, shows detailed stream status.

Examples:
  youtube-rtsp-proxy status
  youtube-rtsp-proxy status lofi`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return showStreamStatus(args[0])
	}
	return showServerStatus()
}

func showServerStatus() error {
	fmt.Println()
	fmt.Println("RTSP Proxy Server Status")
	fmt.Println("══════════════════════════════════════════════════════════════")

	// MediaMTX status
	if srv.IsRunning() {
		pid := srv.GetPID()
		fmt.Printf("  MediaMTX:    ● Running (PID: %d)\n", pid)
		fmt.Printf("  RTSP Port:   %d\n", cfg.Server.RTSPPort)
		fmt.Printf("  API Port:    %d\n", cfg.Server.APIPort)

		// Health check
		if err := srv.HealthCheck(); err == nil {
			fmt.Printf("  Health:      ● Healthy\n")
		} else {
			fmt.Printf("  Health:      ○ Unhealthy (%v)\n", err)
		}
	} else {
		fmt.Printf("  MediaMTX:    ○ Not running\n")
		fmt.Println()
		fmt.Println("  Start with: youtube-rtsp-proxy server start")
	}

	fmt.Println()

	// Monitor status
	if mon.IsRunning() {
		fmt.Printf("  Monitor:     ● Running\n")
		fmt.Printf("  Check Interval: %v\n", cfg.Monitor.HealthCheckInterval)
		fmt.Printf("  URL Refresh:    %v\n", cfg.Monitor.URLRefreshInterval)
	} else {
		fmt.Printf("  Monitor:     ○ Not running\n")
	}

	fmt.Println()

	// Active streams count
	streams := manager.List()
	runningCount := 0
	for _, s := range streams {
		if s.StateString == "running" {
			runningCount++
		}
	}
	fmt.Printf("  Active Streams: %d\n", runningCount)

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════")

	return nil
}

func showStreamStatus(name string) error {
	info, err := manager.Status(name)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Stream Status: %s\n", name)
	fmt.Println("══════════════════════════════════════════════════════════════")

	// Status with icon
	var statusIcon string
	switch info.StateString {
	case "running":
		statusIcon = "●" // Green
	case "reconnecting":
		statusIcon = "◐" // Yellow
	case "error":
		statusIcon = "○" // Red
	default:
		statusIcon = "○" // Gray
	}

	fmt.Printf("  Status:       %s %s\n", statusIcon, info.StateString)
	fmt.Printf("  Stream ID:    %s\n", info.ID)
	fmt.Printf("  FFmpeg PID:   %d\n", info.FFmpegPID)

	fmt.Println()
	fmt.Println("URLs:")
	localIP := getLocalIP()
	fmt.Printf("  RTSP Local:   rtsp://localhost:%d%s\n", info.Port, info.RTSPPath)
	if localIP != "" {
		fmt.Printf("  RTSP Network: rtsp://%s:%d%s\n", localIP, info.Port, info.RTSPPath)
	}
	fmt.Printf("  YouTube:      %s\n", info.YouTubeURL)

	fmt.Println()
	fmt.Println("Timing:")
	fmt.Printf("  Created:      %s\n", info.CreatedAt.Format(time.RFC3339))
	if !info.StartedAt.IsZero() {
		fmt.Printf("  Started:      %s\n", info.StartedAt.Format(time.RFC3339))
		uptime := time.Since(info.StartedAt).Round(time.Second)
		fmt.Printf("  Uptime:       %s\n", formatDuration(uptime))
	}
	if !info.LastURLRefresh.IsZero() {
		fmt.Printf("  URL Refresh:  %s ago\n", formatDuration(time.Since(info.LastURLRefresh).Round(time.Second)))
	}
	if !info.LastChecked.IsZero() {
		fmt.Printf("  Last Check:   %s ago\n", formatDuration(time.Since(info.LastChecked).Round(time.Second)))
	}

	if info.ErrorCount > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		fmt.Printf("  Total:        %d\n", info.ErrorCount)
		fmt.Printf("  Consecutive:  %d\n", info.ConsecutiveErrors)
		if info.LastError != "" {
			fmt.Printf("  Last Error:   %s\n", info.LastError)
		}
	}

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════")

	// MediaMTX path info
	if pathInfo, err := srv.GetPathInfo(info.RTSPPath); err == nil {
		fmt.Println()
		fmt.Println("MediaMTX Path Info:")
		fmt.Printf("  Ready:          %v\n", pathInfo.Ready)
		fmt.Printf("  Bytes Received: %d\n", pathInfo.BytesReceived)
		fmt.Printf("  Bytes Sent:     %d\n", pathInfo.BytesSent)
		fmt.Println()
		fmt.Println("══════════════════════════════════════════════════════════════")
	}

	return nil
}
