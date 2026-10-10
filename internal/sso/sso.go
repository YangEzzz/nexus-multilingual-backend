package sso

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"choice-matrix-backend/internal/config"
	"choice-matrix-backend/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const SessionCookie = "nexus_session"
const CSRFCookie = "nexus_csrf"
const BrowserCookie = "nexus_browser"

type LoginTransaction struct {
	ID          string `gorm:"primaryKey;size:64"`
	BrowserHash string `gorm:"size:64;not null"`
	Verifier    string `gorm:"size:128;not null"`
	ReturnPath  string `gorm:"not null"`
	Mode        string `gorm:"size:16;not null"`
	Status      string `gorm:"size:16;not null"`
	Message     string
	ExpiresAt   time.Time `gorm:"index;not null"`
	CreatedAt   time.Time
}

func (LoginTransaction) TableName() string { return "sso_login_transactions" }

type Session struct {
	ID         string `gorm:"primaryKey;size:64"`
	UserID     uint   `gorm:"not null;index"`
	ExternalID string `gorm:"size:100;not null"`
	VerifiedAt time.Time
	ExpiresAt  time.Time `gorm:"index;not null"`
	CreatedAt  time.Time
}

func (Session) TableName() string { return "auth_sessions" }

type Service struct {
	DB   *gorm.DB
	Cfg  config.Config
	HTTP *http.Client
}

