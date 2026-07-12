package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/your-org/job-scheduler/internal/config"
	"github.com/your-org/job-scheduler/internal/domain/user"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

// RegisterRequest contains the fields needed to create a new user account.
type RegisterRequest struct {
	Email    string `json:"email"     validate:"required,email"`
	Password string `json:"password"  validate:"required,min=8,max=128"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
}

// LoginRequest contains credentials for user authentication.
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse is returned on successful authentication.
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int64    `json:"expires_in"` // seconds until access token expiry
	User         *UserDTO `json:"user"`
}

// RefreshResponse is returned when a refresh token is exchanged for a new access token.
type RefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// UserDTO is the safe public representation of a user (no password hash).
type UserDTO struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Interface ─────────────────────────────────────────────────────────────────

// Service defines the authentication business logic contract.
type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*UserDTO, error)
	Login(ctx context.Context, req LoginRequest, userAgent, ip string) (*LoginResponse, error)
	Refresh(ctx context.Context, rawRefreshToken string) (*RefreshResponse, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
	ValidateAccessToken(tokenStr string) (*Claims, error)
}

// ─── Implementation ────────────────────────────────────────────────────────────

// service is the concrete implementation of Service.
type service struct {
	users         user.Repository
	refreshTokens user.RefreshTokenRepository
	tokens        *tokenService
	cfg           config.JWTConfig
	log           *logger.Logger
}

// NewService creates a new auth Service with all its dependencies injected.
func NewService(
	users user.Repository,
	refreshTokens user.RefreshTokenRepository,
	cfg config.JWTConfig,
	log *logger.Logger,
) Service {
	return &service{
		users:         users,
		refreshTokens: refreshTokens,
		tokens:        newTokenService(cfg),
		cfg:           cfg,
		log:           log.WithField("service", "auth"),
	}
}

// Register creates a new user account.
// Returns AlreadyExists if the email is already registered.
func (s *service) Register(ctx context.Context, req RegisterRequest) (*UserDTO, error) {
	// Normalize email to lowercase.
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Hash password with bcrypt (cost 12 — secure but fast enough for registration).
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	u := &user.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: string(hash),
		FullName:     strings.TrimSpace(req.FullName),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.Create(ctx, u); err != nil {
		return nil, err // already wrapped by repo (AlreadyExists or internal)
	}

	s.log.Info("user registered", logger.String("user_id", u.ID.String()))
	return toUserDTO(u), nil
}

// Login authenticates a user and returns JWT access + refresh tokens.
func (s *service) Login(ctx context.Context, req LoginRequest, userAgent, ip string) (*LoginResponse, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	u, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		// Return generic error — do NOT leak whether the email exists.
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	if !u.IsActive {
		return nil, apperrors.Unauthorized("account is deactivated")
	}

	// Constant-time bcrypt comparison prevents timing attacks.
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	// Generate access token.
	accessToken, err := s.tokens.GenerateAccessToken(u.ID, u.Email)
	if err != nil {
		return nil, apperrors.Internal("could not generate access token", err)
	}

	// Generate opaque refresh token.
	rawRefresh, refreshHash, err := GenerateRefreshToken()
	if err != nil {
		return nil, apperrors.Internal("could not generate refresh token", err)
	}

	// Persist the hashed refresh token.
	rt := &user.RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().UTC().Add(s.cfg.RefreshTokenTTL),
		Revoked:   false,
		UserAgent: userAgent,
		IPAddress: ip,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.refreshTokens.Create(ctx, rt); err != nil {
		return nil, apperrors.Internal("could not store refresh token", err)
	}

	s.log.Info("user logged in",
		logger.String("user_id", u.ID.String()),
		logger.String("ip", ip),
	)

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
		User:         toUserDTO(u),
	}, nil
}

// Refresh exchanges a valid refresh token for a new access token.
func (s *service) Refresh(ctx context.Context, rawRefreshToken string) (*RefreshResponse, error) {
	hash := HashRefreshToken(rawRefreshToken)

	rt, err := s.refreshTokens.GetByHash(ctx, hash)
	if err != nil {
		return nil, apperrors.Unauthorized("invalid refresh token")
	}

	if !rt.IsValid(time.Now().UTC()) {
		return nil, apperrors.Unauthorized("refresh token has expired or been revoked")
	}

	// Fetch the user to ensure they're still active.
	u, err := s.users.GetByID(ctx, rt.UserID)
	if err != nil || !u.IsActive {
		return nil, apperrors.Unauthorized("user account no longer active")
	}

	accessToken, err := s.tokens.GenerateAccessToken(u.ID, u.Email)
	if err != nil {
		return nil, apperrors.Internal("could not generate access token", err)
	}

	return &RefreshResponse{
		AccessToken: accessToken,
		ExpiresIn:   int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

// Logout revokes a specific refresh token (single-device logout).
func (s *service) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := HashRefreshToken(rawRefreshToken)

	rt, err := s.refreshTokens.GetByHash(ctx, hash)
	if err != nil {
		return nil // already invalid — treat as success
	}

	return s.refreshTokens.RevokeByID(ctx, rt.ID)
}

// LogoutAll revokes all refresh tokens for a user (all-device logout).
func (s *service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.refreshTokens.RevokeAllForUser(ctx, userID)
}

// ValidateAccessToken parses and validates a JWT string.
// Used by the auth middleware.
func (s *service) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return s.tokens.ValidateAccessToken(tokenStr)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func toUserDTO(u *user.User) *UserDTO {
	return &UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		CreatedAt: u.CreatedAt,
	}
}
