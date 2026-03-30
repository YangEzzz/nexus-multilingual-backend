package handlers

import (
	"net/http"
	"strconv"

	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type LanguageHandler struct {
	repo *repository.LanguageRepository
}

func NewLanguageHandler(repo *repository.LanguageRepository) *LanguageHandler {
	return &LanguageHandler{repo: repo}
}

// Get GET /projects/:id/languages
func (h *LanguageHandler) Get(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	langs, err := h.repo.GetByProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取语言配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": langs})
}

// Update POST /projects/:id/languages/update
func (h *LanguageHandler) Update(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	var req []struct {
		Code     string `json:"code" binding:"required"`
		Name     string `json:"name" binding:"required"`
		IsSource bool   `json:"is_source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	langs := make([]models.Language, 0, len(req))
	for _, item := range req {
		langs = append(langs, models.Language{
			ProjectID: uint(projectID),
			Code:      item.Code,
			Name:      item.Name,
			IsSource:  item.IsSource,
		})
	}

	if err := h.repo.SetForProject(uint(projectID), langs); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "更新语言配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "语言配置已更新"})
}
