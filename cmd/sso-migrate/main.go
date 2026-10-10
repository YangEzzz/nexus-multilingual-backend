// sso-migrate applies only the additive SSO migration to the configured database.
package main

import (
	"choice-matrix-backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"net"
	"net/url"
	"os"
)

func main() {
	cfg := config.LoadConfig()
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(cfg.DBUser, cfg.DBPass), Host: net.JoinHostPort(cfg.DBHost, cfg.DBPort), Path: "/" + cfg.DBName}
	u.RawQuery = "sslmode=disable"
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	migration, err := os.ReadFile("migrations/20261010_sso.sql")
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Transaction(func(tx *gorm.DB) error { return tx.Exec(string(migration)).Error }); err != nil {
		log.Fatal(err)
	}
	log.Println("SSO migration applied; existing business tables unchanged")
}
