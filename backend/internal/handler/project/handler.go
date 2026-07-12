// Package project provides HTTP handlers for Project management endpoints.
package project

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/org"
	"github.com/your-org/job-scheduler/internal/domain/project"
	"github.com/your-org/job-scheduler/internal/middleware"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	projectsvc "github.com/your-org/job-scheduler/internal/service/project"
	"github.com/your-org/job-scheduler/pkg/validator"
)

// Handler holds dependencies for project HTTP handlers.
type Handler struct {
	svc       projectsvc.Service
	validator *validator.Validator
}

// New creates a new project Handler.
func New(svc projectsvc.Service, v *validator.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// RegisterRoutes mounts project routes under /orgs/:orgId/projects.
func (h *Handler) RegisterRoutes(r fiber.Router, orgRepo middleware.OrgMemberReader, projRepo middleware.ProjectMemberReader) {
	// List and create — requires org membership.
	r.Get("/",
		middleware.RequireOrgRole(orgRepo, org.RoleMember),
		h.List,
	)
	r.Post("/",
		middleware.RequireOrgRole(orgRepo, org.RoleAdmin),
		h.Create,
	)

	// Project-specific routes — requires project membership.
	proj := r.Group("/:projectId",
		middleware.RequireProjectRole(projRepo, project.RoleViewer),
	)
	proj.Get("/", h.Get)

	projDev := r.Group("/:projectId",
		middleware.RequireProjectRole(projRepo, project.RoleDeveloper),
	)
	projDev.Put("/", h.Update)

	projAdmin := r.Group("/:projectId",
		middleware.RequireProjectRole(projRepo, project.RoleAdmin),
	)
	projAdmin.Delete("/", h.Delete)
	projAdmin.Post("/rotate-key", h.RotateAPIKey)
}

// Create godoc
// @Summary Create a new project within an organization
func (h *Handler) Create(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}

	var req projectsvc.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}

	userID := middleware.GetUserID(c)
	proj, apiKey, err := h.svc.Create(c.Context(), orgID, userID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	// Return both the project and the API key (shown once).
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"project": proj,
			"api_key": apiKey,
		},
	})
}

// Get godoc
// @Summary Get a project by ID
func (h *Handler) Get(c *fiber.Ctx) error {
	projectID, err := parseUUID(c, "projectId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.GetByID(c.Context(), projectID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// List godoc
// @Summary List all projects in an organization
func (h *Handler) List(c *fiber.Ctx) error {
	orgID, err := parseUUID(c, "orgId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.ListByOrg(c.Context(), orgID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// Update godoc
// @Summary Update a project's name/description
func (h *Handler) Update(c *fiber.Ctx) error {
	projectID, err := parseUUID(c, "projectId")
	if err != nil {
		return middleware.RespondError(c, err)
	}

	var req projectsvc.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}

	resp, err := h.svc.Update(c.Context(), projectID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

// Delete godoc
// @Summary Delete a project (admin only)
func (h *Handler) Delete(c *fiber.Ctx) error {
	projectID, err := parseUUID(c, "projectId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.Delete(c.Context(), projectID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RotateAPIKey godoc
// @Summary Rotate the project's API key — old key is immediately invalidated
func (h *Handler) RotateAPIKey(c *fiber.Ctx) error {
	projectID, err := parseUUID(c, "projectId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	apiKey, err := h.svc.RotateAPIKey(c.Context(), projectID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": apiKey})
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
