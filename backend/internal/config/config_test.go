package config

import (
	"os"
	"testing"
)

func TestResolveJWTSecret(t *testing.T) {
	t.Run("development allows default when unset", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")
		if err := os.Unsetenv("JWT_SECRET"); err != nil {
			t.Fatalf("unset JWT_SECRET: %v", err)
		}
		secret, err := resolveJWTSecret()
		if err != nil || secret != defaultJWTSecret {
			t.Fatalf("resolveJWTSecret() = %q, %v; want default", secret, err)
		}
	})

	t.Run("production requires explicit secret", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		if err := os.Unsetenv("JWT_SECRET"); err != nil {
			t.Fatalf("unset JWT_SECRET: %v", err)
		}
		if _, err := resolveJWTSecret(); err == nil {
			t.Fatal("expected error when JWT_SECRET is unset in production")
		}
	})

	t.Run("production rejects development default", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("JWT_SECRET", defaultJWTSecret)
		if _, err := resolveJWTSecret(); err == nil {
			t.Fatal("expected error for default JWT_SECRET in production")
		}
	})

	t.Run("production accepts custom secret", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("JWT_SECRET", "a-long-random-production-secret")
		secret, err := resolveJWTSecret()
		if err != nil || secret != "a-long-random-production-secret" {
			t.Fatalf("resolveJWTSecret() = %q, %v", secret, err)
		}
	})
}
