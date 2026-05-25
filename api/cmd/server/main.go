package main

import (
	"log"
	"os"

	"dokvol/api/internal/database"
	"dokvol/api/internal/handler"

	"github.com/gin-gonic/gin"
)

func main() {
	db := database.Init()
	defer db.Close()

	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/applications", handler.GetApplications)
		api.GET("/volumes", handler.GetVolumes)
		api.GET("/drives", handler.GetDrives)
		api.POST("/drives/init", handler.InitDrive)
		api.GET("/drives/health", handler.CheckDriveHealth)
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
