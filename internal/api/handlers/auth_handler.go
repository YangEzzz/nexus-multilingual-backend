package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/repository"
	"choice-matrix-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userRepo *repository.UserRepository
}

func NewAuthHandler(userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{userRepo: userRepo}
}

// LoginRequest requests username, password and project mapping ID
type LoginRequest struct {
	Username        string `json:"username" binding:"required"`
	Password        string `json:"password" binding:"required"`
	ProjectIDString string `json:"project_id_string" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的请求参数: " + err.Error()})
		return
	}

	// 1. 调用外部管理系统 API
	requestBody, _ := json.Marshal(req)
	resp, err := http.Post("http://localhost:3456/api/external/login", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "无法连接到认证服务器: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusOK, gin.H{"code": 401, "message": "认证失败或该用户不是项目成员"})
		return
	}

	// 2. 解析外部系统返回的结果
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取响应失败"})
		return
	}
	fmt.Printf("Management System Response: %s\n", string(bodyBytes))

	var extResp struct {
		Code int `json:"code"`
		Data struct {
			RoleInProject string `json:"role_in_project"`
			Token         string `json:"token"`
			User          struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Email  string `json:"email"`
				Role   string `json:"role"`
				Avatar string `json:"avatar"`
			} `json:"user"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(bodyBytes, &extResp); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "无法解析认证服务器的响应"})
		return
	}

	// 提取管理系统那边的 user 信息
	externalID := extResp.Data.User.ID
	nickname := extResp.Data.User.Name
	avatarURL := extResp.Data.User.Avatar
	projectRole := extResp.Data.RoleInProject

	// 3. 在本地数据库进行 Upsert 操作
	user, err := h.userRepo.FindByExternalID(externalID)
	if err != nil {
		// 不存在则创建
		user = &models.User{
			ExternalID: externalID,
			Nickname:   nickname,
			AvatarURL:  avatarURL,
			Role:       projectRole,
		}
		if err := h.userRepo.Create(user); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存用户信息失败"})
			return
		}
	} else {
		// 存在则更新最新信息
		user.Nickname = nickname
		user.AvatarURL = avatarURL
		user.Role = projectRole
		h.userRepo.Update(user)
	}

	// 4. 生成 Nexus 自己的 JWT Token (或者你也可以直接透传 external_token，但建议自己发)
	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"message": "登录成功",
		"data": gin.H{
			"token": token,
			"user": gin.H{
				"id":          user.ID,
				"external_id": user.ExternalID,
				"nickname":    user.Nickname,
				"avatar":      user.AvatarURL,
				"role":        user.Role,
			},
		},
	})
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
		"code": 200,
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
