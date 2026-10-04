package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppPort        string
	FrontendOrigin string
	UploadDir      string
	Database       DatabaseConfig
	JWT            JWTConfig
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type JWTConfig struct {
	Secret       string
	ExpiresHours int
}

const defaultJWTSecret = "development-only-secret-change-me"

func Load() (Config, error) {
	databasePort, err := strconv.Atoi(valueOrDefault("DB_PORT", "5432"))
	if err != nil || databasePort < 1 || databasePort > 65535 {
		return Config{}, fmt.Errorf("DB_PORT must be a valid port number")
	}

	jwtExpiresHours, err := strconv.Atoi(valueOrDefault("JWT_EXPIRES_HOURS", "24"))
	if err != nil || jwtExpiresHours < 1 {
		return Config{}, fmt.Errorf("JWT_EXPIRES_HOURS must be a positive integer")
	}

	jwtSecret, err := resolveJWTSecret()
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppPort:        valueOrDefault("APP_PORT", "8080"),
		FrontendOrigin: valueOrDefault("FRONTEND_ORIGIN", "http://localhost:4200"),
		UploadDir:      valueOrDefault("UPLOAD_DIR", "uploads"),
		Database: DatabaseConfig{
			Host:     valueOrDefault("DB_HOST", "localhost"),
			Port:     databasePort,
			User:     valueOrDefault("DB_USER", "postgres"),
			Password: valueOrDefault("DB_PASSWORD", "postgres"),
			Name:     valueOrDefault("DB_NAME", "internship_management"),
			SSLMode:  valueOrDefault("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:       jwtSecret,
			ExpiresHours: jwtExpiresHours,
		},
	}, nil
}

func valueOrDefault(name, fallback string) string {
	if value, exists := os.LookupEnv(name); exists && value != "" {
		return value
	}
	return fallback
}

func resolveJWTSecret() (string, error) {
	env := strings.ToLower(strings.TrimSpace(valueOrDefault("APP_ENV", "development")))
	value, explicitlySet := os.LookupEnv("JWT_SECRET")
	if explicitlySet {
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("JWT_SECRET must not be empty when set")
		}
	} else if env == "development" {
		return defaultJWTSecret, nil
	} else {
		return "", fmt.Errorf("JWT_SECRET must be set when APP_ENV is %q", env)
	}
	if env != "development" && value == defaultJWTSecret {
		return "", fmt.Errorf("JWT_SECRET must not use the development default when APP_ENV is %q", env)
	}
	return value, nil
}
