package main

import (
	"log"

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
	r := gin.Default()

	// Configure CORS
	allowedOrigins := cfg.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Register Routes
	routes.RegisterRoutes(r, database.DB)

	// Run Server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
