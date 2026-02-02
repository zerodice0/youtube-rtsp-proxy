package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/storage"
)

var (
	foreground   bool
	favorites    string
	allFavorites bool
)

var serverCmd = &cobra.Command{
	Use:   "server <start|stop|restart>",
	Short: "Control the MediaMTX server",
	Long: `Control the MediaMTX RTSP server.

Commands:
  start   - Start the MediaMTX server
  stop    - Stop the MediaMTX server
  restart - Restart the MediaMTX server

Examples:
  youtube-rtsp-proxy server start
  youtube-rtsp-proxy server start --foreground
  youtube-rtsp-proxy server stop
  youtube-rtsp-proxy server restart`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the MediaMTX server",
	RunE:  runServerStart,
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the MediaMTX server",
	RunE:  runServerStop,
}

var serverRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the MediaMTX server",
	RunE:  runServerRestart,
}

func init() {
	serverStartCmd.Flags().BoolVarP(&foreground, "foreground", "f", false, "run in foreground (blocking)")
	serverStartCmd.Flags().StringVar(&favorites, "favorites", "", "comma-separated favorite names to start")
	serverStartCmd.Flags().BoolVar(&allFavorites, "all-favorites", false, "start all favorites")

	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverRestartCmd)
}

func runServerStart(cmd *cobra.Command, args []string) error {
	// Check dependencies
	if err := checkDependencies(); err != nil {
		return fmt.Errorf("dependency check failed:\n  %v", err)
	}

	if srv.IsRunning() {
		fmt.Println("MediaMTX server is already running.")
		return nil
	}

	fmt.Println("Starting MediaMTX server...")
	ctx := getContext()

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MediaMTX: %w", err)
	}

	fmt.Printf("MediaMTX server started (PID: %d)\n", srv.GetPID())
	fmt.Printf("  RTSP: rtsp://localhost:%d\n", cfg.Server.RTSPPort)
	fmt.Printf("  API:  http://localhost:%d\n", cfg.Server.APIPort)

	if foreground {
		fmt.Println()
		fmt.Println("Running in foreground. Press Ctrl+C to stop.")

		// Start monitor
		mon.Start(ctx)

		// Recover any existing streams
		manager.RecoverStreams()

		// Start favorites if specified
		if allFavorites || favorites != "" {
			if err := startFavorites(ctx); err != nil {
				fmt.Printf("Warning: failed to start some favorites: %v\n", err)
			}
		}

		// Wait for interrupt
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		fmt.Println()
		fmt.Println("Shutting down...")

		// Stop monitor
		mon.Stop()

		// Stop all streams
		manager.StopAll()

		// Stop server
		srv.Stop()

		fmt.Println("Shutdown complete.")
	}

	return nil
}

func runServerStop(cmd *cobra.Command, args []string) error {
	if !srv.IsRunning() {
		fmt.Println("MediaMTX server is not running.")
		return nil
	}

	fmt.Println("Stopping all streams...")
	manager.StopAll()

	fmt.Println("Stopping MediaMTX server...")
	if err := srv.Stop(); err != nil {
		return fmt.Errorf("failed to stop MediaMTX: %w", err)
	}

	fmt.Println("MediaMTX server stopped.")
	return nil
}

func runServerRestart(cmd *cobra.Command, args []string) error {
	fmt.Println("Restarting MediaMTX server...")

	ctx := getContext()
	if err := srv.Restart(ctx); err != nil {
		return fmt.Errorf("failed to restart MediaMTX: %w", err)
	}

	fmt.Printf("MediaMTX server restarted (PID: %d)\n", srv.GetPID())
	return nil
}

// startFavorites starts streams for specified favorites
func startFavorites(ctx context.Context) error {
	favStore, err := storage.NewFavoritesStorage(cfg.Storage.DataDir)
	if err != nil {
		return err
	}

	var names []string
	if allFavorites {
		favList, err := favStore.List()
		if err != nil {
			return fmt.Errorf("failed to list favorites: %w", err)
		}
		for _, f := range favList {
			names = append(names, f.Name)
		}
	} else {
		names = strings.Split(favorites, ",")
	}

	if len(names) == 0 {
		fmt.Println("No favorites to start.")
		return nil
	}

	fmt.Printf("Starting %d favorite(s)...\n", len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		fav, err := favStore.Get(name)
		if err != nil {
			fmt.Printf("  Warning: favorite '%s' not found\n", name)
			continue
		}

		fmt.Printf("  Starting '%s'...\n", name)
		if err := manager.Start(ctx, fav.URL, name, cfg.Server.RTSPPort); err != nil {
			fmt.Printf("    Failed: %v\n", err)
		} else {
			fmt.Printf("    Started: rtsp://localhost:%d/%s\n", cfg.Server.RTSPPort, name)
		}
	}

	return nil
}
