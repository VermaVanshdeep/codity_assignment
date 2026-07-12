// Package org provides HTTP handlers for Organization management endpoints.
package org

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	domainorg "github.com/your-org/job-scheduler/internal/domain/org"
	"github.com/your-org/job-scheduler/internal/middleware"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	orgsvc "github.com/your-org/job-scheduler/internal/service/org"
	"github.com/your-org/job-scheduler/pkg/validator"
)

// Handler holds dependencies for org HTTP handlers.
type Handler struct {
	svc       orgsvc.Service
	validator *validator.Validator
}

// New creates a new org Handler.
func New(svc orgsvc.Service, v *validator.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// RegisterRoutes mounts org routes onto the given Fiber router group.
// The group is expected to already have AuthRequired applied.
func (h *Handler) RegisterRoutes(r fiber.Router, orgRepo middleware.OrgMemberReader) {
	// Public to any authenticated user.
	r.Post("/", h.Create)
	r.Get("/", h.List)

	// All /:orgId routes pass through the org-scope middleware.
	orgGroup := r.Group("/:orgId")

	// Read-only: member role sufficient.
	orgGroup.Get("/",
		middleware.RequireOrgRole(orgRepo, domainorg.RoleMember),
		h.Get,
	)
	orgGroup.Get("/members",
		middleware.RequireOrgRole(orgRepo, domainorg.RoleMember),
		h.ListMembers,
	)

	// Mutations: admin role required.
	orgGroup.Put("/",
		middleware.RequireOrgRole(orgRepo, domainorg.RoleAdmin),
		h.Update,
	)
	orgGroup.Post("/members",
		middleware.RequireOrgRole(orgRepo, domainorg.RoleAdmin),
		h.AddMember,
	)
	orgGroup.Put("/members/:userId/role",
		middleware.RequireOrgRole(orgRepo, domainorg.RoleAdmin),
		h.UpdateMemberRole,
	)
	orgGroup.Delete("/members/:userId",
		middleware.RequireOrgRole(orgRepo, domainorg.RoleAdmin),
		h.RemoveMember,
	)

	// Destructive: owner only.
	orgGroup.Delete("/",
		middleware.RequireOrgRole(orgRepo, domainorg.RoleOwner),
		h.Delete,
	)
}

// Create creates a new organization for the authenticated user.
func (h *Handler) Create(c *fiber.Ctx) error {
	var req orgsvc.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}
	userID := middleware.GetUserID(c)
	resp, err := h.svc.Create(c.Context(), userID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": resp})
}

// Get retrieves a single organization.
func (h *Handler) Get(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.GetByID(c.Context(), orgID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// List returns all organizations the authenticated user belongs to.
func (h *Handler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	resp, err := h.svc.ListForUser(c.Context(), userID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// Update modifies an organization's mutable fields.
func (h *Handler) Update(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	var req orgsvc.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}
	resp, err := h.svc.Update(c.Context(), orgID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// Delete permanently deletes an organization.
func (h *Handler) Delete(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.Delete(c.Context(), orgID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AddMember adds a user to the organization.
func (h *Handler) AddMember(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	var req orgsvc.AddMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}
	resp, err := h.svc.AddMember(c.Context(), orgID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": resp})
}

// UpdateMemberRole updates a member's role in the organization.
func (h *Handler) UpdateMemberRole(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	userID, err := parseUUID(c, "userId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	var req struct {
		Role domainorg.Role `json:"role" validate:"required,oneof=owner admin member"`
	}
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}
	resp, err := h.svc.UpdateMemberRole(c.Context(), orgID, userID, req.Role)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// RemoveMember removes a user from the organization.
func (h *Handler) RemoveMember(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	userID, err := parseUUID(c, "userId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.RemoveMember(c.Context(), orgID, userID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListMembers lists all members of the organization.
func (h *Handler) ListMembers(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.ListMembers(c.Context(), orgID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func parseUUID(c *fiber.Ctx, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(param))
	if err != nil {
		return uuid.Nil, apperrors.InvalidInput(param + " must be a valid UUID")
	}
	return id, nil
}

func validationError(c *fiber.Ctx, details any) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    apperrors.CodeInvalidInput,
			"message": "validation failed",
			"details": details,
		},
	})
}
