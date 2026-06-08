package main

import (
	"context"
	"log"
	"os"
	"strings"

	"dokvol/api/internal/auth"
	"dokvol/api/internal/database"
	"dokvol/api/internal/db"
	"dokvol/api/internal/handler"
	"dokvol/api/internal/middleware"
	"dokvol/api/system"
	"dokvol/api/system/backup"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func bootstrapAdmin(queries *db.Queries) {
	ctx := context.Background()

	users, err := queries.ListUsers(ctx)
	if err != nil {
		log.Fatalf("auth: failed to check existing users: %v", err)
	}
	for _, u := range users {
		if u.Role == "admin" {
			log.Println("auth: admin user found, skipping bootstrap")
			return
		}
	}

	username := os.Getenv("ADMIN_USERNAME")
	password := os.Getenv("ADMIN_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("auth: no admin user in database and ADMIN_USERNAME/ADMIN_PASSWORD not set in .env")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("auth: failed to hash password: %v", err)
	}

	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		Email:                  db.User{}.Email,
		Username:               username,
		PasswordHash:           hash,
		Role:                   "admin",
		PasswordChangeRequired: 1,
	})
	if err != nil {
		log.Fatalf("auth: failed to create admin user: %v", err)
	}

	log.Printf("auth: admin '%s' created from .env (ID: %d)", user.Username, user.ID)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the DokVol HTTP API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPaths := []string{"/etc/dokvol/.env", ".env", os.Getenv("DOKVOL_ENV")}
		loaded := false
		for _, p := range configPaths {
			if p == "" {
				continue
			}
			if err := godotenv.Load(p); err == nil {
				loaded = true
				log.Printf("config: loaded %s", p)
				break
			}
		}
		if !loaded {
			log.Println("config: no .env file found, using system environment")
		}
		database.Init()
		db := database.Init()
		defer db.Close()

		handler.DB = db.Queries
		handler.MigrationManager = system.NewMigrationManager(db.Queries)

		handler.BackupEngine = backup.NewBackupEngine(db.DB())
		backupScheduler := backup.NewBackupScheduler(db.DB(), handler.BackupEngine)
		go backupScheduler.Start()
		defer backupScheduler.Stop()

		bootstrapAdmin(db.Queries)

		collector := system.NewStatsCollector(db.Queries)
		go collector.Start()
		defer collector.Stop()

		r := gin.Default()

		apiPublic := r.Group("/api")
		{
			apiPublic.GET("/health", handler.GetSystemHealth)
			apiPublic.GET("/version", handler.GetVersion)
			apiPublic.POST("/auth/login", handler.Login)
			apiPublic.POST("/auth/register", handler.Register)
			apiPublic.POST("/auth/refresh", handler.RefreshToken)
		}

		apiProtected := r.Group("/api")
		apiProtected.Use(middleware.AuthRequired())
		{
			apiProtected.POST("/auth/logout", handler.Logout)
			apiProtected.GET("/auth/me", handler.GetCurrentUser)
			apiProtected.POST("/auth/change-password", handler.ChangePassword)

			apiProtected.GET("/auth/users", middleware.AdminRequired(), handler.ListUsers)

			apiProtected.GET("/applications", handler.GetApplications)
			apiProtected.POST("/applications/:name/stop", handler.StopApplication)
			apiProtected.POST("/applications/:name/start", handler.StartApplication)
			apiProtected.POST("/applications/:name/restart", handler.RestartApplication)
			apiProtected.GET("/volumes", handler.GetVolumes)
			apiProtected.GET("/drives", handler.GetDrives)
			apiProtected.POST("/drives/init", handler.InitDrive)
			apiProtected.GET("/drives/health", handler.CheckDriveHealth)
			apiProtected.POST("/volumes/migrate", handler.MigrateVolume)
			apiProtected.GET("/volumes/migrate", handler.GetMigrationJobs)
			apiProtected.GET("/volumes/migrate/:id", handler.GetMigrationJob)
			apiProtected.DELETE("/volumes", handler.DeleteVolumes)
			apiProtected.POST("/volumes/browse", handler.BrowseVolume)
			apiProtected.POST("/volumes/read-file", handler.ReadVolumeFile)

			apiProtected.GET("/preferences", handler.GetPreferences)
			apiProtected.PUT("/preferences", handler.UpdatePreference)

			apiProtected.GET("/stats/volumes", handler.ListStatsVolume)
			apiProtected.GET("/stats/drives", handler.ListStatsDrive)
			apiProtected.GET("/stats/applications", handler.ListStatsApplication)
			apiProtected.GET("/stats/migrations", handler.ListStatsMigration)

			apiProtected.GET("/history", handler.ListHistory)
			apiProtected.GET("/history/names", handler.ListHistoryAppNames)
			apiProtected.POST("/history/rescan", handler.RescanHistory)
			apiProtected.GET("/history/:id", handler.GetHistoryJob)

			apiProtected.GET("/backup/targets", handler.ListBackupTargets)
			apiProtected.POST("/backup/targets", handler.CreateBackupTarget)
			apiProtected.PUT("/backup/targets/:id", handler.UpdateBackupTarget)
			apiProtected.DELETE("/backup/targets/:id", handler.DeleteBackupTarget)
			apiProtected.POST("/backup/targets/:id/test", handler.TestBackupTarget)
			apiProtected.POST("/backup/run", handler.RunBackup)
			apiProtected.GET("/backup/jobs", handler.ListBackupJobs)
			apiProtected.GET("/backup/jobs/:id", handler.GetBackupJob)
			apiProtected.GET("/backup/targets/:id/backups", handler.ListBackupsOnTarget)
			apiProtected.POST("/backup/restore", handler.RestoreBackup)
			apiProtected.GET("/backup/schedules", handler.ListBackupSchedules)
			apiProtected.POST("/backup/schedules", handler.CreateBackupSchedule)
			apiProtected.PUT("/backup/schedules/:id", handler.UpdateBackupSchedule)
			apiProtected.DELETE("/backup/schedules/:id", handler.DeleteBackupSchedule)
		}

		staticDir := "/usr/local/share/dokvol/static"
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			filePath := staticDir + c.Request.URL.Path
			if _, err := os.Stat(filePath); err == nil {
				c.File(filePath)
				return
			}
			c.File(staticDir + "/index.html")
		})

		addr := os.Getenv("LISTEN_ADDR")
		if addr == "" {
			addr = ":8080"
		}

		log.Printf("DokVol API listening on %s", addr)
		if err := r.Run(addr); err != nil {
			log.Fatal(err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
