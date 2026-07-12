// Package queue provides HTTP handlers for Queue management endpoints.
package queue

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/project"
	"github.com/your-org/job-scheduler/internal/middleware"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	queuesvc "github.com/your-org/job-scheduler/internal/service/queue"
	"github.com/your-org/job-scheduler/pkg/validator"
)

// Handler holds dependencies for queue HTTP handlers.
type Handler struct {
	svc       queuesvc.Service
	validator *validator.Validator
}

// New creates a new queue Handler.
func New(svc queuesvc.Service, v *validator.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// RegisterRoutes mounts queue routes under /projects/:projectId/queues.
func (h *Handler) RegisterRoutes(r fiber.Router, projRepo middleware.ProjectMemberReader) {
	// List and create — require project role.
	r.Get("/",
		middleware.RequireProjectRole(projRepo, project.RoleViewer),
		h.List,
	)
	r.Post("/",
		middleware.RequireProjectRole(projRepo, project.RoleAdmin),
		h.Create,
	)

	// Specific queue routes.
	q := r.Group("/:queueId",
		middleware.RequireProjectRole(projRepo, project.RoleViewer),
	)
	q.Get("/", h.Get)
	q.Get("/stats", h.GetStats)

	qDev := r.Group("/:queueId",
		middleware.RequireProjectRole(projRepo, project.RoleDeveloper),
	)
	qDev.Put("/", h.Update)
	qDev.Post("/pause", h.Pause)
	qDev.Post("/resume", h.Resume)

	qAdmin := r.Group("/:queueId",
		middleware.RequireProjectRole(projRepo, project.RoleAdmin),
	)
	qAdmin.Delete("/", h.Delete)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	projectID, err := parseUUID(c, "projectId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	var req queuesvc.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}
	resp, err := h.svc.Create(c.Context(), projectID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": resp})
}

func (h *Handler) Get(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.GetByID(c.Context(), queueID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

func (h *Handler) List(c *fiber.Ctx) error {
	projectID, err := parseUUID(c, "projectId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.ListByProject(c.Context(), projectID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

func (h *Handler) Update(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	var req queuesvc.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}
	resp, err := h.svc.Update(c.Context(), queueID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.Delete(c.Context(), queueID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Pause(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.Pause(c.Context(), queueID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Resume(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.Resume(c.Context(), queueID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) GetStats(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.GetStats(c.Context(), queueID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

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
