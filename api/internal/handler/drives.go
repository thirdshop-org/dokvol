package handler

import (
	"net/http"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func GetDrives(c *gin.Context) {
	drives := system.GetDrives()
	if drives == nil {
		drives = []system.DriveInfo{}
	}
	c.JSON(http.StatusOK, drives)
}
