package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"dokvol/api/system"

	"github.com/spf13/cobra"
)

var drivesJSON bool

var drivesCmd = &cobra.Command{
	Use:   "drives",
	Short: "List and manage drives",
}

var drivesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available drives",
	RunE: func(cmd *cobra.Command, args []string) error {
		drives := system.GetDrives()

		if drivesJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(drives)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "DEVICE\tMOUNTPOINT\tFSTYPE\tTOTAL\tFREE\tUSED%")
		for _, d := range drives {
			fmt.Fprintf(w, "%s\t%s\t%s\t%dG\t%dG\t%.1f%%\n",
				d.Device, d.Mountpoint, d.Fstype, d.TotalGB, d.FreeGB, d.UsedPct)
		}
		w.Flush()
		return nil
	},
}

func init() {
	drivesListCmd.Flags().BoolVar(&drivesJSON, "json", false, "Output as JSON")
	drivesCmd.AddCommand(drivesListCmd)
	rootCmd.AddCommand(drivesCmd)
}
