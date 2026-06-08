package middleware

import (
	"net/http"
	"strings"

	"dokvol/api/internal/auth"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		tokenString := auth.ExtractBearerToken(authHeader)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error_code": "AUTH.UNAUTHORIZED",
				"message":    "Missing or invalid authorization header",
			})
			return
		}

		claims, err := auth.ValidateAccessToken(tokenString)
		if err != nil {
			if strings.Contains(err.Error(), "token is expired") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error_code": "AUTH.TOKEN_EXPIRED",
					"message":    "Access token has expired",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error_code": "AUTH.INVALID_TOKEN",
				"message":    "Invalid access token",
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error_code": "AUTH.FORBIDDEN",
				"message":    "Admin access required",
			})
			return
		}
		c.Next()
	}
}
