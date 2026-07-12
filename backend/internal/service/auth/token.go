// Package auth provides authentication services: JWT generation, refresh token
// management, and password hashing.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/config"
)

// Claims are the JWT payload fields embedded in every access token.
type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"uid"`
	Email  string `json:"email"`
}

// tokenService handles JWT generation and refresh token cryptography.
// It is an internal helper — only the AuthService uses it.
type tokenService struct {
	cfg config.JWTConfig
}

func newTokenService(cfg config.JWTConfig) *tokenService {
	return &tokenService{cfg: cfg}
}

// GenerateAccessToken creates a signed JWT access token for the given user.
func (t *tokenService) GenerateAccessToken(userID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.cfg.AccessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "job-scheduler",
		},
		UserID: userID.String(),
		Email:  email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(t.cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

// ValidateAccessToken parses and validates a JWT access token string.
// Returns the decoded Claims on success.
func (t *tokenService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(t.cfg.Secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// GenerateRefreshToken creates a cryptographically random opaque refresh token.
// Returns the raw token (sent to client) and its SHA-256 hash (stored in DB).
func GenerateRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	raw = base64.URLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

// HashRefreshToken returns the SHA-256 hex hash of a raw refresh token.
// Used when looking up an incoming token from the client.
func HashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
