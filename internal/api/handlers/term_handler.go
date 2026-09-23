package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/repository"
	"choice-matrix-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

type TermHandler struct {
	repo *repository.TermRepository
}

type termBulkRequest struct {
	Operation repository.BulkOperation `json:"operation" binding:"required"`
	Scope     repository.TermBulkScope `json:"scope" binding:"required"`
}

type termExportRequest struct {
	Scope     repository.TermBulkScope `json:"scope" binding:"required"`
	Languages []string                 `json:"languages"`
}

type termCopyRequest struct {
	Scope    repository.TermBulkScope `json:"scope" binding:"required"`
	Language string                   `json:"language" binding:"required"`
}

func NewTermHandler(repo *repository.TermRepository) *TermHandler {
	return &TermHandler{repo: repo}
}

func bindTermBulkRequest(c *gin.Context) (uint, termBulkRequest, bool) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return 0, termBulkRequest{}, false
	}

	var req termBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return 0, termBulkRequest{}, false
	}
	if !repository.IsValidBulkOperation(req.Operation) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "不支持的批量操作"})
		return 0, termBulkRequest{}, false
	}
	if req.Scope.Mode != "explicit" && req.Scope.Mode != "filtered" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "不支持的选择范围"})
		return 0, termBulkRequest{}, false
	}
	if req.Scope.Mode == "explicit" && len(req.Scope.IDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请至少选择一个词条"})
		return 0, termBulkRequest{}, false
	}
	return uint(projectID), req, true
}

func validateTermScope(c *gin.Context, scope repository.TermBulkScope) bool {
	if scope.Mode != "explicit" && scope.Mode != "filtered" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "不支持的选择范围"})
		return false
	}
	if scope.Mode == "explicit" && len(scope.IDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请至少选择一个词条"})
		return false
	}
	return true
}

