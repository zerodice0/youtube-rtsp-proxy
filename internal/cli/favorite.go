package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zerodice0/youtube-rtsp-proxy/internal/storage"
)

var favStore *storage.FavoritesStorage

var favCmd = &cobra.Command{
	Use:     "fav",
	Aliases: []string{"favorite"},
	Short:   "Manage favorite YouTube URLs",
	Long: `Manage your favorite YouTube URLs for quick access.

Examples:
  youtube-rtsp-proxy fav                                                    # Interactive mode
  youtube-rtsp-proxy fav add "https://www.youtube.com/watch?v=jfKfPfyJRdk" --name lofi
  youtube-rtsp-proxy fav list
  youtube-rtsp-proxy fav start lofi
  youtube-rtsp-proxy fav remove lofi`,
	RunE: runFavInteractive,
}

var favAddCmd = &cobra.Command{
	Use:   "add <youtube-url>",
	Short: "Add a YouTube URL to favorites",
	Args:  cobra.ExactArgs(1),
	RunE:  runFavAdd,
}

var favListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all favorite URLs",
	RunE:    runFavList,
}

var favRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a favorite",
	Args:    cobra.ExactArgs(1),
	RunE:    runFavRemove,
}

var favStartCmd = &cobra.Command{
	Use:   "start <name>",
	Short: "Start streaming from a favorite",
	Args:  cobra.ExactArgs(1),
	RunE:  runFavStart,
}

var favName string

func init() {
	favAddCmd.Flags().StringVarP(&favName, "name", "n", "", "name for the favorite (required)")
	favAddCmd.MarkFlagRequired("name")

	favStartCmd.Flags().IntVarP(&streamPort, "port", "p", 0, "RTSP port (default: from config)")

	favCmd.AddCommand(favAddCmd)
	favCmd.AddCommand(favListCmd)
	favCmd.AddCommand(favRemoveCmd)
	favCmd.AddCommand(favStartCmd)
}

func initFavStore() error {
	if favStore != nil {
		return nil
	}

	var err error
	favStore, err = storage.NewFavoritesStorage(cfg.Storage.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize favorites storage: %w", err)
	}
	return nil
}

func runFavAdd(cmd *cobra.Command, args []string) error {
	if err := initFavStore(); err != nil {
		return err
	}

	url := args[0]

	if err := favStore.Add(favName, url); err != nil {
		return err
	}

	fmt.Printf("Added favorite '%s'\n", favName)
	fmt.Printf("  URL: %s\n", url)
	return nil
}

func runFavList(cmd *cobra.Command, args []string) error {
	if err := initFavStore(); err != nil {
		return err
	}

	favorites, err := favStore.List()
	if err != nil {
		return err
	}

	if len(favorites) == 0 {
		fmt.Println("No favorites saved yet.")
		fmt.Println("\nAdd a favorite with:")
		fmt.Println("  youtube-rtsp-proxy fav add <url> --name <name>")
		return nil
	}

	fmt.Printf("Favorites (%d):\n\n", len(favorites))
	for _, fav := range favorites {
		fmt.Printf("  %s\n", fav.Name)
		fmt.Printf("    URL: %s\n", fav.URL)
		fmt.Printf("    Created: %s\n", fav.CreatedAt.Format(time.RFC3339))
		if !fav.LastUsed.IsZero() {
			fmt.Printf("    Last used: %s\n", fav.LastUsed.Format(time.RFC3339))
		}
		fmt.Println()
	}

	return nil
}

func runFavRemove(cmd *cobra.Command, args []string) error {
	if err := initFavStore(); err != nil {
		return err
	}

	name := args[0]

	if err := favStore.Remove(name); err != nil {
		return err
	}

	fmt.Printf("Removed favorite '%s'\n", name)
	return nil
}

func runFavStart(cmd *cobra.Command, args []string) error {
	if err := initFavStore(); err != nil {
		return err
	}

	name := args[0]

	fav, err := favStore.Get(name)
	if err != nil {
		return err
	}

	// Update last used
	favStore.UpdateLastUsed(name)

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

	fmt.Printf("Starting favorite '%s'...\n", name)
	fmt.Printf("  URL: %s\n", fav.URL)

	if err := manager.Start(getContext(), fav.URL, name, port); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	// Get local IP for display
	localIP := getLocalIP()
	fmt.Printf("\nStream started!\n")
	fmt.Printf("  RTSP URL: rtsp://%s:%d/%s\n", localIP, port, name)

	return nil
}

// runFavInteractive provides interactive favorite selection with start/stop toggle
func runFavInteractive(cmd *cobra.Command, args []string) error {
	if err := initFavStore(); err != nil {
		return err
	}

	favorites, err := favStore.List()
	if err != nil {
		return err
	}

	// If no favorites, show help
	if len(favorites) == 0 {
		return cmd.Help()
	}

	// Get running streams to show status
	runningStreams := make(map[string]bool)
	if manager != nil {
		streams := manager.List()
		for _, s := range streams {
			runningStreams[s.Name] = true
		}
	}

	// Build selection items
	items := make([]string, 0, len(favorites)+1)
	nameMap := make(map[string]string) // display -> actual name

	for _, fav := range favorites {
		var status string
		if runningStreams[fav.Name] {
			status = "[▶ Running]"
		} else {
			status = "[⏹ Stopped]"
		}
		display := fmt.Sprintf("%-20s %s", fav.Name, status)
		items = append(items, display)
		nameMap[display] = fav.Name
	}

	// Add "add new" option
	items = append(items, addNewOption)

	// Show selection
	selected, err := SelectItem(items, "Select favorite to toggle:")
	if err != nil {
		return err
	}

	// Handle cancel
	if selected == "" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Handle add new
	if selected == addNewOption {
		return runFavInteractiveAdd()
	}

	// Get actual name from selection
	name := nameMap[selected]

	// Toggle: if running -> stop, if stopped -> start
	if runningStreams[name] {
		return runFavStop(name)
	}
	return runFavStartByName(name)
}

// runFavInteractiveAdd prompts for URL and name to add a new favorite
func runFavInteractiveAdd() error {
	url, err := PromptInput("Enter YouTube URL: ")
	if err != nil {
		return err
	}
	if url == "" {
		fmt.Println("Cancelled.")
		return nil
	}

	name, err := PromptInput("Enter name for this favorite: ")
	if err != nil {
		return err
	}
	if name == "" {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := favStore.Add(name, url); err != nil {
		return err
	}

	fmt.Printf("\nAdded favorite '%s'\n", name)
	fmt.Printf("  URL: %s\n", url)
	return nil
}

// runFavStop stops a running stream
func runFavStop(name string) error {
	fmt.Printf("Stopping '%s'...\n", name)
	if err := manager.Stop(name); err != nil {
		return fmt.Errorf("failed to stop stream: %w", err)
	}
	fmt.Printf("Stream '%s' stopped.\n", name)
	return nil
}

// runFavStartByName starts a stream by favorite name (reuses runFavStart logic)
func runFavStartByName(name string) error {
	fav, err := favStore.Get(name)
	if err != nil {
		return err
	}

	// Update last used
	favStore.UpdateLastUsed(name)

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

	// Use default port
	port := cfg.Server.RTSPPort

	fmt.Printf("Starting '%s'...\n", name)
	fmt.Printf("  URL: %s\n", fav.URL)

	if err := manager.Start(getContext(), fav.URL, name, port); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	// Get local IP for display
	localIP := getLocalIP()
	fmt.Printf("\nStream started!\n")
	fmt.Printf("  RTSP URL: rtsp://%s:%d/%s\n", localIP, port, name)

	return nil
}

