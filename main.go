package main

import (
	"log"
	"net/http"
	"strings"

	"choice-matrix-backend/internal/api/routes"
	"choice-matrix-backend/internal/config"
	"choice-matrix-backend/internal/database"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize Configuration
	cfg := config.LoadConfig()

	// Initialize Database Connection
	database.ConnectDB(cfg)

	// Set up Gin Server
	r := gin.New()
	r.Use(gin.Recovery()) // Do not log authorization codes in callback query strings.

	// Configure CORS
	r.Use(browserCORS(cfg))

	// Register Routes
	routes.RegisterRoutes(r, database.DB, cfg)

	// Run Server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Nginx proxies API requests and preflights; the service owns browser CORS.
func browserCORS(cfg config.Config) gin.HandlerFunc {
	origins := cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{cfg.FrontendURL}
	}
	protected := cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	})
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/public/locales/") {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Accept, Content-Type")
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}
		protected(c)
	}
}
