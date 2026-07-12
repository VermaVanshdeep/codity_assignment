// Package middleware provides HTTP middleware for the Fiber web framework.
package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Context keys — typed constants avoid collisions with other middleware.
const (
	keyUserID    = "ctx_user_id"
	keyUserEmail = "ctx_user_email"
	keyOrgID     = "ctx_org_id"
	keyProjectID = "ctx_project_id"
	keyOrgRole   = "ctx_org_role"
)

// SetUserID stores the authenticated user's ID in the request context.
func SetUserID(c *fiber.Ctx, id uuid.UUID) {
	c.Locals(keyUserID, id)
}

// GetUserID retrieves the authenticated user's ID from the request context.
// Panics if called outside of an authenticated route — always use after AuthRequired.
func GetUserID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(keyUserID).(uuid.UUID)
	return id
}

// SetUserEmail stores the authenticated user's email in the request context.
func SetUserEmail(c *fiber.Ctx, email string) {
	c.Locals(keyUserEmail, email)
}

// GetUserEmail retrieves the authenticated user's email from the request context.
func GetUserEmail(c *fiber.Ctx) string {
	email, _ := c.Locals(keyUserEmail).(string)
	return email
}

// SetOrgRole stores the requesting user's role in the current org.
func SetOrgRole(c *fiber.Ctx, role string) {
	c.Locals(keyOrgRole, role)
}

// GetOrgRole retrieves the requesting user's org role.
func GetOrgRole(c *fiber.Ctx) string {
	role, _ := c.Locals(keyOrgRole).(string)
	return role
}
