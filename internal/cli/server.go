package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var foreground bool

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
