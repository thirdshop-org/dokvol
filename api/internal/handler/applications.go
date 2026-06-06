package handler

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func StopApplication(c *gin.Context) {
	name := c.Param("name")
	if err := system.StopContainer(name); err != nil {
		_ = c.Error(err)
		apiErr, ok := err.(*system.APIError)
		if ok {
			c.JSON(apiErr.HTTPStatus(), apiErr)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func StartApplication(c *gin.Context) {
	name := c.Param("name")
	if err := system.StartContainer(name); err != nil {
		_ = c.Error(err)
		apiErr, ok := err.(*system.APIError)
		if ok {
			c.JSON(apiErr.HTTPStatus(), apiErr)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func RestartApplication(c *gin.Context) {
	name := c.Param("name")
	if err := system.RestartContainer(name); err != nil {
		_ = c.Error(err)
		apiErr, ok := err.(*system.APIError)
		if ok {
			c.JSON(apiErr.HTTPStatus(), apiErr)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func GetApplications(c *gin.Context) {
	apps := system.GetDockerVolumesByContainers()
	if apps == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	drives := system.GetDrives()
	sort.Slice(drives, func(i, j int) bool {
		return len(drives[i].Mountpoint) > len(drives[j].Mountpoint)
	})

	for i := range apps {
		for j := range apps[i].Volumes {
			v := &apps[i].Volumes[j]

			var bestMatch *system.DriveInfo
			for _, d := range drives {
				m := d.Mountpoint
				if m != "/" && !strings.HasSuffix(m, "/") {
					m += "/"
				}
				if v.Source == d.Mountpoint || strings.HasPrefix(v.Source, m) {
					bestMatch = &d
					break
				}
			}
			v.SystemDrive = bestMatch

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
