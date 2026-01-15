package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var reconnectCmd = &cobra.Command{
	Use:   "reconnect <stream-name>",
	Short: "Force reconnect a stream",
	Long: `Force a stream to reconnect with a fresh URL.

This is useful for testing the reconnection logic or recovering
from a stale stream state.

Example:
  youtube-rtsp-proxy reconnect lofi`,
	Args: cobra.ExactArgs(1),
	RunE: runReconnect,
}

func runReconnect(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Check if stream exists
	s := manager.GetStream(name)
	if s == nil {
		return fmt.Errorf("stream '%s' not found", name)
	}

	fmt.Printf("Forcing reconnection for stream '%s'...\n", name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mon.ForceReconnect(ctx, name); err != nil {
		return fmt.Errorf("failed to trigger reconnection: %w", err)
	}

	fmt.Printf("Reconnection triggered. Check status with: youtube-rtsp-proxy status %s\n", name)
	return nil
}
