package database

import (
	"fmt"
	"log"

	"choice-matrix-backend/internal/config"
	"choice-matrix-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB(cfg config.Config) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBPort)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 优化连接池设置，避免本地并发查询时反复耗时建立新连接
	sqlDB, err := DB.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(10)   // 限制最大空闲连接
		sqlDB.SetMaxOpenConns(100)  // 限制最大打开连接数
	}

	log.Println("Database connection successfully opened")

	// Auto-migrate models if configured
	if cfg.AutoMigrate {
		err = DB.AutoMigrate(
			&models.User{},
			&models.I18nProject{},
			&models.Language{},
			&models.Term{},
			&models.Translation{},
			&models.HistoryLog{},
		)
		if err != nil {
			log.Fatalf("Failed to migrate database: %v", err)
		}
		log.Println("Database migration completed")
	} else {
		log.Println("Database auto-migration skipped (AUTO_MIGRATE=false)")
	}
}
