package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var languageCodePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

type AvailableLanguageHandler struct {
	repo *repository.AvailableLanguageRepository
}

func NewAvailableLanguageHandler(repo *repository.AvailableLanguageRepository) *AvailableLanguageHandler {
	return &AvailableLanguageHandler{repo: repo}
}

type availableLanguageInput struct {
	Code      string `json:"code" binding:"required"`
	Name      string `json:"name" binding:"required"`
	SortOrder int    `json:"sort_order"`
}

func normalizeAvailableLanguageInput(input availableLanguageInput) (availableLanguageInput, string) {
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	if !languageCodePattern.MatchString(input.Code) || utf8.RuneCountInString(input.Code) > 20 {
		return input, "语言代码必须以字母开头，只能包含小写字母、数字和连字符，且不超过 20 个字符"
	}
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 {
		return input, "语言名称不能为空且不能超过 100 个字符"
	}
	if input.SortOrder < 0 {
		return input, "显示顺序不能小于 0"
	}
	return input, ""
}

func (h *AvailableLanguageHandler) List(c *gin.Context) {
	languages, err := h.repo.List()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取可选语言失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": languages})
}

func (h *AvailableLanguageHandler) Create(c *gin.Context) {
	var input availableLanguageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请填写语言代码和名称"})
		return
	}
	input, validationMessage := normalizeAvailableLanguageInput(input)
	if validationMessage != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": validationMessage})
		return
	}

	language := models.AvailableLanguage{Code: input.Code, Name: input.Name, SortOrder: input.SortOrder}
	if err := h.repo.Create(&language); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			c.JSON(http.StatusOK, gin.H{"code": 409, "message": "该语言代码已经存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "创建可选语言失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "可选语言已创建", "data": language})
}

func (h *AvailableLanguageHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的语言 ID"})
		return
	}
	var input availableLanguageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请填写语言代码和名称"})
		return
	}
	input, validationMessage := normalizeAvailableLanguageInput(input)
	if validationMessage != "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": validationMessage})
		return
	}

	language, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "可选语言不存在"})
		return
	}
	previousCode := language.Code
	if previousCode != input.Code {
		usageCount, countErr := h.repo.CountUsage(language.Code)
		if countErr != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "检查语言使用情况失败"})
			return
		}
		if usageCount > 0 {
			c.JSON(http.StatusOK, gin.H{"code": 409, "message": "该语言已被项目使用，不能修改语言代码"})
			return
		}
	}
	language.Code = input.Code
	language.Name = input.Name
	language.SortOrder = input.SortOrder
	if err := h.repo.Update(language, previousCode); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			c.JSON(http.StatusOK, gin.H{"code": 409, "message": "该语言代码已经存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "更新可选语言失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "可选语言已更新", "data": language})
}

func (h *AvailableLanguageHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的语言 ID"})
		return
	}
	if err := h.repo.Delete(uint(id)); err != nil {
		if errors.Is(err, repository.ErrAvailableLanguageInUse) {
			c.JSON(http.StatusOK, gin.H{"code": 409, "message": "该语言正在被项目使用，不能删除"})
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"code": 404, "message": "可选语言不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "删除可选语言失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "可选语言已删除"})
}
