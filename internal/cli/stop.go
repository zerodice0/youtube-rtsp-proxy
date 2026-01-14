package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <stream-name|all>",
	Short: "Stop a stream or all streams",
	Long: `Stop a specific stream or all running streams.

Examples:
  youtube-rtsp-proxy stop lofi
  youtube-rtsp-proxy stop all`,
	Args: cobra.ExactArgs(1),
	RunE: runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	target := args[0]

	if target == "all" {
		fmt.Println("Stopping all streams...")
		if err := manager.StopAll(); err != nil {
			return fmt.Errorf("failed to stop streams: %w", err)
		}
		fmt.Println("All streams stopped.")
		return nil
	}

	// Stop specific stream
	fmt.Printf("Stopping stream '%s'...\n", target)
	if err := manager.Stop(target); err != nil {
		return fmt.Errorf("failed to stop stream: %w", err)
	}
	fmt.Printf("Stream '%s' stopped.\n", target)

	return nil
}
