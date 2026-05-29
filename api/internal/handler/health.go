package handler

import (
	"errors"
	"net/http"

	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

func GetSystemHealth(c *gin.Context) {
	if err := system.CheckSystemHealth(); err != nil {
		var apiErr *system.APIError
		if errors.As(err, &apiErr) {
			c.JSON(apiErr.HTTPStatus(), apiErr)
			return
		}
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			err.Error(),
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, gin.H{"healthy": true})
}
