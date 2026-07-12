// Package org defines the Organization domain entity and its repository contract.
// An Organization is the top-level multi-tenant container. Every project, queue,
// and job belongs to exactly one organization via the project hierarchy.
package org

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Role represents the level of access a user has within an organization.
// Roles form a hierarchy: owner > admin > member.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// CanManageMembers returns true if the role can add/remove org members.
func (r Role) CanManageMembers() bool {
	return r == RoleOwner || r == RoleAdmin
}

// CanDeleteOrg returns true if the role can delete the organization.
func (r Role) CanDeleteOrg() bool {
	return r == RoleOwner
}

// Organization is the root multi-tenant container.
type Organization struct {
	ID          uuid.UUID
	Name        string
	Slug        string // URL-safe identifier, globally unique
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Member represents a user's membership in an organization.
type Member struct {
	OrgID    uuid.UUID
	UserID   uuid.UUID
	Role     Role
	JoinedAt time.Time
	// Denormalized user fields (populated by JOIN in queries).
	UserEmail    string
	UserFullName string
}

// Repository defines the data access contract for organization persistence.
type Repository interface {
	Create(ctx context.Context, org *Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	GetBySlug(ctx context.Context, slug string) (*Organization, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*Organization, error)
	Update(ctx context.Context, org *Organization) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Member management
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role Role) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*Member, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*Member, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role Role) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
}
