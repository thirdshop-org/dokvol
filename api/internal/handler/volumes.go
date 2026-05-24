package handler

import (
	"net/http"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func GetVolumes(c *gin.Context) {
	apps := system.GetDockerVolumesByContainers()
	if apps == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	var volumes []system.VolumeDetail
	for _, app := range apps {
		volumes = append(volumes, app.Volumes...)
	}

	if volumes == nil {
		volumes = []system.VolumeDetail{}
	}

	c.JSON(http.StatusOK, volumes)
}
