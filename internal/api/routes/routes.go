package routes

import (
	"choice-matrix-backend/internal/api/handlers"
	"choice-matrix-backend/internal/api/middleware"
	"choice-matrix-backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB) {
	// Repositories
	userRepo := repository.NewUserRepository(db)
	i18nProjectRepo := repository.NewI18nProjectRepository(db)
	termRepo := repository.NewTermRepository(db)
	langRepo := repository.NewLanguageRepository(db)

	// Handlers
	authHandler := handlers.NewAuthHandler(userRepo)
	i18nProjectHandler := handlers.NewI18nProjectHandler(i18nProjectRepo, termRepo)
	termHandler := handlers.NewTermHandler(termRepo)
	langHandler := handlers.NewLanguageHandler(langRepo)

	api := r.Group("/api/v1")
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "pong"})
		})

		// Public Routes (Auth)
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
		}

		// Protected Routes
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/auth/me", authHandler.GetCurrentUser)

			// I18n Projects CRUD
			projects := protected.Group("/projects")
			{
				projects.GET("", i18nProjectHandler.List)
				projects.GET("/:id", i18nProjectHandler.Get)
				projects.GET("/:id/dashboard", i18nProjectHandler.Dashboard)
				projects.POST("", i18nProjectHandler.Create)
				projects.POST("/:id/update", i18nProjectHandler.Update)
				projects.POST("/:id/delete", i18nProjectHandler.Delete)

				// Term routes for a specific project
				projects.GET("/:id/terms", termHandler.List)
				projects.GET("/:id/stats", termHandler.GetStats)
				projects.POST("/:id/terms", termHandler.Create)
				projects.POST("/:id/terms/:termId/update", termHandler.UpdateTerm)
				projects.POST("/:id/terms/batch-update", termHandler.BatchUpdateTerms)
				projects.POST("/:id/terms/:termId/publish", termHandler.PublishTerm)
				projects.POST("/:id/terms/:termId/delete", termHandler.DeleteTerm)
				projects.POST("/:id/terms/import-json", termHandler.ImportJSON)
				projects.POST("/:id/terms/batch-publish", termHandler.BatchPublishTerms)
				projects.POST("/:id/terms/batch-delete", termHandler.BatchDeleteTerms)
				
				// Project logs
				projects.GET("/:id/logs", termHandler.GetProjectLogs)

				// Language routes for a specific project
				projects.GET("/:id/languages", langHandler.Get)
				projects.POST("/:id/languages/update", langHandler.Update)
			}
		}
	}
}
