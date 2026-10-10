package handlers

import (
	"choice-matrix-backend/internal/repository"
	"github.com/gin-gonic/gin"
	"net/http"
)

type AuthHandler struct{ userRepo *repository.UserRepository }

func NewAuthHandler(userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{userRepo: userRepo}
}

// GetCurrentUser returns the profile of the currently logged-in user
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusOK, gin.H{"code": 401, "message": "未授权的访问"})
		return
	}

	user, err := h.userRepo.FindByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "用户不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"id":          user.ID,
			"external_id": user.ExternalID,
			"nickname":    user.Nickname,
			"avatar":      user.AvatarURL,
			"role":        user.Role,
		},
	})
}
