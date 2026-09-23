package services

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultAIRelayBaseURL = "https://43.172.5.210:3000/v1"

type AIRelayService struct {
	baseURL string
	aead    cipher.AEAD
	client  *http.Client
}

type TranslationRequest struct {
	Model           string
	APIKey          string
	Prompt          string
	TargetLanguages []string
	PlaceholderRule string
}

type relayRequest struct {
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	Messages    []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type relayResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewAIRelayService(encryptionKey, fallbackSecret, baseURL string) (*AIRelayService, error) {
	secret := strings.TrimSpace(encryptionKey)
	if secret == "" {
		secret = strings.TrimSpace(fallbackSecret)
	}
	if secret == "" {
		return nil, errors.New("AI Key encryption secret is empty")
	}

	key := sha256.Sum256([]byte("nexus-ai-relay-credential-v1:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create AI credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AI credential cipher mode: %w", err)
	}

	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultAIRelayBaseURL
	}
	return &AIRelayService{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		aead:    aead,
		client:  &http.Client{Timeout: 90 * time.Second},
	}, nil
}

func (s *AIRelayService) BaseURL() string {
	return s.baseURL
}

func (s *AIRelayService) EncryptKey(apiKey string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", errors.New("API Key is empty")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate credential nonce: %w", err)
	}
	ciphertext := s.aead.Seal(nil, nonce, []byte(apiKey), nil)
	return base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (s *AIRelayService) DecryptKey(encrypted string) (string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encrypted)
	if err != nil || len(payload) < s.aead.NonceSize() {
		return "", errors.New("stored AI Key cannot be decrypted")
	}
	nonce, ciphertext := payload[:s.aead.NonceSize()], payload[s.aead.NonceSize():]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("stored AI Key cannot be decrypted")
	}
	return string(plaintext), nil
}

func (s *AIRelayService) Translate(input TranslationRequest) (map[string]string, error) {
	if strings.TrimSpace(input.Model) == "" || strings.TrimSpace(input.APIKey) == "" {
		return nil, errors.New("AI 配置不完整，请先设置 Key 和模型")
	}

	languages := make([]string, 0, len(input.TargetLanguages))
	for _, language := range input.TargetLanguages {
		if trimmed := strings.TrimSpace(language); trimmed != "" {
			languages = append(languages, trimmed)
		}
	}
	if len(languages) == 0 {
		return nil, errors.New("至少需要一个目标语言")
	}

	requestBody := relayRequest{Model: input.Model, Temperature: 0.2}
	requestBody.Messages = append(requestBody.Messages,
		struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			Role:    "system",
			Content: fmt.Sprintf("你是专业软件本地化翻译引擎。将用户提供的内容翻译成目标语言：%s。只返回一个 JSON 对象，键必须是语言代码，值必须是译文字符串。不要包含 Markdown、解释或额外字段。必须原样保留占位符。占位符规则：%s", strings.Join(languages, ", "), strings.TrimSpace(input.PlaceholderRule)),
		},
		struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "user", Content: input.Prompt},
	)

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("serialize relay request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create relay request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+input.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求中转站失败：%w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取中转站响应失败：%w", err)
	}

	var decoded relayResponse
	_ = json.Unmarshal(responseBody, &decoded)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := decoded.Error.Message
		if message == "" {
			message = strings.TrimSpace(string(responseBody))
		}
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("中转站返回 %d：%s", resp.StatusCode, sanitizeRelayMessage(message, input.APIKey))
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return nil, errors.New("中转站未返回可用的翻译内容")
	}

	translations, err := parseTranslations(decoded.Choices[0].Message.Content, languages)
	if err != nil {
		return nil, err
	}
	return translations, nil
}

func parseTranslations(content string, languages []string) (map[string]string, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(strings.TrimSpace(content), "```")

	var raw map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &raw); err != nil {
		return nil, errors.New("中转站返回的翻译格式无效，请重试")
	}
	translations := make(map[string]string, len(languages))
	for _, language := range languages {
		value, exists := raw[language]
		if !exists {
			translations[language] = ""
			continue
		}
		translation, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("中转站返回的 %s 翻译不是文本", language)
		}
		translations[language] = translation
	}
	return translations, nil
}

func sanitizeRelayMessage(message, apiKey string) string {
	message = strings.ReplaceAll(message, apiKey, "[REDACTED]")
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 500 {
		return message[:500] + "…"
	}
	return message
}
