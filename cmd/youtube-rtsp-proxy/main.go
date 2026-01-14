package main

import (
	"fmt"
	"os"

	"github.com/zerodice0/youtube-rtsp-proxy/internal/cli"
)

// Version and build info (set by ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Set version info in CLI package
	cli.Version = Version
	cli.BuildTime = BuildTime

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
