package handler

import (
	"net/http"

	"dokvol/api/internal/db"
	"dokvol/api/system"

	"github.com/gin-gonic/gin"
)

type preferenceUpdate struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func GetPreferences(c *gin.Context) {
	prefs, err := MigrationManager.Queries.ListPreferences(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			err.Error(),
			nil,
		))
		return
	}

	result := make(map[string]string, len(prefs))
	for _, p := range prefs {
		result[p.Key] = p.Value
	}

	c.JSON(http.StatusOK, result)
}

func UpdatePreference(c *gin.Context) {
	var req preferenceUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"invalid request body, 'key' and 'value' required",
			nil,
		))
		return
	}
	if req.Key == "" {
		c.JSON(http.StatusBadRequest, system.NewAPIError(
			"INTERNAL_ERROR",
			"'key' is required",
			nil,
		))
		return
	}

	if err := MigrationManager.Queries.UpsertPreference(c.Request.Context(), db.UpsertPreferenceParams{
		Key:   req.Key,
		Value: req.Value,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, system.NewAPIError(
			"INTERNAL_ERROR",
			err.Error(),
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": req.Key, "value": req.Value})
}
