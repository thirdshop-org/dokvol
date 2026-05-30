package main

import (
	"log"
	"os"

	"dokvol/api/internal/database"
	"dokvol/api/internal/handler"
	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func main() {
	db := database.Init()
	defer db.Close()

	handler.MigrationManager = system.NewMigrationManager(db.Queries)

	collector := system.NewStatsCollector(db.Queries)
	go collector.Start()
	defer collector.Stop()

	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/applications", handler.GetApplications)
		api.GET("/volumes", handler.GetVolumes)
		api.GET("/drives", handler.GetDrives)
		api.POST("/drives/init", handler.InitDrive)
		api.GET("/drives/health", handler.CheckDriveHealth)
		api.GET("/health", handler.GetSystemHealth)
		api.POST("/volumes/migrate", handler.MigrateVolume)
		api.GET("/volumes/migrate", handler.GetMigrationJobs)
		api.GET("/volumes/migrate/:id", handler.GetMigrationJob)

		api.GET("/preferences", handler.GetPreferences)
		api.PUT("/preferences", handler.UpdatePreference)

		api.GET("/stats/volumes", handler.ListStatsVolume)
		api.GET("/stats/drives", handler.ListStatsDrive)
		api.GET("/stats/applications", handler.ListStatsApplication)
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("DokVol API listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
