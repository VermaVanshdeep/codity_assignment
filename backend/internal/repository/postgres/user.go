// Package postgres provides PostgreSQL-backed implementations of all domain repositories.
// Each repository wraps a *pgxpool.Pool and translates between SQL rows and domain types.
// All queries use parameterized statements — never string concatenation.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-org/job-scheduler/internal/domain/user"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
)

// UserRepo implements user.Repository using PostgreSQL.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a new user row. Returns AlreadyExists if the email is taken.
func (r *UserRepo) Create(ctx context.Context, u *user.User) error {
	const q = `
		INSERT INTO users (id, email, password_hash, full_name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.Exec(ctx, q,
		u.ID, u.Email, u.PasswordHash, u.FullName,
		u.IsActive, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.AlreadyExists("email")
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by primary key.
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, is_active, created_at, updated_at
		FROM users WHERE id = $1`

	u, err := scanUser(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// GetByEmail retrieves a user by email address (case-insensitive).
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, is_active, created_at, updated_at
		FROM users WHERE email = lower($1)`

	u, err := scanUser(r.db.QueryRow(ctx, q, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user")
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// Update persists changes to an existing user record.
func (r *UserRepo) Update(ctx context.Context, u *user.User) error {
	const q = `
		UPDATE users SET email = $2, full_name = $3, is_active = $4, updated_at = $5
		WHERE id = $1`

	tag, err := r.db.Exec(ctx, q, u.ID, u.Email, u.FullName, u.IsActive, u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("user")
	}
	return nil
}

func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName,
		&u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ─── Refresh Token Repo ────────────────────────────────────────────────────────

// RefreshTokenRepo implements user.RefreshTokenRepository.
type RefreshTokenRepo struct {
	db *pgxpool.Pool
}

// NewRefreshTokenRepo creates a new RefreshTokenRepo.
func NewRefreshTokenRepo(db *pgxpool.Pool) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

// Create inserts a new refresh token.
func (r *RefreshTokenRepo) Create(ctx context.Context, rt *user.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, user_agent, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.Exec(ctx, q,
		rt.ID, rt.UserID, rt.TokenHash, rt.ExpiresAt,
		rt.Revoked, rt.UserAgent, rt.IPAddress, rt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

// GetByHash retrieves a refresh token by its SHA-256 hash.
func (r *RefreshTokenRepo) GetByHash(ctx context.Context, hash string) (*user.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, revoked, user_agent, ip_address, created_at
		FROM refresh_tokens WHERE token_hash = $1`

	var rt user.RefreshToken
	err := r.db.QueryRow(ctx, q, hash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt,
		&rt.Revoked, &rt.UserAgent, &rt.IPAddress, &rt.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("refresh token")
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &rt, nil
}

// RevokeByID marks a single refresh token as revoked.
func (r *RefreshTokenRepo) RevokeByID(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked = true WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllForUser revokes all refresh tokens belonging to a user (used on logout-all).
func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`
	_, err := r.db.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("revoke all user tokens: %w", err)
	}
	return nil
}

// DeleteExpired removes expired tokens to keep the table lean. Called by a periodic cleanup job.
func (r *RefreshTokenRepo) DeleteExpired(ctx context.Context) error {
	const q = `DELETE FROM refresh_tokens WHERE expires_at < NOW()`
	_, err := r.db.Exec(ctx, q)
	if err != nil {
		return fmt.Errorf("delete expired tokens: %w", err)
	}
	return nil
}

// ─── Shared Helpers ────────────────────────────────────────────────────────────

// isUniqueViolation returns true if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isForeignKeyViolation returns true if the error is a foreign key violation.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
