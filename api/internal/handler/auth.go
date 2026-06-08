package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"dokvol/api/internal/auth"
	"dokvol/api/internal/db"

	"github.com/gin-gonic/gin"
)

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type authUserResponse struct {
	ID                      int64  `json:"id"`
	Email                   string `json:"email"`
	Username                string `json:"username"`
	Role                    string `json:"role"`
	PasswordChangeRequired  bool   `json:"password_change_required"`
	CreatedAt               string `json:"created_at"`
}

type authResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	User         authUserResponse `json:"user"`
}

func userToResponse(user db.User) authUserResponse {
	return authUserResponse{
		ID:                     user.ID,
		Email:                  user.Email.String,
		Username:               user.Username,
		Role:                   user.Role,
		PasswordChangeRequired: user.PasswordChangeRequired == 1,
		CreatedAt:              user.CreatedAt.Time.Format(time.RFC3339),
	}
}

func Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error_code": "AUTH.VALIDATION_ERROR",
			"message":    err.Error(),
		})
		return
	}

	ctx := context.Background()

	existing, err := DB.GetUserByEmail(ctx, sql.NullString{String: req.Email, Valid: true})
	if err == nil && existing.ID != 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error_code": "AUTH.EMAIL_EXISTS",
			"message":    "Email already registered",
		})
		return
	}

	existing, err = DB.GetUserByUsername(ctx, req.Username)
	if err == nil && existing.ID != 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error_code": "AUTH.USERNAME_EXISTS",
			"message":    "Username already taken",
		})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to hash password",
		})
		return
	}

	user, err := DB.CreateUser(ctx, db.CreateUserParams{
		Email:                sql.NullString{String: req.Email, Valid: req.Email != ""},
		Username:             req.Username,
		PasswordHash:         hash,
		Role:                 "user",
		PasswordChangeRequired: 0,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to create user",
		})
		return
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to generate token",
		})
		return
	}

	refreshToken, expiresAt, err := auth.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to generate refresh token",
		})
		return
	}

	_, err = DB.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to store refresh token",
		})
		return
	}

	c.JSON(http.StatusCreated, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         userToResponse(user),
	})
}

func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error_code": "AUTH.VALIDATION_ERROR",
			"message":    err.Error(),
		})
		return
	}

	ctx := context.Background()

	user, err := DB.GetUserByEmail(ctx, sql.NullString{String: req.Email, Valid: true})
	if err != nil {
		user, err = DB.GetUserByUsername(ctx, req.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error_code": "AUTH.INVALID_CREDENTIALS",
				"message":    "Invalid email or password",
			})
			return
		}
	}

	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error_code": "AUTH.INVALID_CREDENTIALS",
			"message":    "Invalid email or password",
		})
		return
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to generate token",
		})
		return
	}

	refreshToken, expiresAt, err := auth.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to generate refresh token",
		})
		return
	}

	_, err = DB.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to store refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         userToResponse(user),
	})
}

func RefreshToken(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error_code": "AUTH.VALIDATION_ERROR",
			"message":    err.Error(),
		})
		return
	}

	ctx := context.Background()

	stored, err := DB.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error_code": "AUTH.INVALID_REFRESH_TOKEN",
			"message":    "Invalid or expired refresh token",
		})
		return
	}

	if stored.ExpiresAt.Before(time.Now()) {
		DB.DeleteRefreshToken(ctx, req.RefreshToken)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error_code": "AUTH.REFRESH_TOKEN_EXPIRED",
			"message":    "Refresh token has expired",
		})
		return
	}

	user, err := DB.GetUserByID(ctx, stored.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error_code": "AUTH.USER_NOT_FOUND",
			"message":    "User not found",
		})
		return
	}

	DB.DeleteRefreshToken(ctx, req.RefreshToken)

	accessToken, err := auth.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to generate token",
		})
		return
	}

	newRefreshToken, expiresAt, err := auth.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to generate refresh token",
		})
		return
	}

	_, err = DB.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    user.ID,
		Token:     newRefreshToken,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to store refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

func Logout(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error_code": "AUTH.VALIDATION_ERROR",
			"message":    err.Error(),
		})
		return
	}

	ctx := context.Background()
	DB.DeleteRefreshToken(ctx, req.RefreshToken)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func GetCurrentUser(c *gin.Context) {
	userID := c.GetInt64("user_id")
	ctx := context.Background()

	user, err := DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error_code": "AUTH.USER_NOT_FOUND",
			"message":    "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, userToResponse(user))
}

func ChangePassword(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error_code": "AUTH.VALIDATION_ERROR",
			"message":    err.Error(),
		})
		return
	}

	ctx := context.Background()

	user, err := DB.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error_code": "AUTH.USER_NOT_FOUND",
			"message":    "User not found",
		})
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.OldPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error_code": "AUTH.WRONG_PASSWORD",
			"message":    "Current password is incorrect",
		})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to hash password",
		})
		return
	}

	if err := DB.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		PasswordHash: hash,
		ID:           userID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to update password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func ListUsers(c *gin.Context) {
	ctx := context.Background()

	users, err := DB.ListUsers(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error_code": "INTERNAL_ERROR",
			"message":    "Failed to list users",
		})
		return
	}

	result := make([]authUserResponse, len(users))
	for i, u := range users {
		result[i] = userToResponse(u)
	}

	c.JSON(http.StatusOK, result)
}
