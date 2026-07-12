package execlog

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	execlogsvc "github.com/your-org/job-scheduler/internal/service/execlog"
)

type Handler struct {
	svc *execlogsvc.Service
}

func New(svc *execlogsvc.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(jobGroup fiber.Router) {
	jobGroup.Get("/:jobId/executions", h.ListExecutions)
	jobGroup.Get("/:jobId/logs", h.ListLogs)
}

func (h *Handler) ListExecutions(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return apperrors.InvalidInput("invalid job ID")
	}

	execs, err := h.svc.ListExecutions(c.Context(), jobID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": execs})
}

func (h *Handler) ListLogs(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return apperrors.InvalidInput("invalid job ID")
	}

	logs, err := h.svc.ListLogs(c.Context(), jobID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": logs})
}
