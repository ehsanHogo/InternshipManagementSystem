package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"internship-management-system/backend/internal/model"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UserID uint       `json:"user_id"`
	Role   model.Role `json:"role"`
	Email  string     `json:"email,omitempty"`
	Issued int64      `json:"iat"`
	Expiry int64      `json:"exp"`
}

func CreateToken(user model.User, secret string, lifetime time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		Email:  user.Email,
		Issued: now.Unix(),
		Expiry: now.Add(lifetime).Unix(),
	}

	headerJSON, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", fmt.Errorf("marshal token header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal token claims: %w", err)
	}

	unsigned := encode(headerJSON) + "." + encode(claimsJSON)
	signature := sign(unsigned, secret)
	return unsigned + "." + encode(signature), nil
}

func ParseToken(token, secret string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, ErrInvalidToken
	}

	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(providedSignature, sign(parts[0]+"."+parts[1], secret)) {
		return Claims{}, ErrInvalidToken
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if json.Unmarshal(claimsBytes, &claims) != nil || claims.UserID == 0 || !claims.Role.Valid() || claims.Expiry <= time.Now().Unix() {
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}

func encode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func sign(value, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
