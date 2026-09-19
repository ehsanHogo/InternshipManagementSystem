package auth

import (
	"testing"
	"time"

	"internship-management-system/backend/internal/model"
)

func TestCreateAndParseToken(t *testing.T) {
	user := model.User{ID: 42, Email: "student@demo.local", Role: model.RoleStudent}
	token, err := CreateToken(user, "test-secret", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned an error: %v", err)
	}

	claims, err := ParseToken(token, "test-secret")
	if err != nil {
		t.Fatalf("ParseToken returned an error: %v", err)
	}
	if claims.UserID != user.ID || claims.Role != user.Role || claims.Email != user.Email {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestParseTokenRejectsInvalidTokens(t *testing.T) {
	user := model.User{ID: 1, Role: model.RoleAdmin}
	validToken, err := CreateToken(user, "correct-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	expiredToken, err := CreateToken(user, "correct-secret", -time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	for name, token := range map[string]string{
		"wrong signature": validToken,
		"expired":         expiredToken,
		"malformed":       "not-a-jwt",
	} {
		t.Run(name, func(t *testing.T) {
			secret := "correct-secret"
			if name == "wrong signature" {
				secret = "wrong-secret"
			}
			if _, err := ParseToken(token, secret); err == nil {
				t.Fatal("invalid token was accepted")
			}
		})
	}
}
