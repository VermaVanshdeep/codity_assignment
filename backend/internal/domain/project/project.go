// Package project defines the Project domain entity and its repository contract.
// A Project belongs to an Organization and is the grouping unit for queues and jobs.
// Each project has an API key that external systems use to enqueue jobs programmatically.
package project

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Role represents the level of access a user has within a project.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleViewer    Role = "viewer"
)

// CanManageQueues returns true if the role can create/modify queues.
func (r Role) CanManageQueues() bool {
	return r == RoleAdmin || r == RoleDeveloper
}

// CanEnqueueJobs returns true if the role can submit jobs.
func (r Role) CanEnqueueJobs() bool {
	return r == RoleAdmin || r == RoleDeveloper
}

// CanView returns true if the role can read project data.
func (r Role) CanView() bool {
	return true // all roles can view
}

// Project is a logical grouping of queues within an organization.
type Project struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Name         string
	Slug         string // unique within the org
	Description  string
	APIKeyHash   string // SHA-256 of the raw API key
	APIKeyPrefix string // first 12 chars shown in UI (non-secret)
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Member represents a user's membership in a project.
type Member struct {
	ProjectID    uuid.UUID
	UserID       uuid.UUID
	Role         Role
	JoinedAt     time.Time
	UserEmail    string
	UserFullName string
}

// Repository defines the data access contract for project persistence.
type Repository interface {
	Create(ctx context.Context, p *Project) error
	GetByID(ctx context.Context, id uuid.UUID) (*Project, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error)
	GetByAPIKeyHash(ctx context.Context, hash string) (*Project, error)
	ListByOrgID(ctx context.Context, orgID uuid.UUID) ([]*Project, error)
	Update(ctx context.Context, p *Project) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Member management
	AddMember(ctx context.Context, projectID, userID uuid.UUID, role Role) error
	GetMember(ctx context.Context, projectID, userID uuid.UUID) (*Member, error)
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]*Member, error)
	UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role Role) error
	RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error
}
