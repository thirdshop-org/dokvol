package handler

import (
	"net/http"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

type versionResponse struct {
	Version string `json:"version"`
}

func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, versionResponse{
		Version: system.VERSION,
	})
}
