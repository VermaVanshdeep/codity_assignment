package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-org/job-scheduler/internal/domain/project"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
)

// ProjectRepo implements project.Repository using PostgreSQL.
type ProjectRepo struct {
	db *pgxpool.Pool
}

// NewProjectRepo creates a new ProjectRepo.
func NewProjectRepo(db *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create inserts a new project record.
func (r *ProjectRepo) Create(ctx context.Context, p *project.Project) error {
	const q = `
		INSERT INTO projects (id, org_id, name, slug, description, api_key_hash, api_key_prefix, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.db.Exec(ctx, q,
		p.ID, p.OrgID, p.Name, p.Slug, p.Description,
		p.APIKeyHash, p.APIKeyPrefix, p.IsActive, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.AlreadyExists("project slug")
		}
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// GetByID retrieves a project by its primary key.
func (r *ProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*project.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, description, api_key_hash, api_key_prefix, is_active, created_at, updated_at
		FROM projects WHERE id = $1`

	p, err := scanProject(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("project")
		}
		return nil, fmt.Errorf("get project by id: %w", err)
	}
	return p, nil
}

// GetBySlug retrieves a project by org_id + slug (unique together).
func (r *ProjectRepo) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*project.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, description, api_key_hash, api_key_prefix, is_active, created_at, updated_at
		FROM projects WHERE org_id = $1 AND slug = $2`

	p, err := scanProject(r.db.QueryRow(ctx, q, orgID, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("project")
		}
		return nil, fmt.Errorf("get project by slug: %w", err)
	}
	return p, nil
}

// GetByAPIKeyHash retrieves a project by its hashed API key.
// Used by the API key authentication middleware.
func (r *ProjectRepo) GetByAPIKeyHash(ctx context.Context, hash string) (*project.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, description, api_key_hash, api_key_prefix, is_active, created_at, updated_at
		FROM projects WHERE api_key_hash = $1 AND is_active = true`

	p, err := scanProject(r.db.QueryRow(ctx, q, hash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Unauthorized("invalid API key")
		}
		return nil, fmt.Errorf("get project by api key: %w", err)
	}
	return p, nil
}

// ListByOrgID returns all projects belonging to an organization.
func (r *ProjectRepo) ListByOrgID(ctx context.Context, orgID uuid.UUID) ([]*project.Project, error) {
	const q = `
		SELECT id, org_id, name, slug, description, api_key_hash, api_key_prefix, is_active, created_at, updated_at
		FROM projects WHERE org_id = $1
		ORDER BY name`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list projects by org: %w", err)
	}
	defer rows.Close()

	var projects []*project.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Update persists changes to a project.
func (r *ProjectRepo) Update(ctx context.Context, p *project.Project) error {
	const q = `
		UPDATE projects
		SET name = $2, description = $3, api_key_hash = $4, api_key_prefix = $5,
		    is_active = $6, updated_at = $7
		WHERE id = $1`

	tag, err := r.db.Exec(ctx, q,
		p.ID, p.Name, p.Description, p.APIKeyHash, p.APIKeyPrefix,
		p.IsActive, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("project")
	}
	return nil
}

// Delete removes a project (cascades to queues/jobs via FK).
func (r *ProjectRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM projects WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("project")
	}
	return nil
}

// AddMember adds a user to a project with the given role.
func (r *ProjectRepo) AddMember(ctx context.Context, projectID, userID uuid.UUID, role project.Role) error {
	const q = `
		INSERT INTO project_members (project_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role`

	_, err := r.db.Exec(ctx, q, projectID, userID, string(role))
	if err != nil {
		return fmt.Errorf("add project member: %w", err)
	}
	return nil
}

// GetMember retrieves a user's membership in a project.
func (r *ProjectRepo) GetMember(ctx context.Context, projectID, userID uuid.UUID) (*project.Member, error) {
	const q = `
		SELECT pm.project_id, pm.user_id, pm.role, pm.joined_at, u.email, u.full_name
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1 AND pm.user_id = $2`

	var m project.Member
	var role string
	err := r.db.QueryRow(ctx, q, projectID, userID).Scan(
		&m.ProjectID, &m.UserID, &role, &m.JoinedAt, &m.UserEmail, &m.UserFullName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("project member")
		}
		return nil, fmt.Errorf("get project member: %w", err)
	}
	m.Role = project.Role(role)
	return &m, nil
}

// ListMembers returns all members of a project.
func (r *ProjectRepo) ListMembers(ctx context.Context, projectID uuid.UUID) ([]*project.Member, error) {
	const q = `
		SELECT pm.project_id, pm.user_id, pm.role, pm.joined_at, u.email, u.full_name
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1 ORDER BY pm.joined_at`

	rows, err := r.db.Query(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()

	var members []*project.Member
	for rows.Next() {
		var m project.Member
		var role string
		if err := rows.Scan(&m.ProjectID, &m.UserID, &role, &m.JoinedAt, &m.UserEmail, &m.UserFullName); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		m.Role = project.Role(role)
		members = append(members, &m)
	}
	return members, rows.Err()
}

// UpdateMemberRole changes the role of an existing project member.
func (r *ProjectRepo) UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role project.Role) error {
	const q = `UPDATE project_members SET role = $3 WHERE project_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, projectID, userID, string(role))
	if err != nil {
		return fmt.Errorf("update project member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("project member")
	}
	return nil
}

// RemoveMember removes a user from a project.
func (r *ProjectRepo) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	const q = `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, projectID, userID)
	if err != nil {
		return fmt.Errorf("remove project member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("project member")
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func scanProject(row scanner) (*project.Project, error) {
	var p project.Project
	err := row.Scan(
		&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.Description,
		&p.APIKeyHash, &p.APIKeyPrefix, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
