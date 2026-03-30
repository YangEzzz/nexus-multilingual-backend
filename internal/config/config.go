package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DBHost         string
	DBUser         string
	DBPass         string
	DBName         string
	DBPort         string
	JWTSecret      string
	AutoMigrate    bool
	AllowedOrigins []string
}

func LoadConfig() Config {
	// 优先加载 .env-production，如果没有则退而求其次加载 .env
	err := godotenv.Load(".env-production")
	if err != nil {
		log.Println(".env-production not found, trying .env")
		godotenv.Load(".env")
	}

	allowedOriginsRaw := getEnv("ALLOWED_ORIGINS", "")
	var allowedOrigins []string
	if allowedOriginsRaw != "" {
		for _, o := range strings.Split(allowedOriginsRaw, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	return Config{
		Port:           getEnv("PORT", "8080"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPass:         getEnv("DB_PASSWORD", getEnv("DB_PASS", "postgres")),
		DBName:         getEnv("DB_NAME", "choice_matrix"),
		DBPort:         getEnv("DB_PORT", "5432"),
		JWTSecret:      getEnv("JWT_SECRET", "my-super-secret-choice-matrix-key"),
		AutoMigrate:    getEnv("AUTO_MIGRATE", "false") == "true",
		AllowedOrigins: allowedOrigins,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
