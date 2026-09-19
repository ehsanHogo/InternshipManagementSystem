package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	AppPort        string
	FrontendOrigin string
	Database       DatabaseConfig
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func Load() (Config, error) {
	databasePort, err := strconv.Atoi(valueOrDefault("DB_PORT", "5432"))
	if err != nil || databasePort < 1 || databasePort > 65535 {
		return Config{}, fmt.Errorf("DB_PORT must be a valid port number")
	}

	return Config{
		AppPort:        valueOrDefault("APP_PORT", "8080"),
		FrontendOrigin: valueOrDefault("FRONTEND_ORIGIN", "http://localhost:4200"),
		Database: DatabaseConfig{
			Host:     valueOrDefault("DB_HOST", "localhost"),
			Port:     databasePort,
			User:     valueOrDefault("DB_USER", "postgres"),
			Password: valueOrDefault("DB_PASSWORD", "postgres"),
			Name:     valueOrDefault("DB_NAME", "internship_management"),
			SSLMode:  valueOrDefault("DB_SSLMODE", "disable"),
		},
	}, nil
}

func valueOrDefault(name, fallback string) string {
	if value, exists := os.LookupEnv(name); exists && value != "" {
		return value
	}
	return fallback
}
