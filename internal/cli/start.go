package cli

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
)

var (
	streamName string
	streamPort int
)

var startCmd = &cobra.Command{
	Use:   "start <youtube-url>",
	Short: "Start proxying a YouTube stream",
	Long: `Start proxying a YouTube stream to RTSP.

Examples:
  youtube-rtsp-proxy start "https://www.youtube.com/watch?v=jfKfPfyJRdk" --name lofi
  youtube-rtsp-proxy start "https://www.youtube.com/live/xyz" --name news --port 8555`,
	Args: cobra.ExactArgs(1),
	RunE: runStart,
}

func init() {
	startCmd.Flags().StringVarP(&streamName, "name", "n", "stream", "stream name (used in RTSP path)")
	startCmd.Flags().IntVarP(&streamPort, "port", "p", 0, "RTSP port (default: from config)")
}

func runStart(cmd *cobra.Command, args []string) error {
	youtubeURL := args[0]

	// Check dependencies first
	if err := checkDependencies(); err != nil {
		return fmt.Errorf("dependency check failed:\n  %v", err)
	}

	// Ensure MediaMTX server is running
	if !srv.IsRunning() {
		fmt.Println("Starting MediaMTX server...")
		if err := srv.Start(getContext()); err != nil {
			return fmt.Errorf("failed to start MediaMTX: %w", err)
		}
	}

	// Start monitoring if not already running
	if !mon.IsRunning() {
		mon.Start(getContext())
	}

	// Use default port if not specified
	port := streamPort
	if port == 0 {
		port = cfg.Server.RTSPPort
	}

	fmt.Printf("Extracting stream URL from YouTube...\n")
	printVerbose("  URL: %s\n", youtubeURL)

	// Start the stream
	ctx := getContext()
	if err := manager.Start(ctx, youtubeURL, streamName, port); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	// Get local IP for network access URL
	localIP := getLocalIP()

	fmt.Println()
	fmt.Println("Stream started successfully!")
	fmt.Println()
	fmt.Printf("RTSP URLs:\n")
	fmt.Printf("  Local:   rtsp://localhost:%d/%s\n", port, streamName)
	if localIP != "" {
		fmt.Printf("  Network: rtsp://%s:%d/%s\n", localIP, port, streamName)
	}
	fmt.Println()
	fmt.Println("Test with:")
	fmt.Printf("  ffplay rtsp://localhost:%d/%s\n", port, streamName)
	fmt.Printf("  vlc rtsp://localhost:%d/%s\n", port, streamName)

	return nil
}

// getLocalIP returns the local IP address
func getLocalIP() string {
	// Try to get default route IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	// Fallback: iterate interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return ""
}
