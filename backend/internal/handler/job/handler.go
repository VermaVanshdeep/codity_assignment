// Package job provides HTTP handlers for Job management endpoints.
package job

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/domain/project"
	"github.com/your-org/job-scheduler/internal/middleware"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	jobsvc "github.com/your-org/job-scheduler/internal/service/job"
	"github.com/your-org/job-scheduler/pkg/validator"
)

// Handler holds dependencies for job HTTP handlers.
type Handler struct {
	svc       jobsvc.Service
	validator *validator.Validator
}

// New creates a new job Handler.
func New(svc jobsvc.Service, v *validator.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// RegisterRoutes mounts job routes under /projects/:projectId/queues/:queueId/jobs.
// These routes require project membership.
func (h *Handler) RegisterRoutes(r fiber.Router, projRepo middleware.ProjectMemberReader) {
	// List and create.
	r.Get("/",
		middleware.RequireProjectRole(projRepo, project.RoleViewer),
		h.List,
	)
	r.Post("/",
		middleware.RequireProjectRole(projRepo, project.RoleDeveloper),
		h.Enqueue,
	)

	// Specific job routes.
	j := r.Group("/:jobId",
		middleware.RequireProjectRole(projRepo, project.RoleViewer),
	)
	j.Get("/", h.Get)

	jDev := r.Group("/:jobId",
		middleware.RequireProjectRole(projRepo, project.RoleDeveloper),
	)
	jDev.Post("/cancel", h.Cancel)
	jDev.Post("/retry", h.Retry)
}

func (h *Handler) Enqueue(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}

	var req jobsvc.EnqueueRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("malformed JSON"))
	}
	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return validationError(c, fieldErrs)
	}

	resp, err := h.svc.Enqueue(c.Context(), queueID, req)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": resp})
}

func (h *Handler) Get(c *fiber.Ctx) error {
	jobID, err := parseUUID(c, "jobId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	resp, err := h.svc.GetByID(c.Context(), jobID)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

func (h *Handler) List(c *fiber.Ctx) error {
	queueID, err := parseUUID(c, "queueId")
	if err != nil {
		return middleware.RespondError(c, err)
	}

	// Parse query params.
	filter := job.ListFilter{Limit: 50, Offset: 0}
	if l := c.QueryInt("limit", 50); l > 0 && l <= 200 {
		filter.Limit = l
	}
	if o := c.QueryInt("offset", 0); o >= 0 {
		filter.Offset = o
	}
	if s := c.Query("status"); s != "" {
		st := job.Status(s)
		filter.Status = &st
	}
	if t := c.Query("type"); t != "" {
		filter.Type = &t
	}

	resp, err := h.svc.List(c.Context(), queueID, filter)
	if err != nil {
		return middleware.RespondError(c, err)
	}
	return c.JSON(fiber.Map{"data": resp})
}

func (h *Handler) Cancel(c *fiber.Ctx) error {
	jobID, err := parseUUID(c, "jobId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.Cancel(c.Context(), jobID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Retry(c *fiber.Ctx) error {
	jobID, err := parseUUID(c, "jobId")
	if err != nil {
		return middleware.RespondError(c, err)
	}
	if err := h.svc.Retry(c.Context(), jobID); err != nil {
		return middleware.RespondError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
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
