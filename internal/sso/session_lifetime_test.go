package sso

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"choice-matrix-backend/internal/config"
	"github.com/gin-gonic/gin"
)

func TestCSRFCookieMatchesThirtyDaySession(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/csrf", nil)
	New(nil, config.Config{CookieSecure: true}).CSRF(c)
	cookies := w.Result().Cookies()
	if w.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Name != CSRFCookie {
		t.Fatal("expected successful CSRF response with cookie")
	}
	cookie := cookies[0]
	if cookie.MaxAge != 30*24*60*60 || !cookie.Secure || cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected CSRF cookie policy: max-age=%d secure=%t httponly=%t samesite=%v", cookie.MaxAge, cookie.Secure, cookie.HttpOnly, cookie.SameSite)
	}
}
