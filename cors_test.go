package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"choice-matrix-backend/internal/config"
	"github.com/gin-gonic/gin"
)

func TestAPIOriginCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(browserCORS(config.Config{FrontendURL: "https://nexus.team-tool.top"}))
	r.GET("/api/session", func(c *gin.Context) { c.Status(200) })
	r.POST("/api/write", func(c *gin.Context) { c.Status(200) })
	for _, origin := range []string{"https://nexus.team-tool.top", "https://evil.example"} {
		for _, method := range []string{"OPTIONS", "GET", "POST"} {
			req := httptest.NewRequest(method, "https://nexus-api.team-tool.top/api/session", nil)
			if method == "POST" {
				req = httptest.NewRequest(method, "https://nexus-api.team-tool.top/api/write", nil)
			}
			req.Header.Set("Origin", origin)
			if method == "OPTIONS" {
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "content-type,x-csrf-token")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if origin == "https://nexus.team-tool.top" {
				if w.Header().Get("Access-Control-Allow-Origin") != origin || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Fatalf("%s missing credentialed CORS: %v", method, w.Header())
				}
				if method == "OPTIONS" && (w.Code != http.StatusNoContent || !strings.Contains(strings.ToLower(w.Header().Get("Access-Control-Allow-Headers")), "x-csrf-token")) {
					t.Fatalf("preflight denied CSRF header: %d %v", w.Code, w.Header())
				}
			} else if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Fatalf("untrusted origin allowed: %v", w.Header())
			}
		}
	}
}
