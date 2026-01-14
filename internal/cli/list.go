package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all active streams",
	Long:    `List all active RTSP proxy streams with their status and URLs.`,
	RunE:    runList,
}

func runList(cmd *cobra.Command, args []string) error {
	streams := manager.List()

	fmt.Println()
	fmt.Println("Active RTSP Proxy Streams")
	fmt.Println("══════════════════════════════════════════════════════════════")

	if len(streams) == 0 {
		fmt.Println()
		fmt.Println("  No active streams")
		fmt.Println()
		fmt.Println("  Start one with:")
		fmt.Println("    youtube-rtsp-proxy start <youtube-url> --name <name>")
		fmt.Println()
		fmt.Println("══════════════════════════════════════════════════════════════")
		return nil
	}

	localIP := getLocalIP()

	for _, s := range streams {
		fmt.Println()
		fmt.Printf("Stream: %s\n", s.Name)

		// Status with icon
		var statusIcon string
		switch s.StateString {
		case "running":
			statusIcon = "●" // Green circle
		case "reconnecting":
			statusIcon = "◐" // Half circle
		case "error":
			statusIcon = "○" // Empty circle
		default:
			statusIcon = "○"
		}
		fmt.Printf("  Status:    %s %s (PID: %d)\n", statusIcon, s.StateString, s.FFmpegPID)

		// RTSP URLs
		fmt.Printf("  RTSP URL:  rtsp://localhost:%d%s\n", s.Port, s.RTSPPath)
		if localIP != "" {
			fmt.Printf("  Network:   rtsp://%s:%d%s\n", localIP, s.Port, s.RTSPPath)
		}

		// Source
		fmt.Printf("  Source:    %s\n", truncateURL(s.YouTubeURL, 60))

		// Timing info
		if !s.StartedAt.IsZero() {
			uptime := time.Since(s.StartedAt).Round(time.Second)
			fmt.Printf("  Uptime:    %s\n", formatDuration(uptime))
		}

		// Error info if any
		if s.ErrorCount > 0 {
			fmt.Printf("  Errors:    %d total, %d consecutive\n", s.ErrorCount, s.ConsecutiveErrors)
			if s.LastError != "" {
				fmt.Printf("  Last Error: %s\n", s.LastError)
			}
		}
	}

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════")

	return nil
}

// truncateURL truncates a URL to maxLen characters
func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
