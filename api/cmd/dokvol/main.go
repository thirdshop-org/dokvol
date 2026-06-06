package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dokvol",
	Short: "Docker Volume Manager — CLI & Server",
	Long: `DokVol is a lightweight appliance that scans Docker volumes,
tracks disk usage, and lets you migrate data between drives.

Use 'dokvol server' to start the HTTP API server,
or use the subcommands for direct CLI operations.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
