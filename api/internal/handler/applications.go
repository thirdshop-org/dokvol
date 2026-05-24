package handler

import (
	"net/http"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func GetApplications(c *gin.Context) {
	apps := system.GetDockerVolumesByContainers()
	if apps == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}
	c.JSON(http.StatusOK, apps)
}
