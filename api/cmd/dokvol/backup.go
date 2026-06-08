package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"dokvol/api/internal/database"
	"dokvol/api/system/backup"

	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage backups and backup targets",
}

var backupTargetCmd = &cobra.Command{
	Use:   "target",
	Short: "Manage backup targets",
}

var backupTargetCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a backup target",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		provider, _ := cmd.Flags().GetString("provider")
		configJSON, _ := cmd.Flags().GetString("config-json")
		if name == "" || provider == "" || configJSON == "" {
			return fmt.Errorf("--name, --provider, and --config-json are required")
		}

		db := database.Init()
		defer db.Close()

		providerType := backup.ProviderType(provider)
		var config interface{}
		switch providerType {
		case backup.ProviderS3:
			config = &backup.S3Config{}
		case backup.ProviderSFTP:
			config = &backup.SFTPConfig{}
		case backup.ProviderLocal:
			config = &backup.LocalConfig{}
		default:
			return fmt.Errorf("invalid provider: %s (must be s3, sftp, local)", provider)
		}
		if err := json.Unmarshal([]byte(configJSON), config); err != nil {
			return fmt.Errorf("invalid config JSON: %w", err)
		}

		target, err := backup.CreateTarget(db.DB(), name, providerType, config)
		if err != nil {
			return fmt.Errorf("create target: %w", err)
		}
		fmt.Printf("Created backup target: %s (%s)\n", target.Name, target.ID)
		return nil
	},
}

var backupTargetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backup targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		db := database.Init()
		defer db.Close()

		targets, err := backup.ListTargets(db.DB())
		if err != nil {
			return fmt.Errorf("list targets: %w", err)
		}
		if len(targets) == 0 {
			fmt.Println("No backup targets found.")
			return nil
		}
		for _, t := range targets {
			fmt.Printf("- %s (%s) [%s]\n", t.Name, t.Provider, t.ID)
		}
		return nil
	},
}

var backupTargetTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection to a backup target",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		db := database.Init()
		defer db.Close()

		targets, err := backup.ListTargets(db.DB())
		if err != nil {
			return err
		}
		var found *backup.BackupTarget
		for _, t := range targets {
			if t.Name == name || t.ID == name {
				found = &t
				break
			}
		}
		if found == nil {
			return fmt.Errorf("target '%s' not found", name)
		}

		configJSON, err := backup.GetTargetDecryptedConfig(db.DB(), found.ID)
		if err != nil {
			return err
		}

		rcloneConfig, err := backup.BuildRcloneConfig(found.Provider, configJSON)
		if err != nil {
			return err
		}

		cmd2 := exec.CommandContext(cmd.Context(), "rclone", "lsf",
			"--config", "/dev/stdin",
			"--max-depth", "1",
			"backup-target:",
		)
		cmd2.Stdin = strings.NewReader(rcloneConfig)
		output, err := cmd2.CombinedOutput()
		if err != nil {
			return fmt.Errorf("connection test failed: %s\n%s", err, output)
		}
		fmt.Printf("Connection OK for '%s'\n%s", name, output)
		return nil
	},
}

var backupRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a backup for an application",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, _ := cmd.Flags().GetString("app")
		targetName, _ := cmd.Flags().GetString("target")
		if appName == "" || targetName == "" {
			return fmt.Errorf("--app and --target are required")
		}

		db := database.Init()
		defer db.Close()

		targets, err := backup.ListTargets(db.DB())
		if err != nil {
			return err
		}
		var targetID string
		for _, t := range targets {
			if t.Name == targetName || t.ID == targetName {
				targetID = t.ID
				break
			}
		}
		if targetID == "" {
			return fmt.Errorf("target '%s' not found", targetName)
		}

		engine := backup.NewBackupEngine(db.DB())
		jobID, err := engine.RunBackup(appName, targetID)
		if err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
		fmt.Printf("Backup started: %s\n", jobID)
		return nil
	},
}

var backupScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage backup schedules",
}

var backupScheduleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a backup schedule",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, _ := cmd.Flags().GetString("app")
		targetName, _ := cmd.Flags().GetString("target")
		cronExpr, _ := cmd.Flags().GetString("cron")
		retention, _ := cmd.Flags().GetInt("retention")
		if appName == "" || targetName == "" || cronExpr == "" {
			return fmt.Errorf("--app, --target, and --cron are required")
		}

		db := database.Init()
		defer db.Close()

		targets, err := backup.ListTargets(db.DB())
		if err != nil {
			return err
		}
		var targetID string
		for _, t := range targets {
			if t.Name == targetName || t.ID == targetName {
				targetID = t.ID
				break
			}
		}
		if targetID == "" {
			return fmt.Errorf("target '%s' not found", targetName)
		}

		schedule, err := backup.CreateSchedule(db.DB(), targetID, appName, cronExpr, retention)
		if err != nil {
			return fmt.Errorf("create schedule: %w", err)
		}
		fmt.Printf("Created schedule: %s (cron: %s, retention: %d)\n", schedule.ID, cronExpr, retention)
		return nil
	},
}

var backupScheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backup schedules",
	RunE: func(cmd *cobra.Command, args []string) error {
		db := database.Init()
		defer db.Close()

		rows, err := db.DB().Query(
			`SELECT s.id, t.name, s.app_name, s.cron_expr, s.retention, s.enabled
			 FROM backup_schedule s JOIN backup_target t ON t.id = s.target_id
			 ORDER BY s.created_at`,
		)
		if err != nil {
			return fmt.Errorf("list schedules: %w", err)
		}
		defer rows.Close()

		found := false
		for rows.Next() {
			found = true
			var id, targetName, appName, cronExpr string
			var retention, enabled int
			if err := rows.Scan(&id, &targetName, &appName, &cronExpr, &retention, &enabled); err != nil {
				continue
			}
			status := "enabled"
			if enabled == 0 {
				status = "disabled"
			}
			fmt.Printf("- [%s] %s -> %s: cron=%s ret=%d (%s)\n", id[:8], appName, targetName, cronExpr, retention, status)
		}
		if !found {
			fmt.Println("No schedules found.")
		}
		return nil
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore a backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID, _ := cmd.Flags().GetString("job-id")
		appName, _ := cmd.Flags().GetString("app")
		targetName, _ := cmd.Flags().GetString("target")
		destPath, _ := cmd.Flags().GetString("dest")
		if jobID == "" || appName == "" || targetName == "" {
			return fmt.Errorf("--job-id, --app, and --target are required")
		}

		db := database.Init()
		defer db.Close()

		targets, err := backup.ListTargets(db.DB())
		if err != nil {
			return err
		}
		var targetID string
		for _, t := range targets {
			if t.Name == targetName || t.ID == targetName {
				targetID = t.ID
				break
			}
		}
		if targetID == "" {
			return fmt.Errorf("target '%s' not found", targetName)
		}

		result, err := backup.RestoreBackup(db.DB(), backup.RestoreOptions{
			JobID:          jobID,
			AppName:        appName,
			TargetID:       targetID,
			DestMountpoint: destPath,
		})
		if err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}

		fmt.Printf("Restore completed: status=%s\n", result.Status)
		for _, v := range result.Volumes {
			fmt.Printf("  %s -> %s: %s\n", v.VolumeName, v.DestPath, v.Status)
		}
		return nil
	},
}

var backupListBackupsCmd = &cobra.Command{
	Use:   "list",
	Short: "List backups for an application",
	RunE: func(cmd *cobra.Command, args []string) error {
		appName, _ := cmd.Flags().GetString("app")
		targetName, _ := cmd.Flags().GetString("target")
		if appName == "" || targetName == "" {
			return fmt.Errorf("--app and --target are required")
		}

		db := database.Init()
		defer db.Close()

		targets, err := backup.ListTargets(db.DB())
		if err != nil {
			return err
		}
		var targetID string
		for _, t := range targets {
			if t.Name == targetName || t.ID == targetName {
				targetID = t.ID
				break
			}
		}
		if targetID == "" {
			return fmt.Errorf("target '%s' not found", targetName)
		}

		entries, err := backup.ListBackups(db.DB(), targetID, appName)
		if err != nil {
			return fmt.Errorf("list backups: %w", err)
		}
		if len(entries) == 0 {
			fmt.Printf("No backups found for '%s' on '%s'.\n", appName, targetName)
			return nil
		}
		for _, e := range entries {
			fmt.Printf("- %s (%s)\n", e.Path, e.ModifiedAt.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.AddCommand(backupTargetCmd)
	backupCmd.AddCommand(backupRunCmd)
	backupCmd.AddCommand(backupScheduleCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	backupCmd.AddCommand(backupListBackupsCmd)

	backupTargetCmd.AddCommand(backupTargetCreateCmd)
	backupTargetCmd.AddCommand(backupTargetListCmd)
	backupTargetCmd.AddCommand(backupTargetTestCmd)

	backupScheduleCmd.AddCommand(backupScheduleCreateCmd)
	backupScheduleCmd.AddCommand(backupScheduleListCmd)

	backupTargetCreateCmd.Flags().String("name", "", "Target name")
	backupTargetCreateCmd.Flags().String("provider", "", "Provider (s3, sftp, local)")
	backupTargetCreateCmd.Flags().String("config-json", "", "Provider config as JSON")
	backupTargetTestCmd.Flags().String("name", "", "Target name or ID")

	backupRunCmd.Flags().String("app", "", "Application name")
	backupRunCmd.Flags().String("target", "", "Target name or ID")

	backupScheduleCreateCmd.Flags().String("app", "", "Application name")
	backupScheduleCreateCmd.Flags().String("target", "", "Target name or ID")
	backupScheduleCreateCmd.Flags().String("cron", "", "Cron expression")
	backupScheduleCreateCmd.Flags().Int("retention", 7, "Number of backups to retain")

	backupRestoreCmd.Flags().String("job-id", "", "Backup job ID")
	backupRestoreCmd.Flags().String("app", "", "Application name")
	backupRestoreCmd.Flags().String("target", "", "Target name or ID")
	backupRestoreCmd.Flags().String("dest", "", "Restore destination path (optional)")

	backupListBackupsCmd.Flags().String("app", "", "Application name")
	backupListBackupsCmd.Flags().String("target", "", "Target name or ID")
}
