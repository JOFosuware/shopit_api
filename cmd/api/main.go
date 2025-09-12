package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jofosuware/go/shopit/config"
	"github.com/jofosuware/go/shopit/internal/server"
	"github.com/jofosuware/go/shopit/pkg/driver"
	"github.com/jofosuware/go/shopit/pkg/logger"
)

func main() {
	log.Println("Starting api server")

	cfgFile, err := config.LoadConfig("./config/config-local")
	if err != nil {
		log.Fatalf("LoadConfig: %v", err)
	}

	cfg, err := config.ParseConfig(cfgFile)
	if err != nil {
		log.Fatalf("ParseConfig: %v", err)
	}

	appLogger := logger.NewApiLogger(cfg)

	appLogger.InitLogger()
	appLogger.Infof("AppVersion: %s, LogLevel: %s, Mode: %s, SSL: %t", cfg.Server.AppVersion, cfg.Logger.Level, cfg.Server.Mode, cfg.Server.SSL)

	// connect to database
	appLogger.Info("Connecting to database...")

	var connectionString = cfg.Postgres.Url

	if cfg.Server.Mode == "Development" {
		connectionString = fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s", cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Dbname, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.SSLMode)
	}

	db, err := driver.ConnectSQL(connectionString)
	if err != nil {
		appLogger.Fatal(err)
	}
	d := db.SQL
	appLogger.Info("Connected to database!")

	cfg.Server.Port = os.Getenv("PORT")
	if cfg.Server.Port == "" {
		cfg.Server.Port = "5000"
	}

	s := server.NewServer(cfg, appLogger, d)
	s.Setup()

	if err = s.Run(); err != nil {
		log.Fatal(err)
	}
}