func New(db *gorm.DB, cfg config.Config) *Service {
	return &Service{db, cfg, &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func random() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func hash(v string) string { b := sha256.Sum256([]byte(v)); return hex.EncodeToString(b[:]) }
func challenge(v string) string {
	b := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func equal(a, b string) bool { return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "message": msg})
}
func success(c *gin.Context, data any) {
	c.JSON(200, gin.H{"code": 200, "data": data, "message": "操作成功"})
}
func (s *Service) cookie(c *gin.Context, name, value string, age int, httpOnly bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, age, "/", "", s.Cfg.CookieSecure, httpOnly)
}
func (s *Service) origin() string {
	u, _ := url.Parse(s.Cfg.FrontendURL)
	if u == nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
func (s *Service) guard(c *gin.Context) bool {
	csrf, _ := c.Cookie(CSRFCookie)
	if c.GetHeader("Origin") != s.origin() || !equal(csrf, c.GetHeader("X-CSRF-Token")) {
		c.AbortWithStatusJSON(403, gin.H{"code": 403, "message": "请求校验失败，请刷新页面重试"})
		return false
	}
	return true
}
func ReturnPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\\r\n") || u.IsAbs() || u.Host != "" || strings.HasPrefix(u.Path, "/auth/") {
		return "/dashboard"
	}
	return raw
}
func (s *Service) Register(auth *gin.RouterGroup) {
	auth.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	})
	auth.GET("/csrf", s.CSRF)
	auth.POST("/sso/start", s.Start)
	auth.GET("/sso/callback", s.Callback)
	auth.GET("/sso/status", s.Status)
	auth.POST("/sso/cancel", s.Cancel)
	auth.POST("/logout", s.Logout)
	auth.POST("/login", func(c *gin.Context) { fail(c, 410, "请使用 Helios 统一登录") })
}
func (s *Service) CSRF(c *gin.Context) {
	token, _ := c.Cookie(CSRFCookie)
	if len(token) != 43 {
		var err error
		token, err = random()
		if err != nil {
			fail(c, 500, "无法初始化登录")
			return
		}
		s.cookie(c, CSRFCookie, token, 8*3600, false)
	}
	success(c, gin.H{"csrf_token": token})
}
func (s *Service) Start(c *gin.Context) {
	if !s.guard(c) {
		return
	}
	if s.Cfg.HeliosClientID == "" || s.Cfg.HeliosClientSecret == "" {
		fail(c, 503, "统一登录尚未配置，请联系管理员")
		return
	}
	var in struct {
		ReturnPath string `json:"return_path"`
		Mode       string `json:"mode"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.ReturnPath) > 2048 {
		fail(c, 400, "请求无效")
		return
	}
	if in.Mode != "popup" && in.Mode != "redirect" {
		fail(c, 400, "登录方式无效")
		return
	}
	browser, _ := c.Cookie(BrowserCookie)
	if len(browser) != 43 {
		var err error
		browser, err = random()
		if err != nil {
			fail(c, 500, "无法初始化登录")
			return
		}
		s.cookie(c, BrowserCookie, browser, 8*3600, true)
	}
	state, err := random()
	if err != nil {
		fail(c, 500, "无法初始化登录")
		return
	}
	verifier, err := random()
	if err != nil {
		fail(c, 500, "无法初始化登录")
		return
	}
	t := LoginTransaction{ID: hash(state), BrowserHash: hash(browser), Verifier: verifier, ReturnPath: ReturnPath(in.ReturnPath), Mode: in.Mode, Status: "pending", ExpiresAt: time.Now().Add(10 * time.Minute)}
	if s.DB.Create(&t).Error != nil {
		fail(c, 500, "无法初始化登录")
		return
	}
	target, err := url.Parse(strings.TrimRight(s.Cfg.HeliosBaseURL, "/") + "/api/sso/authorize")
	if err != nil {
		fail(c, 503, "统一登录配置无效")
		return
	}
	q := target.Query()
	q.Set("client_id", s.Cfg.HeliosClientID)
	q.Set("redirect_uri", s.Cfg.HeliosRedirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge(verifier))
	q.Set("code_challenge_method", "S256")
	target.RawQuery = q.Encode()
	success(c, gin.H{"authorize_url": target.String(), "transaction_id": state})
}
func (s *Service) transaction(c *gin.Context, state string) (*LoginTransaction, error) {
	if len(state) != 43 {
		return nil, errors.New("invalid state")
	}
	browser, _ := c.Cookie(BrowserCookie)
	var t LoginTransaction
	err := s.DB.First(&t, "id = ? AND browser_hash = ? AND expires_at > ?", hash(state), hash(browser), time.Now()).Error
	return &t, err
}

type Identity struct {
	User struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	} `json:"user"`
	ProjectID string `json:"project_id_string"`
	Role      string `json:"role_in_project"`
}

var ErrDenied = errors.New("identity denied")

func (s *Service) remote(ctx context.Context, path string, data any) (*Identity, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(s.Cfg.HeliosInternalURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(s.Cfg.HeliosClientID, s.Cfg.HeliosClientSecret)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, ErrDenied
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("helios unavailable")
	}
	var result struct {
		Code int      `json:"code"`
		Data Identity `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result); err != nil {
		return nil, err
	}
	if result.Code != 200 || result.Data.User.ID == "" || result.Data.ProjectID != s.Cfg.HeliosProjectID || !s.allowedRole(result.Data.Role) {
		return nil, ErrDenied
	}
	return &result.Data, nil
}
func (s *Service) allowedRole(role string) bool {
	for _, r := range s.Cfg.HeliosAllowedRoles {
		if r == role {
			return true
		}
	}
	return false
}
func syncUser(tx *gorm.DB, info *Identity) (*models.User, error) {
	u := models.User{ExternalID: info.User.ID, Nickname: info.User.Name, AvatarURL: info.User.Avatar, Role: info.Role}
	err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "external_id"}}, DoUpdates: clause.AssignmentColumns([]string{"nickname", "avatar_url", "role", "updated_at"})}).Create(&u).Error
	if err != nil {
		return nil, err
	}
	if err = tx.First(&u, "external_id = ?", info.User.ID).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
