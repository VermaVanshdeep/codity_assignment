// Package project provides business logic for Project management and API key lifecycle.
package project

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/project"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

// CreateRequest contains fields to create a new project.
type CreateRequest struct {
	Name        string `json:"name"        validate:"required,min=2,max=100"`
	Slug        string `json:"slug"        validate:"required,slug,max=60"`
	Description string `json:"description" validate:"max=500"`
}

// UpdateRequest contains mutable project fields.
type UpdateRequest struct {
	Name        string `json:"name"        validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// ProjectResponse is the public API representation of a project.
// API key hash is never returned; only the non-secret prefix is shown.
type ProjectResponse struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	APIKeyPrefix string    `json:"api_key_prefix"` // e.g. "sk_abc123" — safe to show
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// APIKeyResponse is returned ONLY on project creation or key rotation.
// The raw API key is shown once and never again.
type APIKeyResponse struct {
	APIKey       string `json:"api_key"` // raw key — display once
	APIKeyPrefix string `json:"api_key_prefix"`
}

// ─── Interface ─────────────────────────────────────────────────────────────────

// Service defines the project management business logic.
type Service interface {
	Create(ctx context.Context, orgID, ownerID uuid.UUID, req CreateRequest) (*ProjectResponse, *APIKeyResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*ProjectResponse, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*ProjectResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*ProjectResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	RotateAPIKey(ctx context.Context, id uuid.UUID) (*APIKeyResponse, error)
}

// ─── Implementation ────────────────────────────────────────────────────────────

type service struct {
	projects project.Repository
	log      *logger.Logger
}

// NewService creates a new project Service.
func NewService(projects project.Repository, log *logger.Logger) Service {
	return &service{
		projects: projects,
		log:      log.WithField("service", "project"),
	}
}

// Create creates a new project and generates its initial API key.
// The raw API key is returned only once; subsequent calls return only the prefix.
func (s *service) Create(ctx context.Context, orgID, ownerID uuid.UUID, req CreateRequest) (*ProjectResponse, *APIKeyResponse, error) {
	rawKey, keyHash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, nil, apperrors.Internal("could not generate API key", err)
	}

	now := time.Now().UTC()
	p := &project.Project{
		ID:           uuid.New(),
		OrgID:        orgID,
		Name:         strings.TrimSpace(req.Name),
		Slug:         strings.ToLower(strings.TrimSpace(req.Slug)),
		Description:  strings.TrimSpace(req.Description),
		APIKeyHash:   keyHash,
		APIKeyPrefix: prefix,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.projects.Create(ctx, p); err != nil {
		return nil, nil, err
	}

	// Make creator an admin of the project.
	if err := s.projects.AddMember(ctx, p.ID, ownerID, project.RoleAdmin); err != nil {
		_ = s.projects.Delete(ctx, p.ID)
		return nil, nil, apperrors.Internal("could not assign project admin", err)
	}

	s.log.Info("project created",
		logger.String("project_id", p.ID.String()),
		logger.String("org_id", orgID.String()),
	)

	return toProjectResponse(p), &APIKeyResponse{APIKey: rawKey, APIKeyPrefix: prefix}, nil
}

// GetByID retrieves a project by its ID.
func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*ProjectResponse, error) {
	p, err := s.projects.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toProjectResponse(p), nil
}

// ListByOrg returns all projects belonging to an organization.
func (s *service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*ProjectResponse, error) {
	projects, err := s.projects.ListByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	responses := make([]*ProjectResponse, 0, len(projects))
	for _, p := range projects {
		responses = append(responses, toProjectResponse(p))
	}
	return responses, nil
}

// Update modifies a project's mutable fields.
func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*ProjectResponse, error) {
	p, err := s.projects.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	p.Name = strings.TrimSpace(req.Name)
	p.Description = strings.TrimSpace(req.Description)
	p.UpdatedAt = time.Now().UTC()

	if err := s.projects.Update(ctx, p); err != nil {
		return nil, err
	}
	return toProjectResponse(p), nil
}

// Delete permanently removes a project.
func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.projects.Delete(ctx, id)
}

// RotateAPIKey invalidates the old API key and generates a new one.
// The new raw key is returned and must be shown to the user immediately.
func (s *service) RotateAPIKey(ctx context.Context, id uuid.UUID) (*APIKeyResponse, error) {
	p, err := s.projects.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	rawKey, keyHash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, apperrors.Internal("could not generate API key", err)
	}

	p.APIKeyHash = keyHash
	p.APIKeyPrefix = prefix
	p.UpdatedAt = time.Now().UTC()

	if err := s.projects.Update(ctx, p); err != nil {
		return nil, err
	}

	s.log.Info("API key rotated", logger.String("project_id", id.String()))
	return &APIKeyResponse{APIKey: rawKey, APIKeyPrefix: prefix}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// generateAPIKey creates a new random API key, its SHA-256 hash, and a safe prefix.
// Format: sk_<first-12-hex-chars-of-random-bytes>  (prefix, safe to store/display)
func generateAPIKey() (rawKey, hash, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("rand read: %w", err)
	}
	rawHex := hex.EncodeToString(b)
	rawKey = "sk_" + rawHex

	h := sha256.Sum256([]byte(rawKey))
	hash = hex.EncodeToString(h[:])
	prefix = rawKey[:15] + "..." // show first 15 chars in UI
	return
}

func toProjectResponse(p *project.Project) *ProjectResponse {
	return &ProjectResponse{
		ID:           p.ID,
		OrgID:        p.OrgID,
		Name:         p.Name,
		Slug:         p.Slug,
		Description:  p.Description,
		APIKeyPrefix: p.APIKeyPrefix,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}
