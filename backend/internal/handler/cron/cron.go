package cron

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	domaincron "github.com/your-org/job-scheduler/internal/domain/cron"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	cronsvc "github.com/your-org/job-scheduler/internal/service/cron"
	"github.com/your-org/job-scheduler/pkg/validator"
)

type Handler struct {
	svc      *cronsvc.Service
	validate *validator.Validator
}

func New(svc *cronsvc.Service, validate *validator.Validator) *Handler {
	return &Handler{svc: svc, validate: validate}
}

func (h *Handler) RegisterRoutes(queueGroup fiber.Router, cronGroup fiber.Router) {
	queueGroup.Post("/cron", h.Create)
	queueGroup.Get("/cron", h.List)
	cronGroup.Delete("/:cronId", h.Delete)
}

type CreateCronRequest struct {
	Name          string         `json:"name" validate:"required"`
	CronExpr      string         `json:"cron_expr" validate:"required"`
	JobType       string         `json:"job_type" validate:"required"`
	Payload       map[string]any `json:"payload" validate:"required"`
	MaxRetries    *int           `json:"max_retries" validate:"omitempty"`
	RetryStrategy *string        `json:"retry_strategy" validate:"omitempty"`
}

func (h *Handler) Create(c *fiber.Ctx) error {
	queueID, err := uuid.Parse(c.Params("queueId"))
	if err != nil {
		return apperrors.InvalidInput("invalid queue ID")
	}

	var req CreateCronRequest
	if err := h.validate.ParseAndValidate(c, &req); err != nil {
		return err
	}

	cj := &domaincron.CronJob{
		QueueID:  queueID,
		Name:     req.Name,
		CronExpr: req.CronExpr,
		JobType:  req.JobType,
		Payload:  req.Payload,
		IsActive: true,
		Timezone: "UTC",
	}

	if req.MaxRetries != nil {
		cj.MaxRetries = req.MaxRetries
	}
	if req.RetryStrategy != nil {
		cj.RetryStrategy = req.RetryStrategy
	}

	if err := h.svc.Create(c.Context(), cj); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(cj)
}

func (h *Handler) List(c *fiber.Ctx) error {
	queueID, err := uuid.Parse(c.Params("queueId"))
	if err != nil {
		return apperrors.InvalidInput("invalid queue ID")
	}

	crons, err := h.svc.ListByQueueID(c.Context(), queueID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": crons})
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	cronID, err := uuid.Parse(c.Params("cronId"))
	if err != nil {
		return apperrors.InvalidInput("invalid cron ID")
	}

	if err := h.svc.Delete(c.Context(), cronID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
