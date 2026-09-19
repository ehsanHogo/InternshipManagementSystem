package main

import (
	"log"

	"internship-management-system/backend/internal/config"
	"internship-management-system/backend/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	if err := database.MigrateAndSeed(db); err != nil {
		log.Fatalf("prepare database: %v", err)
	}

	log.Print("database migration and seed completed")
}