func exportLanguageCodes(projectLanguages []models.Language, requested []string) ([]string, bool) {
	allowed := make(map[string]bool, len(projectLanguages))
	for _, language := range projectLanguages {
		allowed[language.Code] = true
	}
	if len(requested) == 0 {
		codes := make([]string, 0, len(projectLanguages))
		for _, language := range projectLanguages {
			codes = append(codes, language.Code)
		}
		return codes, true
	}
	seen := make(map[string]bool, len(requested))
	codes := make([]string, 0, len(requested))
	for _, code := range requested {
		if !allowed[code] {
			return nil, false
		}
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	return codes, true
}

// List GET /projects/:id/terms
func (h *TermHandler) List(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.repo.ListByProjectPage(uint(projectID), repository.TermListFilter{
		Page:          page,
		PageSize:      pageSize,
		Search:        c.Query("search"),
		ChineseSearch: c.Query("chinese_search"),
		EnglishSearch: c.Query("english_search"),
		Module:        c.Query("module"),
		Status:        c.Query("status"),
		ExcludeDraft:  c.Query("exclude_draft") == "true",
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取词条失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": result})
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

// BatchCreateTerms POST /projects/:id/terms/batch-create
func (h *TermHandler) BatchCreateTerms(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}

	var req struct {
		Terms []repository.BatchCreateTerm `json:"terms" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if len(req.Terms) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请至少提交一个词条"})
		return
	}
	if len(req.Terms) > 500 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "单次最多创建 500 个词条"})
		return
	}

	type validationError struct {
		ClientID string `json:"clientId"`
		Field    string `json:"field"`
		Reason   string `json:"reason"`
		Message  string `json:"message"`
	}
	validationErrors := make([]validationError, 0)
	seen := make(map[string]string, len(req.Terms))

	for i := range req.Terms {
		term := &req.Terms[i]
		term.ClientID = strings.TrimSpace(term.ClientID)
		term.Module = strings.TrimSpace(term.Module)
		term.Key = strings.TrimSpace(term.Key)
		term.Description = strings.TrimSpace(term.Description)
		if term.ClientID == "" {
			term.ClientID = "row-" + strconv.Itoa(i+1)
		}

		if term.Key == "" {
			validationErrors = append(validationErrors, validationError{term.ClientID, "key", "required", "请填写 Key"})
		} else if utf8.RuneCountInString(term.Key) > 255 {
			validationErrors = append(validationErrors, validationError{term.ClientID, "key", "too_long", "Key 不能超过 255 个字符"})
		}
		if utf8.RuneCountInString(term.Module) > 100 {
			validationErrors = append(validationErrors, validationError{term.ClientID, "module", "too_long", "模块不能超过 100 个字符"})
		}

		identity := term.Module + "\x00" + term.Key
		if firstClientID, exists := seen[identity]; term.Key != "" && exists {
			validationErrors = append(validationErrors, validationError{
				term.ClientID,
				"key",
				"duplicate_in_batch",
				"与本批次中的词条 " + firstClientID + " 重复",
			})
		} else if term.Key != "" {
			seen[identity] = term.ClientID
		}
	}

	if len(validationErrors) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "请修正批量创建数据",
			"data":    gin.H{"errors": validationErrors},
		})
		return
	}

	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 401, "message": "用户未登录"})
		return
	}

	created, err := h.repo.BatchCreate(uint(projectID), req.Terms, userID.(uint))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 409, "message": "批量创建失败，请检查 Key、模块和项目语言配置后重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "批量创建成功",
		"data": gin.H{
			"createdCount": len(created),
			"terms":        created,
		},
	})
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

// PreviewBulkTerms POST /projects/:id/terms/bulk/preview
func (h *TermHandler) PreviewBulkTerms(c *gin.Context) {
	projectID, req, ok := bindTermBulkRequest(c)
	if !ok {
		return
	}
	result, err := h.repo.PreviewBulk(projectID, req.Operation, req.Scope)
	if err != nil {
		log.Printf("bulk preview failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "批量操作预检查失败，请刷新后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "预检查完成", "data": result})
}

// ExecuteBulkTerms POST /projects/:id/terms/bulk/execute
func (h *TermHandler) ExecuteBulkTerms(c *gin.Context) {
	projectID, req, ok := bindTermBulkRequest(c)
	if !ok {
		return
	}
	userIDValue, exists := c.Get("userID")
	userID, validUserID := userIDValue.(uint)
	if !exists || !validUserID {
		c.JSON(http.StatusOK, gin.H{"code": 401, "message": "登录状态已失效"})
		return
	}
	result, err := h.repo.ExecuteBulk(projectID, userID, req.Operation, req.Scope)
	if err != nil {
		log.Printf("bulk execution failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "批量操作未完成，请重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "批量操作完成", "data": result})
}

// ExportTermsJSON POST /projects/:id/terms/export/json
func (h *TermHandler) ExportTermsJSON(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	var req termExportRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validateTermScope(c, req.Scope) {
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误"})
		}
		return
	}
	terms, err := h.repo.TermsForScope(uint(projectID), req.Scope)
	if err != nil {
		log.Printf("JSON export scope failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取导出数据失败"})
		return
	}
	languages, err := h.repo.ProjectLanguages(uint(projectID))
	if err != nil {
		log.Printf("JSON export languages failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取项目语言失败"})
		return
	}
	codes, valid := exportLanguageCodes(languages, req.Languages)
	if !valid || len(codes) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "导出语言未配置在当前项目中"})
		return
	}
	content, err := utils.BuildLanguageJSONArchive(terms, codes)
	if err != nil {
		log.Printf("JSON archive failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成 JSON 压缩包失败"})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="translations.zip"`)
	c.Data(http.StatusOK, "application/zip", content)
}

// ExportTermsExcel POST /projects/:id/terms/export/excel
func (h *TermHandler) ExportTermsExcel(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	var req termExportRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validateTermScope(c, req.Scope) {
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误"})
		}
		return
	}
	terms, err := h.repo.TermsForScope(uint(projectID), req.Scope)
	if err != nil {
		log.Printf("Excel export scope failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取导出数据失败"})
		return
	}
	languages, err := h.repo.ProjectLanguages(uint(projectID))
	if err != nil {
		log.Printf("Excel export languages failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取项目语言失败"})
		return
	}
	content, err := utils.BuildTermXLSX(terms, languages)
	if err != nil {
		log.Printf("Excel generation failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成 Excel 文件失败"})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="translations.xlsx"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

// CopyTermsJSON POST /projects/:id/terms/export/copy
func (h *TermHandler) CopyTermsJSON(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil || projectID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	var req termCopyRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validateTermScope(c, req.Scope) {
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误"})
		}
		return
	}
	languages, err := h.repo.ProjectLanguages(uint(projectID))
	if err != nil {
		log.Printf("copy languages failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取项目语言失败"})
		return
	}
	codes, valid := exportLanguageCodes(languages, []string{req.Language})
	if !valid || len(codes) != 1 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "复制语言未配置在当前项目中"})
		return
	}
	terms, err := h.repo.TermsForScope(uint(projectID), req.Scope)
	if err != nil {
		log.Printf("copy scope failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取复制数据失败"})
		return
	}
	if len(terms) > 1000 {
		c.JSON(http.StatusOK, gin.H{"code": 413, "message": "复制最多支持 1000 条，请改用 JSON 导出"})
		return
	}
	content, err := utils.BuildLanguageJSON(terms, req.Language)
	if err != nil {
		log.Printf("copy JSON generation failed for project %d: %v", projectID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成复制内容失败"})
		return
	}
	if len(content) > 1024*1024 {
		c.JSON(http.StatusOK, gin.H{"code": 413, "message": "复制内容超过 1MB，请改用 JSON 导出"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "复制内容已生成", "data": gin.H{
		"content": string(content),
		"count":   len(terms),
	}})
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
		LanguageCode string                      `json:"language_code" binding:"required"`
		Data         map[string]string           `json:"data"`
		Items        []repository.ImportJSONItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	var created, updated int
	if len(req.Items) > 0 {
		created, updated, err = h.repo.ImportJSONItems(uint(projectID), req.LanguageCode, req.Items)
	} else if len(req.Data) > 0 {
		created, updated, err = h.repo.ImportJSON(uint(projectID), req.LanguageCode, req.Data)
	} else {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "导入内容不能为空"})
		return
	}
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
