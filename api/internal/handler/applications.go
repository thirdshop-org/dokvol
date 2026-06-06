package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func GetApplications(c *gin.Context) {
	apps := system.GetDockerVolumesByContainers()
	if apps == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	drives := system.GetDrives()
	for i := range apps {
		for j := range apps[i].Volumes {
			v := &apps[i].Volumes[j]
			if resolved, err := filepath.EvalSymlinks(v.Source); err == nil {
				if idx := strings.Index(resolved, "/"+system.DOKVOL_FOLDER+"/"); idx != -1 {
					driveMount := resolved[:idx]
					for _, d := range drives {
						if d.Mountpoint == driveMount {
							v.MigratedDriveMountpoint = d.Mountpoint
							v.MigratedDestPath = resolved
							break
						}
					}
					if v.MigratedDriveMountpoint == "" {
						v.MigratedDriveMountpoint = driveMount
						v.MigratedDestPath = resolved
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, apps)
}
