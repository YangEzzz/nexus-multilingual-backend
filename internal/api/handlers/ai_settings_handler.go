package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/repository"
	"choice-matrix-backend/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AISettingsHandler struct {
	repo  *repository.AICredentialRepository
	relay *services.AIRelayService
}

func NewAISettingsHandler(repo *repository.AICredentialRepository, relay *services.AIRelayService) *AISettingsHandler {
	return &AISettingsHandler{repo: repo, relay: relay}
}

type aiSettingsInput struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model" binding:"required"`
}

type aiTranslateInput struct {
	Prompt          string   `json:"prompt" binding:"required"`
	TargetLanguages []string `json:"target_languages" binding:"required"`
	PlaceholderRule string   `json:"placeholder_rule"`
}

func (h *AISettingsHandler) Settings(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	credential, err := h.repo.FindByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": gin.H{
			"configured":     false,
			"model":          "",
			"relay_base_url": h.relay.BaseURL(),
		}})
		return
	}
	if err != nil {
		log.Printf("get AI settings failed for user %d: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "获取 AI 翻译设置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": aiSettingsOutput(credential, h.relay.BaseURL())})
}

func (h *AISettingsHandler) Save(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var input aiSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请填写模型名称"})
		return
	}
	input.Model = strings.TrimSpace(input.Model)
	input.APIKey = strings.TrimSpace(input.APIKey)
	if input.Model == "" || utf8.RuneCountInString(input.Model) > 255 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模型名称不能为空且不能超过 255 个字符"})
		return
	}

	existing, err := h.repo.FindByUserID(userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("find AI settings failed for user %d: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存 AI 翻译设置失败"})
		return
	}
	if input.APIKey == "" && errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "首次配置时请输入 API Key"})
		return
	}

	credential := &models.AICredential{UserID: userID, Model: input.Model}
	if existing != nil {
		credential.EncryptedKey = existing.EncryptedKey
		credential.KeyLastFour = existing.KeyLastFour
	}
	if input.APIKey != "" {
		encryptedKey, encryptErr := h.relay.EncryptKey(input.APIKey)
		if encryptErr != nil {
			log.Printf("encrypt AI key failed for user %d: %v", userID, encryptErr)
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存 AI Key 失败"})
			return
		}
		credential.EncryptedKey = encryptedKey
		credential.KeyLastFour = lastFour(input.APIKey)
	}
	if err := h.repo.Save(credential); err != nil {
		log.Printf("save AI settings failed for user %d: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存 AI 翻译设置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "AI 翻译设置已保存", "data": aiSettingsOutput(credential, h.relay.BaseURL())})
}

func (h *AISettingsHandler) Delete(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	if err := h.repo.DeleteByUserID(userID); err != nil {
		log.Printf("delete AI settings failed for user %d: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "删除 AI 翻译设置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "AI 翻译设置已删除"})
}

func (h *AISettingsHandler) Translate(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var input aiTranslateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请提供待翻译内容和目标语言"})
		return
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Prompt == "" || len(input.TargetLanguages) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请提供待翻译内容和目标语言"})
		return
	}

	credential, err := h.repo.FindByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请先在 AI 翻译设置中保存 API Key 和模型"})
		return
	}
	if err != nil {
		log.Printf("find AI credential for translation failed for user %d: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取 AI 翻译设置失败"})
		return
	}
	apiKey, err := h.relay.DecryptKey(credential.EncryptedKey)
	if err != nil {
		log.Printf("decrypt AI credential failed for user %d: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存的 AI Key 无法读取，请重新设置"})
		return
	}
	translations, err := h.relay.Translate(services.TranslationRequest{
		Model:           credential.Model,
		APIKey:          apiKey,
		Prompt:          input.Prompt,
		TargetLanguages: input.TargetLanguages,
		PlaceholderRule: input.PlaceholderRule,
	})
	if err != nil {
		log.Printf("AI relay translation failed for user %d: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 502, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": gin.H{
		"translations":      translations,
		"model":             credential.Model,
		"credential_source": "personal",
	}})
}

func currentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return 0, false
	}
	userID, ok := value.(uint)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return 0, false
	}
	return userID, true
}

func lastFour(value string) string {
	runes := []rune(value)
	if len(runes) <= 4 {
		return string(runes)
	}
	return string(runes[len(runes)-4:])
}

func aiSettingsOutput(credential *models.AICredential, relayBaseURL string) gin.H {
	return gin.H{
		"configured":     true,
		"model":          credential.Model,
		"masked_key":     "••••" + credential.KeyLastFour,
		"key_last_four":  credential.KeyLastFour,
		"updated_at":     credential.UpdatedAt,
		"relay_base_url": relayBaseURL,
	}
}
