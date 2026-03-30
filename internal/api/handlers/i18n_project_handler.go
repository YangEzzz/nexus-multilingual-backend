package handlers

import (
	"net/http"
	"strconv"

	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type I18nProjectHandler struct {
	repo     *repository.I18nProjectRepository
	termRepo *repository.TermRepository
}

func NewI18nProjectHandler(repo *repository.I18nProjectRepository, termRepo *repository.TermRepository) *I18nProjectHandler {
	return &I18nProjectHandler{repo: repo, termRepo: termRepo}
}

// List GET /projects
func (h *I18nProjectHandler) List(c *gin.Context) {
	projects, err := h.repo.List()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取项目列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": projects})
}

// Get GET /projects/:id
func (h *I18nProjectHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "项目不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": project})
}

// Dashboard GET /projects/:id/dashboard
func (h *I18nProjectHandler) Dashboard(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "项目不存在"})
		return
	}

	dashboard, err := h.termRepo.Dashboard(uint(id), 8)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取仪表盘数据失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data": gin.H{
			"project":   project,
			"stats":     dashboard.Stats,
			"languages": dashboard.Languages,
			"recent_logs": dashboard.RecentLogs,
			"activity_trend": dashboard.ActivityTrend,
			"focus":     dashboard.Focus,
			"last_activity_at": dashboard.LastActivityAt,
		},
	})
}

// Create POST /projects
func (h *I18nProjectHandler) Create(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Code        string `json:"code" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("userID")

	project := &models.I18nProject{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		CreatedByID: userID.(uint),
	}
	if err := h.repo.Create(project); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "创建成功", "data": project})
}

// Update PUT /projects/:id
func (h *I18nProjectHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "项目不存在"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		AiPrompt    string `json:"ai_prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	if req.Name != "" {
		project.Name = req.Name
	}
	project.Description = req.Description
	project.AiPrompt = req.AiPrompt

	if err := h.repo.Update(project); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存项目配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "更新成功", "data": project})
}

// Delete DELETE /projects/:id
func (h *I18nProjectHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	if err := h.repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}
