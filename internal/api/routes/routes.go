package routes

import (
	"choice-matrix-backend/internal/api/handlers"
	"choice-matrix-backend/internal/api/middleware"
	"choice-matrix-backend/internal/config"
	"choice-matrix-backend/internal/repository"
	"choice-matrix-backend/internal/services"
	"choice-matrix-backend/internal/sso"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg config.Config) {
	// Repositories
	userRepo := repository.NewUserRepository(db)
	i18nProjectRepo := repository.NewI18nProjectRepository(db)
	termRepo := repository.NewTermRepository(db)
	localeReleaseRepo := repository.NewLocaleReleaseRepository(db)
	langRepo := repository.NewLanguageRepository(db)
	availableLangRepo := repository.NewAvailableLanguageRepository(db)
	aiCredentialRepo := repository.NewAICredentialRepository(db)
	aiRelayService, aiRelayErr := services.NewAIRelayService(cfg.AIKeyEncryptionKey, cfg.JWTSecret, cfg.AIRelayBaseURL)

	// Handlers
	authHandler := handlers.NewAuthHandler(userRepo)
	i18nProjectHandler := handlers.NewI18nProjectHandler(i18nProjectRepo, termRepo)
	termHandler := handlers.NewTermHandler(termRepo)
	localeReleaseHandler := handlers.NewLocaleReleaseHandler(localeReleaseRepo)
	langHandler := handlers.NewLanguageHandler(langRepo, availableLangRepo)
	availableLangHandler := handlers.NewAvailableLanguageHandler(availableLangRepo)
	var aiSettingsHandler *handlers.AISettingsHandler
	if aiRelayErr == nil {
		aiSettingsHandler = handlers.NewAISettingsHandler(aiCredentialRepo, aiRelayService)
	}

	api := r.Group("/api/v1")
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "pong"})
		})
		api.GET("/public/locales/:projectCode/:languageCode", localeReleaseHandler.PublicFile)

		// Public Routes (Auth)
		auth := api.Group("/auth")
		{
			sso.New(db, cfg).Register(auth)
		}

		// Protected Routes
		protected := api.Group("/")
		protected.Use(sso.New(db, cfg).AuthMiddleware())
		{
			protected.GET("/auth/me", authHandler.GetCurrentUser)
			if aiSettingsHandler != nil {
				protected.GET("/ai/settings", aiSettingsHandler.Settings)
				protected.POST("/ai/settings", aiSettingsHandler.Save)
				protected.POST("/ai/settings/delete", aiSettingsHandler.Delete)
				protected.POST("/ai/translate", aiSettingsHandler.Translate)
			} else {
				aiUnavailable := func(c *gin.Context) {
					c.JSON(200, gin.H{"code": 503, "message": "AI 翻译服务初始化失败"})
				}
				protected.GET("/ai/settings", aiUnavailable)
				protected.POST("/ai/settings", aiUnavailable)
				protected.POST("/ai/settings/delete", aiUnavailable)
				protected.POST("/ai/translate", aiUnavailable)
			}
			protected.GET("/languages", availableLangHandler.List)
			languageAdmin := protected.Group("/languages")
			languageAdmin.Use(middleware.AdminMiddleware(db))
			{
				languageAdmin.POST("", availableLangHandler.Create)
				languageAdmin.POST("/:id/update", availableLangHandler.Update)
				languageAdmin.POST("/:id/delete", availableLangHandler.Delete)
			}

			// I18n Projects CRUD
			projects := protected.Group("/projects")
			{
				projects.GET("/:id/locale-release", localeReleaseHandler.Latest)
				projectReleaseAdmin := projects.Group("/:id/locale-release")
				projectReleaseAdmin.Use(middleware.AdminMiddleware(db))
				projectReleaseAdmin.POST("/publish", localeReleaseHandler.Publish)
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
				projects.POST("/:id/terms/batch-create", termHandler.BatchCreateTerms)
				projects.POST("/:id/terms/:termId/update", termHandler.UpdateTerm)
				projects.POST("/:id/terms/batch-update", termHandler.BatchUpdateTerms)
				projects.POST("/:id/terms/:termId/publish", termHandler.PublishTerm)
				projects.POST("/:id/terms/:termId/delete", termHandler.DeleteTerm)
				projects.POST("/:id/terms/import-json", termHandler.ImportJSON)
				projects.POST("/:id/terms/bulk/preview", termHandler.PreviewBulkTerms)
				projects.POST("/:id/terms/bulk/execute", termHandler.ExecuteBulkTerms)
				projects.POST("/:id/terms/export/json", termHandler.ExportTermsJSON)
				projects.POST("/:id/terms/export/excel", termHandler.ExportTermsExcel)
				projects.POST("/:id/terms/export/copy", termHandler.CopyTermsJSON)
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
