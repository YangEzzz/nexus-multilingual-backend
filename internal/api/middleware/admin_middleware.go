package middleware

import (
	"net/http"

	"choice-matrix-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminMiddleware protects global configuration mutations.
func AdminMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDValue, exists := c.Get("userID")
		userID, ok := userIDValue.(uint)
		if !exists || !ok {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权管理全局配置"})
			c.Abort()
			return
		}

		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权管理全局配置"})
			c.Abort()
			return
		}
		if user.ID != 1 && user.Role != "admin" && user.Role != "owner" {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "仅管理员可以管理全局配置"})
			c.Abort()
			return
		}
		c.Next()
	}
}
