// sso-inspect lists database names on the configured server; it never writes.
package main

import (
	"choice-matrix-backend/internal/config"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"net"
	"net/url"
)

func main() {
	cfg := config.LoadConfig()
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(cfg.DBUser, cfg.DBPass), Host: net.JoinHostPort(cfg.DBHost, cfg.DBPort), Path: "/" + cfg.DBName}
	u.RawQuery = "sslmode=disable"
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	var names []string
	if err = db.Raw("SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname").Scan(&names).Error; err != nil {
		log.Fatal(err)
	}
	for _, name := range names {
		fmt.Println(name)
	}
	var tables []string
	if err = db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name").Scan(&tables).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Println("Tables in configured database:")
	for _, name := range tables {
		fmt.Println(name)
	}
}
