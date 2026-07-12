// Package user defines the User domain entity and its repository contracts.
// This package has zero infrastructure dependencies — it is the innermost
// layer of clean architecture and defines what a User IS and what the system
// needs to persist and retrieve.
package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// User is the core authenticated principal in the system.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FullName     string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RefreshToken represents a long-lived opaque token stored as a one-way hash.
// Design: The raw token value is returned to the client exactly once (on login).
// Only its SHA-256 hash is stored in the database, so a DB breach cannot be
// used to forge tokens.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string // SHA-256(raw_token) encoded as hex
	ExpiresAt time.Time
	Revoked   bool
	UserAgent string
	IPAddress string
	CreatedAt time.Time
}

// IsExpired returns true if the token has passed its expiry time.
func (rt *RefreshToken) IsExpired(now time.Time) bool {
	return now.After(rt.ExpiresAt)
}

// IsValid returns true if the token can be used (not expired and not revoked).
func (rt *RefreshToken) IsValid(now time.Time) bool {
	return !rt.Revoked && !rt.IsExpired(now)
}

// Repository defines the data access contract for user persistence.
type Repository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
}

// RefreshTokenRepository defines operations for persisting refresh tokens.
type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeByID(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}