func (s *Service) complete(c *gin.Context, state string) {
	c.Redirect(303, strings.TrimRight(s.Cfg.FrontendURL, "/")+"/auth/sso/complete?transaction="+url.QueryEscape(state))
}
func (s *Service) Callback(c *gin.Context) {
	state := c.Query("state")
	t, err := s.transaction(c, state)
	if err != nil {
		c.Redirect(303, strings.TrimRight(s.Cfg.FrontendURL, "/")+"/auth/sso/complete?error=invalid_request")
		return
	}
	result := s.DB.Model(&LoginTransaction{}).Where("id = ? AND status = ?", t.ID, "pending").Update("status", "processing")
	if result.Error != nil || result.RowsAffected != 1 {
		s.complete(c, state)
		return
	}
	mark := func(status, msg string) {
		s.DB.Model(&LoginTransaction{}).Where("id = ?", t.ID).Updates(map[string]any{"status": status, "message": msg, "verifier": ""})
	}
	if c.Query("error") != "" {
		mark("cancelled", "已取消登录")
		s.complete(c, state)
		return
	}
	info, err := s.remote(c.Request.Context(), "/api/sso/exchange", gin.H{"code": c.Query("code"), "redirect_uri": s.Cfg.HeliosRedirectURI, "code_verifier": t.Verifier})
	if err != nil {
		mark("failed", "登录未完成，授权已失效或认证服务不可用，请重新登录")
		s.complete(c, state)
		return
	}
	token, err := random()
	if err != nil {
		mark("failed", "无法建立会话，请重试")
		s.complete(c, state)
		return
	}
	old, _ := c.Cookie(SessionCookie)
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		u, err := syncUser(tx, info)
		if err != nil {
			return err
		}
		if old != "" {
			if err = tx.Delete(&Session{}, "id = ?", hash(old)).Error; err != nil {
				return err
			}
		}
		if err = tx.Create(&Session{ID: hash(token), UserID: u.ID, ExternalID: u.ExternalID, VerifiedAt: time.Now(), ExpiresAt: time.Now().Add(8 * time.Hour)}).Error; err != nil {
			return err
		}
		result := tx.Model(&LoginTransaction{}).Where("id = ? AND status = ?", t.ID, "processing").Updates(map[string]any{"status": "success", "verifier": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("cancelled transaction")
		}
		return nil
	})
	if err != nil {
		mark("failed", "无法保存登录信息，请重试")
		s.complete(c, state)
		return
	}
	s.cookie(c, SessionCookie, token, 8*3600, true)
	s.complete(c, state)
}
func (s *Service) Status(c *gin.Context) {
	t, err := s.transaction(c, c.Query("transaction"))
	if err != nil {
		fail(c, 400, "登录请求已失效，请重新登录")
		return
	}
	success(c, gin.H{"status": t.Status, "message": t.Message, "return_path": t.ReturnPath, "mode": t.Mode})
}
func (s *Service) Cancel(c *gin.Context) {
	if !s.guard(c) {
		return
	}
	var in struct {
		Transaction string `json:"transaction"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "请求无效")
		return
	}
	t, err := s.transaction(c, in.Transaction)
	if err != nil {
		fail(c, 400, "登录请求已失效")
		return
	}
	if s.DB.Model(&LoginTransaction{}).Where("id = ? AND status = ?", t.ID, "pending").Updates(map[string]any{"status": "cancelled", "message": "已取消登录", "verifier": ""}).Error != nil {
		fail(c, 500, "取消失败，请重试")
		return
	}
	success(c, gin.H{})
}
func (s *Service) Logout(c *gin.Context) {
	if !s.guard(c) {
		return
	}
	token, _ := c.Cookie(SessionCookie)
	if token != "" {
		if s.DB.Delete(&Session{}, "id = ?", hash(token)).Error != nil {
			fail(c, 500, "退出失败，请重试")
			return
		}
	}
	s.cookie(c, SessionCookie, "", -1, true)
	success(c, gin.H{})
}
func (s *Service) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		token, _ := c.Cookie(SessionCookie)
		var sess Session
		if token == "" || s.DB.First(&sess, "id = ? AND expires_at > ?", hash(token), time.Now()).Error != nil {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "登录已失效，请重新登录"})
			return
		}
		write := c.Request.Method != "GET" && c.Request.Method != "HEAD"
		if write && !s.guard(c) {
			return
		}
		if write || time.Since(sess.VerifiedAt) >= 5*time.Minute {
			info, err := s.remote(c.Request.Context(), "/api/sso/identity", gin.H{"user_id": sess.ExternalID})
			if errors.Is(err, ErrDenied) {
				s.DB.Delete(&Session{}, "id = ?", sess.ID)
				s.cookie(c, SessionCookie, "", -1, true)
				c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "账号或项目权限已失效，请重新登录"})
				return
			}
			if err != nil {
				c.AbortWithStatusJSON(503, gin.H{"code": 503, "message": "暂时无法校验访问权限，请稍后重试"})
				return
			}
			err = s.DB.Transaction(func(tx *gorm.DB) error {
				if _, err := syncUser(tx, info); err != nil {
					return err
				}
				return tx.Model(&sess).Update("verified_at", time.Now()).Error
			})
			if err != nil {
				c.AbortWithStatusJSON(503, gin.H{"code": 503, "message": "暂时无法更新访问权限"})
				return
			}
		}
		c.Set("userID", sess.UserID)
		c.Next()
	}
}
