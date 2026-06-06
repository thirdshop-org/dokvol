package main

import (
	"fmt"
	"os"
	"time"

	"dokvol/api/internal/database"
	"dokvol/api/internal/handler"
	"dokvol/api/system"

	"github.com/spf13/cobra"
)

var migrateDest string

var migrateCmd = &cobra.Command{
	Use:   "migrate <app-name> --dest <mountpoint>",
	Short: "Migrate all volumes of an application to a target drive",
	Long: `Migrate all volumes of a Docker application to another drive.

Steps performed for each volume:
  1. Stop the container
  2. rsync data to destination drive
  3. Verify checksum
  4. Replace source with symlink to destination
  5. Start the container

Migration is blocking: progress is displayed in real-time.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]

		if migrateDest == "" {
			return fmt.Errorf("destination mountpoint is required (--dest /mnt/sdb)")
		}

		db := database.Init()
		defer db.Close()
		handler.MigrationManager = system.NewMigrationManager(db.Queries)
		manager := handler.MigrationManager

		s, err := system.New()
		if err != nil {
			return fmt.Errorf("failed to initialize system: %w", err)
		}

		var app *system.Application
		for i := range s.Applications {
			if s.Applications[i].Name == appName {
				app = &s.Applications[i]
				break
			}
		}
		if app == nil {
			return fmt.Errorf("application '%s' not found", appName)
		}

		drives := system.GetDrives()
		var destDrive *system.DriveInfo
		for _, d := range drives {
			if d.Mountpoint == migrateDest {
				destDrive = &d
				break
			}
		}
		if destDrive == nil {
			return fmt.Errorf("no drive found with mountpoint '%s'", migrateDest)
		}

		volumeOpts := make([]system.ApplicationVolumeOptions, len(app.DockerVolumes))
		for i, vol := range app.DockerVolumes {
			volumeOpts[i] = system.ApplicationVolumeOptions{
				VolumeDetail:     vol,
				DestinationDrive: *destDrive,
			}
		}

		ctx := cmd.Context()
		jobID, err := manager.StartJob(ctx, appName, *app, volumeOpts)
		if err != nil {
			return fmt.Errorf("failed to start migration: %w", err)
		}

		fmt.Printf("Migration of '%s' to %s started (job: %s)\n\n", appName, migrateDest, jobID[:8])

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			job, err := manager.GetJob(ctx, jobID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error fetching job status: %s\n", err)
				continue
			}

			printProgress(job)

			if job.Status == system.JobCompleted {
				fmt.Println("\n✓ Migration completed successfully!")
				return nil
			}
			if job.Status == system.JobFailed {
				fmt.Println("\n✗ Migration failed!")
				for _, vol := range job.Volumes {
					if vol.Error != "" {
						fmt.Fprintf(os.Stderr, "  Volume '%s': %s\n", vol.VolumeName, vol.Error)
					}
				}
				return fmt.Errorf("migration failed")
			}
		}

		return nil
	},
}

func printProgress(job *system.Job) {
	const clear = "\033[2J\033[H"
	fmt.Print(clear)

	fmt.Printf("Migration of '%s' — Status: %s\n\n", job.AppName, job.Status)
	fmt.Printf("%-20s %-14s %-22s  %s\n", "VOLUME", "STEP", "PROGRESS", "STATUS")
	fmt.Println("---------------------------------------------------------------")

	for _, vol := range job.Volumes {
		stepDisplay := vol.Step
		progress := "—"
		if vol.TotalBytes > 0 {
			progress = fmt.Sprintf("%s / %s", formatBytes(vol.Transferred), formatBytes(vol.TotalBytes))
		}

		statusSymbol := "○"
		switch vol.Step {
		case system.StepCompleted:
			statusSymbol = "✓"
		case system.StepFailed:
			statusSymbol = "✗"
		case system.StepPending:
			statusSymbol = "○"
		default:
			statusSymbol = "●"
		}

		fmt.Printf("%-20s %-14s %-22s  %s\n", vol.VolumeName, stepDisplay, progress, statusSymbol)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func init() {
	migrateCmd.Flags().StringVarP(&migrateDest, "dest", "d", "", "Destination drive mountpoint (e.g. /mnt/sdb)")
	migrateCmd.MarkFlagRequired("dest")
	rootCmd.AddCommand(migrateCmd)
}
