// Package org provides business logic for Organization management.
package org

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/org"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

// CreateRequest contains fields to create a new organization.
type CreateRequest struct {
	Name        string `json:"name"        validate:"required,min=2,max=100"`
	Slug        string `json:"slug"        validate:"required,slug,max=60"`
	Description string `json:"description" validate:"max=500"`
}

// UpdateRequest contains mutable organization fields.
type UpdateRequest struct {
	Name        string `json:"name"        validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// AddMemberRequest contains fields to invite a member.
type AddMemberRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Role   org.Role  `json:"role"    validate:"required,oneof=owner admin member"`
}

// OrgResponse is the API representation of an Organization.
type OrgResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// Optional: populated when listing orgs for a user
	Role *org.Role `json:"role,omitempty"`
}

// MemberResponse is the API representation of an org member.
type MemberResponse struct {
	UserID       uuid.UUID `json:"user_id"`
	UserEmail    string    `json:"email"`
	UserFullName string    `json:"full_name"`
	Role         org.Role  `json:"role"`
	JoinedAt     time.Time `json:"joined_at"`
}

// ─── Interface ─────────────────────────────────────────────────────────────────

// Service defines the org management business logic.
type Service interface {
	Create(ctx context.Context, ownerID uuid.UUID, req CreateRequest) (*OrgResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*OrgResponse, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]*OrgResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*OrgResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, orgID uuid.UUID, req AddMemberRequest) (*MemberResponse, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role org.Role) (*MemberResponse, error)
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*MemberResponse, error)
}

// ─── Implementation ────────────────────────────────────────────────────────────

type service struct {
	orgs org.Repository
	log  *logger.Logger
}

// NewService creates a new org Service.
func NewService(orgs org.Repository, log *logger.Logger) Service {
	return &service{
		orgs: orgs,
		log:  log.WithField("service", "org"),
	}
}

// Create creates a new organization and makes the creator its owner.
func (s *service) Create(ctx context.Context, ownerID uuid.UUID, req CreateRequest) (*OrgResponse, error) {
	now := time.Now().UTC()
	o := &org.Organization{
		ID:          uuid.New(),
		Name:        strings.TrimSpace(req.Name),
		Slug:        strings.ToLower(strings.TrimSpace(req.Slug)),
		Description: strings.TrimSpace(req.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.orgs.Create(ctx, o); err != nil {
		return nil, err
	}

	// Make the creator the owner — transactional in a real system,
	// but acceptable here as partial failure recovery is handled via cleanup.
	if err := s.orgs.AddMember(ctx, o.ID, ownerID, org.RoleOwner); err != nil {
		// Best-effort: delete the org if member assignment fails.
		_ = s.orgs.Delete(ctx, o.ID)
		return nil, apperrors.Internal("could not assign org owner", err)
	}

	s.log.Info("org created",
		logger.String("org_id", o.ID.String()),
		logger.String("owner_id", ownerID.String()),
	)
	return toOrgResponse(o, nil), nil
}

// GetByID retrieves an organization by ID.
func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*OrgResponse, error) {
	o, err := s.orgs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toOrgResponse(o, nil), nil
}

// ListForUser returns all organizations a user belongs to, with their role.
func (s *service) ListForUser(ctx context.Context, userID uuid.UUID) ([]*OrgResponse, error) {
	orgs, err := s.orgs.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	responses := make([]*OrgResponse, 0, len(orgs))
	for _, o := range orgs {
		// Fetch the user's role in each org for the response.
		member, err := s.orgs.GetMember(ctx, o.ID, userID)
		var role *org.Role
		if err == nil {
			r := member.Role
			role = &r
		}
		responses = append(responses, toOrgResponse(o, role))
	}
	return responses, nil
}

// Update modifies an organization's mutable fields.
func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*OrgResponse, error) {
	o, err := s.orgs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	o.Name = strings.TrimSpace(req.Name)
	o.Description = strings.TrimSpace(req.Description)
	o.UpdatedAt = time.Now().UTC()

	if err := s.orgs.Update(ctx, o); err != nil {
		return nil, err
	}
	return toOrgResponse(o, nil), nil
}

// Delete permanently removes an organization and all its data.
func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.orgs.Delete(ctx, id)
}

// AddMember adds a user to an organization with the specified role.
func (s *service) AddMember(ctx context.Context, orgID uuid.UUID, req AddMemberRequest) (*MemberResponse, error) {
	if err := s.orgs.AddMember(ctx, orgID, req.UserID, req.Role); err != nil {
		return nil, err
	}

	member, err := s.orgs.GetMember(ctx, orgID, req.UserID)
	if err != nil {
		return nil, err
	}
	return toMemberResponse(member), nil
}

// UpdateMemberRole changes a member's role in the organization.
func (s *service) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role org.Role) (*MemberResponse, error) {
	if err := s.orgs.UpdateMemberRole(ctx, orgID, userID, role); err != nil {
		return nil, err
	}
	member, err := s.orgs.GetMember(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	return toMemberResponse(member), nil
}

// RemoveMember removes a user from an organization.
func (s *service) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.orgs.RemoveMember(ctx, orgID, userID)
}

// ListMembers returns all members of an organization.
func (s *service) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*MemberResponse, error) {
	members, err := s.orgs.ListMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	responses := make([]*MemberResponse, 0, len(members))
	for _, m := range members {
		responses = append(responses, toMemberResponse(m))
	}
	return responses, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func toOrgResponse(o *org.Organization, role *org.Role) *OrgResponse {
	return &OrgResponse{
		ID:          o.ID,
		Name:        o.Name,
		Slug:        o.Slug,
		Description: o.Description,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
		Role:        role,
	}
}

func toMemberResponse(m *org.Member) *MemberResponse {
	return &MemberResponse{
		UserID:       m.UserID,
		UserEmail:    m.UserEmail,
		UserFullName: m.UserFullName,
		Role:         m.Role,
		JoinedAt:     m.JoinedAt,
	}
}
