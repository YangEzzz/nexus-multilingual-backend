package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"choice-matrix-backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LocaleReleaseHandler struct {
	repo *repository.LocaleReleaseRepository
}

func NewLocaleReleaseHandler(repo *repository.LocaleReleaseRepository) *LocaleReleaseHandler {
	return &LocaleReleaseHandler{repo: repo}
}

func (h *LocaleReleaseHandler) Latest(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || projectID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	release, err := h.repo.Latest(uint(projectID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "读取发布版本失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": release})
}

func (h *LocaleReleaseHandler) Publish(c *gin.Context) {
	projectID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || projectID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的项目 ID"})
		return
	}
	userID := c.MustGet("userID").(uint)
	release, err := h.repo.Publish(uint(projectID), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "发布语言文件失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "语言文件已发布", "data": release})
}

func (h *LocaleReleaseHandler) PublicFile(c *gin.Context) {
	projectCode := c.Param("projectCode")
	requestedLanguage := c.Param("languageCode")
	languageCode := strings.TrimSuffix(requestedLanguage, ".json")
	if projectCode == "" || languageCode == "" || requestedLanguage == languageCode {
		c.Status(http.StatusNotFound)
		return
	}
	file, err := h.repo.PublicFile(projectCode, languageCode)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	etag := `"` + file.SHA256 + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, no-cache")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(file.Content))
}
