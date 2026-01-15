package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/config"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/extractor"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/monitor"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/server"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/storage"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/stream"
)

var (
	cfgFile   string
	verbose   bool
	cfg       *config.Config
	store     *storage.FileStorage
	srv       *server.MediaMTXServer
	ext       extractor.Extractor
	manager   *stream.Manager
	mon       *monitor.Monitor

	// Version info (set by build flags)
	Version   = "dev"
	BuildTime = "unknown"
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "youtube-rtsp-proxy",
	Short: "YouTube to RTSP proxy",
	Long: `YouTube to RTSP Proxy - Stream YouTube videos via RTSP protocol.

This tool extracts YouTube stream URLs using yt-dlp, transcodes them
with FFmpeg, and serves them via MediaMTX RTSP server.

Features:
  - Automatic URL refresh for live streams
  - Health monitoring and auto-reconnection
  - Multiple stream support`,
	PersistentPreRunE: initApp,
	Version:           fmt.Sprintf("%s (built at %s)", Version, BuildTime),
}

// Execute runs the CLI
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(favCmd)
	rootCmd.AddCommand(reconnectCmd)
}

// initApp initializes the application components
func initApp(cmd *cobra.Command, args []string) error {
	// Skip init for help commands
	if cmd.Name() == "help" || cmd.Name() == "version" {
		return nil
	}

	// Load configuration
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize storage
	store, err = storage.NewFileStorage(cfg.Storage.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize extractor
	ext = extractor.NewYtdlpExtractor(
		cfg.Ytdlp.BinaryPath,
		cfg.Ytdlp.Timeout,
		cfg.Ytdlp.Format,
	)

	// Initialize MediaMTX server manager
	srv = server.NewMediaMTXServer(&cfg.MediaMTX, &cfg.Server, cfg.Storage.DataDir)

	// Initialize stream manager
	manager = stream.NewManager(cfg, ext, srv, store)

	// Initialize monitor
	mon = monitor.NewMonitor(&cfg.Monitor, manager, srv, ext)

	// Recover streams from previous session
	manager.RecoverStreams()

	return nil
}

// getContext returns a context that's cancelled on interrupt
func getContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	return ctx
}

// checkDependencies verifies all required binaries exist
func checkDependencies() error {
	// Check yt-dlp
	ytdlp := extractor.NewYtdlpExtractor(cfg.Ytdlp.BinaryPath, 0, "")
	if err := ytdlp.CheckBinary(); err != nil {
		return fmt.Errorf("yt-dlp: %w\n  Install with: pip install yt-dlp", err)
	}

	// Check ffmpeg
	ffmpegMgr := stream.NewFFmpegManager(&cfg.FFmpeg)
	if err := ffmpegMgr.CheckBinary(); err != nil {
		return fmt.Errorf("ffmpeg: %w\n  Install with: apt install ffmpeg", err)
	}

	// Check mediamtx
	if err := srv.CheckBinary(); err != nil {
		return fmt.Errorf("mediamtx: %w\n  Download from: https://github.com/bluenviron/mediamtx/releases", err)
	}

	return nil
}

// printVerbose prints message only in verbose mode
func printVerbose(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format, args...)
	}
}
