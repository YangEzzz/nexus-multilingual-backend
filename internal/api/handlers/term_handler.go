package handlers

import (
	"net/http"
	"strconv"

	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/repository"

	"github.com/gin-gonic/gin"
)

type TermHandler struct {
	repo *repository.TermRepository
}

func NewTermHandler(repo *repository.TermRepository) *TermHandler {
	return &TermHandler{repo: repo}
}

// List GET /projects/:id/terms
func (h *TermHandler) List(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	terms, err := h.repo.ListByProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取词条失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": terms})
}

// GetStats GET /projects/:id/stats
func (h *TermHandler) GetStats(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	stats, err := h.repo.Stats(uint(projectID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取统计失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": stats})
}

// Create POST /projects/:id/terms
func (h *TermHandler) Create(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	var req struct {
		Module       string            `json:"module"`
		Key          string            `json:"key" binding:"required"`
		Description  string            `json:"description"`
		Status       models.TermStatus `json:"status"`
		Translations map[string]string `json:"translations"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("userID")

	// Construct Term
	term := &models.Term{
		ProjectID:   uint(projectID),
		Module:      req.Module,
		Key:         req.Key,
		Description: req.Description,
		Status:      models.TermStatusDraft,
	}

	// New terms always start as draft; status advances automatically when translations are saved
	// (pending = some translations missing, review = all filled, published = manually triggered)
	if req.Status != "" {
		term.Status = req.Status
	}
	// If translations are provided at create time, set to pending
	hasTranslation := false
	for _, v := range req.Translations {
		if v != "" {
			hasTranslation = true
			break
		}
	}
	if hasTranslation && term.Status == models.TermStatusDraft {
		term.Status = models.TermStatusPending
	}

	// Construct Translations
	for langCode, content := range req.Translations {
		if content != "" {
			term.Translations = append(term.Translations, models.Translation{
				LanguageCode: langCode,
				Content:      content,
			})
		}
	}

	// Construct Initial History
	term.HistoryLogs = append(term.HistoryLogs, models.HistoryLog{
		UserID: userID.(uint),
		Action: "创建了词条",
	})

	if err := h.repo.Create(term); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "创建词条失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "词条创建成功", "data": term})
}

// UpdateTerm POST /projects/:id/terms/:termId/update
func (h *TermHandler) UpdateTerm(c *gin.Context) {
	projectIDStr := c.Param("id")
	termIDStr := c.Param("termId")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	termID, err := strconv.Atoi(termIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的词条 ID"})
		return
	}

	var req struct {
		Module       string            `json:"module"`
		Key          string            `json:"key"`
		Description  string            `json:"description"`
		Status       models.TermStatus `json:"status"`
		Translations map[string]string `json:"translations"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("userID")

	term := &models.Term{
		ID:          uint(termID),
		ProjectID:   uint(projectID),
		Module:      req.Module,
		Key:         req.Key,
		Description: req.Description,
		Status:      req.Status,
	}

	if err := h.repo.Update(term, req.Translations, userID.(uint), "在详情面板手动更新了内容"); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "更新成功"})
}

// BatchUpdateTerms POST /projects/:id/terms/batch-update
func (h *TermHandler) BatchUpdateTerms(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	var req struct {
		Terms []repository.BatchUpdateTerm `json:"terms" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("userID")

	count, err := h.repo.BatchUpdate(uint(projectID), req.Terms, userID.(uint))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "批量更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "批量更新成功", "data": gin.H{"count": count}})
}

// PublishTerm POST /projects/:id/terms/:termId/publish
func (h *TermHandler) PublishTerm(c *gin.Context) {
	termIDStr := c.Param("termId")
	termID, err := strconv.Atoi(termIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的词条 ID"})
		return
	}
	userID, _ := c.Get("userID")
	if err := h.repo.Publish(uint(termID), userID.(uint)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "发布失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "发布成功"})
}

// BatchPublishTerms POST /projects/:id/terms/batch-publish
func (h *TermHandler) BatchPublishTerms(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	userID := uint(1) // FIXME: extract from JWT
	if err := h.repo.BatchPublish(req.IDs, userID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "批量发布失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "批量发布成功", "data": gin.H{"count": len(req.IDs)}})
}

// DeleteTerm POST /projects/:id/terms/:termId/delete
func (h *TermHandler) DeleteTerm(c *gin.Context) {
	termIDStr := c.Param("termId")
	termID, err := strconv.Atoi(termIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的词条 ID"})
		return
	}
	if err := h.repo.Delete(uint(termID)); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// BatchDeleteTerms POST /projects/:id/terms/batch-delete
func (h *TermHandler) BatchDeleteTerms(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if err := h.repo.BatchDelete(req.IDs); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "批量删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "批量删除成功", "data": gin.H{"count": len(req.IDs)}})
}

// ImportJSON POST /projects/:id/terms/import-json
func (h *TermHandler) ImportJSON(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	var req struct {
		LanguageCode string            `json:"language_code" binding:"required"`
		Data         map[string]string `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	created, updated, err := h.repo.ImportJSON(uint(projectID), req.LanguageCode, req.Data)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "导入失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "导入成功",
		"data": gin.H{
			"created": created,
			"updated": updated,
		},
	})
}

// GetProjectLogs GET /projects/:id/logs
func (h *TermHandler) GetProjectLogs(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	limit := 500 // Return up to 500 recent logs
	logs, err := h.repo.ProjectLogs(uint(projectID), limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取日志失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": logs})
}


