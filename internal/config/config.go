package config

import (
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DBHost             string
	DBUser             string
	DBPass             string
	DBName             string
	DBPort             string
	JWTSecret          string
	AIKeyEncryptionKey string
	AIRelayBaseURL     string
	AutoMigrate        bool
	AllowedOrigins     []string
	FrontendURL        string
	CookieSecure       bool
	HeliosBaseURL      string
	HeliosInternalURL  string
	HeliosClientID     string
	HeliosClientSecret string
	HeliosRedirectURI  string
	HeliosProjectID    string
	HeliosAllowedRoles []string
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

	cfg := Config{
		Port:               getEnv("PORT", "8080"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPass:             getEnv("DB_PASSWORD", getEnv("DB_PASS", "postgres")),
		DBName:             getEnv("DB_NAME", "choice_matrix"),
		DBPort:             getEnv("DB_PORT", "5432"),
		JWTSecret:          getEnv("JWT_SECRET", "my-super-secret-choice-matrix-key"),
		AIKeyEncryptionKey: getEnv("AI_KEY_ENCRYPTION_KEY", ""),
		AIRelayBaseURL:     getEnv("AI_RELAY_BASE_URL", "https://43.172.5.210:3000/v1"),
		AutoMigrate:        getEnv("AUTO_MIGRATE", "false") == "true",
		AllowedOrigins:     allowedOrigins,
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		CookieSecure:       getEnv("COOKIE_SECURE", "true") == "true",
		HeliosBaseURL:      getEnv("HELIOS_BASE_URL", "https://helios-api.team-tool.top"),
		HeliosInternalURL:  getEnv("HELIOS_INTERNAL_URL", "http://127.0.0.1:3456"),
		HeliosClientID:     getEnv("HELIOS_CLIENT_ID", ""),
		HeliosClientSecret: getEnv("HELIOS_CLIENT_SECRET", ""),
		HeliosRedirectURI:  getEnv("HELIOS_REDIRECT_URI", "https://nexus-api.team-tool.top/api/v1/auth/sso/callback"),
		HeliosProjectID:    getEnv("HELIOS_PROJECT_ID", "nexus-multilingual"),
		HeliosAllowedRoles: strings.Split(getEnv("HELIOS_ALLOWED_ROLES", "admin,owner,productor,developer,translator"), ","),
	}
	for _, raw := range []string{cfg.FrontendURL, cfg.HeliosBaseURL, cfg.HeliosRedirectURI} {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && loopback(u.Hostname()))) {
			log.Fatal("Browser SSO URLs must use HTTPS, or HTTP on loopback for local development")
		}
	}
	u, _ := url.Parse(cfg.FrontendURL)
	if !cfg.CookieSecure && !loopback(u.Hostname()) {
		log.Fatal("COOKIE_SECURE=false is only allowed for loopback development")
	}
	for i := range cfg.HeliosAllowedRoles {
		cfg.HeliosAllowedRoles[i] = strings.TrimSpace(cfg.HeliosAllowedRoles[i])
	}
	return cfg
}

func loopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
