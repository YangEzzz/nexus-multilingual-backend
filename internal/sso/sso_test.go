package sso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"choice-matrix-backend/internal/config"
	"github.com/gin-gonic/gin"
)

func TestReturnPath(t *testing.T) {
	for _, raw := range []string{"https://evil.example", "//evil.example", "/\\evil.example", "/auth/sign-in", "/auth/sso/complete", "/\r\nevil"} {
		if got := ReturnPath(raw); got != "/dashboard" {
			t.Fatalf("unsafe redirect %q => %q", raw, got)
		}
	}
	if got := ReturnPath("/projects?tab=terms"); got != "/projects?tab=terms" {
		t.Fatalf("lost local path: %s", got)
	}
}
func TestCSRFOriginAndToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := New(nil, config.Config{FrontendURL: "https://nexus.example"})
	r := gin.New()
	r.POST("/write", func(c *gin.Context) {
		if s.guard(c) {
			c.Status(204)
		}
	})
	for _, tc := range []struct {
		origin, token string
		want          int
	}{{"https://nexus.example", "abc", 204}, {"https://evil.example", "abc", 403}, {"https://nexus.example", "wrong", 403}, {"", "abc", 403}} {
		req := httptest.NewRequest("POST", "/write", nil)
		req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "abc"})
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("X-CSRF-Token", tc.token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("CSRF %q %q: %d", tc.origin, tc.token, w.Code)
		}
	}
}
func TestRemoteIdentityFailClosed(t *testing.T) {
	tests := []struct {
		body    string
		status  int
		allowed bool
	}{
		{`{"code":200,"data":{"user":{"id":"user"},"project_id_string":"nexus","role_in_project":"translator"}}`, 200, true},
		{`{"code":200,"data":{"user":{"id":"user"},"project_id_string":"other","role_in_project":"translator"}}`, 200, false},
		{`{"code":200,"data":{"user":{"id":"user"},"project_id_string":"nexus","role_in_project":"super_admin"}}`, 200, false},
		{`{"code":500,"data":{}}`, 200, false}, {`{}`, 503, false},
	}
	for _, tc := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, secret, _ := r.BasicAuth()
			if id != "client" || secret != "secret" {
				t.Error("missing backend authentication")
			}
			w.WriteHeader(tc.status)
			w.Write([]byte(tc.body))
		}))
		s := New(nil, config.Config{HeliosInternalURL: srv.URL, HeliosClientID: "client", HeliosClientSecret: "secret", HeliosProjectID: "nexus", HeliosAllowedRoles: []string{"translator"}})
		_, err := s.remote(context.Background(), "/identity", map[string]string{"user_id": "user"})
		if (err == nil) != tc.allowed {
			t.Errorf("identity result: %v", err)
		}
		srv.Close()
	}
}
