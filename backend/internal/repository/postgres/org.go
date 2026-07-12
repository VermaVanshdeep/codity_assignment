package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-org/job-scheduler/internal/domain/org"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
)

// OrgRepo implements org.Repository using PostgreSQL.
type OrgRepo struct {
	db *pgxpool.Pool
}

// NewOrgRepo creates a new OrgRepo.
func NewOrgRepo(db *pgxpool.Pool) *OrgRepo {
	return &OrgRepo{db: db}
}

// Create inserts a new organization.
func (r *OrgRepo) Create(ctx context.Context, o *org.Organization) error {
	const q = `
		INSERT INTO organizations (id, name, slug, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.Exec(ctx, q, o.ID, o.Name, o.Slug, o.Description, o.CreatedAt, o.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.AlreadyExists("organization slug")
		}
		return fmt.Errorf("create org: %w", err)
	}
	return nil
}

// GetByID retrieves an organization by its primary key.
func (r *OrgRepo) GetByID(ctx context.Context, id uuid.UUID) (*org.Organization, error) {
	const q = `
		SELECT id, name, slug, description, created_at, updated_at
		FROM organizations WHERE id = $1`

	o, err := scanOrg(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("organization")
		}
		return nil, fmt.Errorf("get org by id: %w", err)
	}
	return o, nil
}

// GetBySlug retrieves an organization by its URL slug.
func (r *OrgRepo) GetBySlug(ctx context.Context, slug string) (*org.Organization, error) {
	const q = `
		SELECT id, name, slug, description, created_at, updated_at
		FROM organizations WHERE slug = $1`

	o, err := scanOrg(r.db.QueryRow(ctx, q, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("organization")
		}
		return nil, fmt.Errorf("get org by slug: %w", err)
	}
	return o, nil
}

// ListByUserID returns all organizations a user belongs to.
func (r *OrgRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*org.Organization, error) {
	const q = `
		SELECT o.id, o.name, o.slug, o.description, o.created_at, o.updated_at
		FROM organizations o
		JOIN org_members om ON o.id = om.org_id
		WHERE om.user_id = $1
		ORDER BY o.name`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list orgs by user: %w", err)
	}
	defer rows.Close()

	var orgs []*org.Organization
	for rows.Next() {
		o, err := scanOrg(rows)
		if err != nil {
			return nil, fmt.Errorf("scan org row: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// Update persists changes to an organization.
func (r *OrgRepo) Update(ctx context.Context, o *org.Organization) error {
	const q = `
		UPDATE organizations SET name = $2, description = $3, updated_at = $4
		WHERE id = $1`

	tag, err := r.db.Exec(ctx, q, o.ID, o.Name, o.Description, o.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update org: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("organization")
	}
	return nil
}

// Delete removes an organization (cascades to all projects/queues/jobs via FK).
func (r *OrgRepo) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM organizations WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete org: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("organization")
	}
	return nil
}

// AddMember inserts a user into an organization with the given role.
func (r *OrgRepo) AddMember(ctx context.Context, orgID, userID uuid.UUID, role org.Role) error {
	const q = `
		INSERT INTO org_members (org_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`

	_, err := r.db.Exec(ctx, q, orgID, userID, string(role))
	if err != nil {
		if isForeignKeyViolation(err) {
			return apperrors.NotFound("user")
		}
		return fmt.Errorf("add org member: %w", err)
	}
	return nil
}

// GetMember retrieves a single member's membership record.
func (r *OrgRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*org.Member, error) {
	const q = `
		SELECT om.org_id, om.user_id, om.role, om.joined_at,
		       u.email, u.full_name
		FROM org_members om
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1 AND om.user_id = $2`

	m, err := scanMember(r.db.QueryRow(ctx, q, orgID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("organization member")
		}
		return nil, fmt.Errorf("get org member: %w", err)
	}
	return m, nil
}

// ListMembers returns all members of an organization.
func (r *OrgRepo) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*org.Member, error) {
	const q = `
		SELECT om.org_id, om.user_id, om.role, om.joined_at,
		       u.email, u.full_name
		FROM org_members om
		JOIN users u ON u.id = om.user_id
		WHERE om.org_id = $1
		ORDER BY om.joined_at`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list org members: %w", err)
	}
	defer rows.Close()

	var members []*org.Member
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// UpdateMemberRole changes the role of an existing member.
func (r *OrgRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role org.Role) error {
	const q = `
		UPDATE org_members SET role = $3
		WHERE org_id = $1 AND user_id = $2`

	tag, err := r.db.Exec(ctx, q, orgID, userID, string(role))
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("organization member")
	}
	return nil
}

// RemoveMember removes a user from an organization.
func (r *OrgRepo) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	const q = `DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, orgID, userID)
	if err != nil {
		return fmt.Errorf("remove org member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("organization member")
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanOrg(row scanner) (*org.Organization, error) {
	var o org.Organization
	err := row.Scan(&o.ID, &o.Name, &o.Slug, &o.Description, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func scanMember(row scanner) (*org.Member, error) {
	var m org.Member
	var role string
	err := row.Scan(&m.OrgID, &m.UserID, &role, &m.JoinedAt, &m.UserEmail, &m.UserFullName)
	if err != nil {
		return nil, err
	}
	m.Role = org.Role(role)
	return &m, nil
}
