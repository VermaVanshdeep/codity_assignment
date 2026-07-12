package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/org"
	"github.com/your-org/job-scheduler/internal/domain/project"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	authsvc "github.com/your-org/job-scheduler/internal/service/auth"
)

// ─── JWT Authentication Middleware ────────────────────────────────────────────

// AuthRequired is a Fiber middleware that enforces JWT authentication.
// It reads the Bearer token from the Authorization header, validates it via
// the auth service, and injects the user ID and email into request context.
//
// Design: We inject the full auth.Service rather than a raw secret so that
// validation logic stays in one place and is testable without a real HTTP stack.
func AuthRequired(authService authsvc.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token, err := extractBearerToken(c)
		if err != nil {
			return RespondError(c, err)
		}

		claims, err := authService.ValidateAccessToken(token)
		if err != nil {
			return RespondError(c, apperrors.Unauthorized("invalid or expired token"))
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			return RespondError(c, apperrors.Unauthorized("malformed token subject"))
		}

		SetUserID(c, userID)
		SetUserEmail(c, claims.Email)

		return c.Next()
	}
}

// ─── Org RBAC Middleware ───────────────────────────────────────────────────────

// OrgMemberReader is the minimal repo interface needed by RequireOrgRole.
// Defining a narrow interface here prevents import cycles with the postgres package.
type OrgMemberReader interface {
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*org.Member, error)
}

// RequireOrgRole returns a middleware that verifies the requesting user holds at
// least the specified role within the org identified by :orgId in the URL.
// Must be chained after AuthRequired.
func RequireOrgRole(orgRepo OrgMemberReader, minRole org.Role) fiber.Handler {
	hierarchy := map[org.Role]int{
		org.RoleOwner:  3,
		org.RoleAdmin:  2,
		org.RoleMember: 1,
	}

	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)

		orgID, err := parseUUIDParam(c, "orgId")
		if err != nil {
			return RespondError(c, err)
		}

		member, err := orgRepo.GetMember(c.Context(), orgID, userID)
		if err != nil {
			return RespondError(c, apperrors.Forbidden("you are not a member of this organization"))
		}

		if hierarchy[member.Role] < hierarchy[minRole] {
			return RespondError(c, apperrors.Forbidden("insufficient organization role"))
		}

		SetOrgRole(c, string(member.Role))
		return c.Next()
	}
}

// ─── Project RBAC Middleware ──────────────────────────────────────────────────

// ProjectMemberReader is the minimal repo interface needed by RequireProjectRole.
type ProjectMemberReader interface {
	GetMember(ctx context.Context, projectID, userID uuid.UUID) (*project.Member, error)
}

// RequireProjectRole returns a middleware that verifies the requesting user holds
// at least the specified role within the project identified by :projectId.
// Must be chained after AuthRequired.
func RequireProjectRole(projRepo ProjectMemberReader, minRole project.Role) fiber.Handler {
	hierarchy := map[project.Role]int{
		project.RoleAdmin:     3,
		project.RoleDeveloper: 2,
		project.RoleViewer:    1,
	}

	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)

		projectID, err := parseUUIDParam(c, "projectId")
		if err != nil {
			return RespondError(c, err)
		}

		member, err := projRepo.GetMember(c.Context(), projectID, userID)
		if err != nil {
			return RespondError(c, apperrors.Forbidden("you are not a member of this project"))
		}

		if hierarchy[member.Role] < hierarchy[minRole] {
			return RespondError(c, apperrors.Forbidden("insufficient project role"))
		}

		return c.Next()
	}
}

// ─── Request Logging Middleware ────────────────────────────────────────────────

// Logger is a thin Fiber middleware that logs every request using the application logger.
// It logs method, path, status, and latency for observability.

// ─── Private helpers ──────────────────────────────────────────────────────────

// extractBearerToken parses "Bearer <token>" from the Authorization header.
func extractBearerToken(c *fiber.Ctx) (string, error) {
	header := c.Get("Authorization")
	if header == "" {
		return "", apperrors.Unauthorized("authorization header missing")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", apperrors.Unauthorized("authorization header must be 'Bearer <token>'")
	}
	if parts[1] == "" {
		return "", apperrors.Unauthorized("token is empty")
	}
	return parts[1], nil
}

// parseUUIDParam parses a URL path parameter as a UUID.
func parseUUIDParam(c *fiber.Ctx, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(param))
	if err != nil {
		return uuid.Nil, apperrors.InvalidInput(param + " must be a valid UUID")
	}
	return id, nil
}

// RespondError is the canonical error serializer used by all middleware and handlers.
// It translates AppErrors to structured JSON responses.
func RespondError(c *fiber.Ctx, err error) error {
	ae, ok := apperrors.As(err)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    "INTERNAL",
				"message": "an unexpected error occurred",
			},
		})
	}
	return c.Status(ae.HTTPStatus()).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    ae.Code,
			"message": ae.Message,
			"details": ae.Details,
		},
	})
}
