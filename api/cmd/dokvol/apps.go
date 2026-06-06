package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"dokvol/api/system"

	"github.com/spf13/cobra"
)

var appsJSON bool

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List applications and their volumes",
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List applications with their Docker volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		apps := system.GetDockerVolumesByContainers()

		if appsJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(apps)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "APP\tSTATUS\tVOLUME\tTYPE\tSOURCE\tDESTINATION\tDRIVE\tMIGRATABLE")
		for _, app := range apps {
			for i, vol := range app.Volumes {
				appName := app.ContainerName
				if i > 0 {
					appName = ""
				}

				drive := ""
				if vol.SystemDrive != nil {
					drive = vol.SystemDrive.Mountpoint
				} else if vol.Type == "volume" {
					drive = "<docker>"
				}

				migratable := "✗"
				if vol.IsMigratable {
					migratable = "✓"
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					appName, app.Status, vol.Name, vol.Type,
					vol.Source, vol.Destination, drive, migratable)
			}
		}
		w.Flush()
		return nil
	},
}

func init() {
	appsListCmd.Flags().BoolVar(&appsJSON, "json", false, "Output as JSON")
	appsCmd.AddCommand(appsListCmd)
	rootCmd.AddCommand(appsCmd)
}
