package batch

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/job"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	batchsvc "github.com/your-org/job-scheduler/internal/service/batch"
	"github.com/your-org/job-scheduler/pkg/validator"
)

type Handler struct {
	svc      *batchsvc.Service
	validate *validator.Validator
}

func New(svc *batchsvc.Service, validate *validator.Validator) *Handler {
	return &Handler{svc: svc, validate: validate}
}

func (h *Handler) RegisterRoutes(queueGroup fiber.Router, batchGroup fiber.Router) {
	queueGroup.Post("/batches", h.CreateBatch)
	batchGroup.Get("/:batchId", h.GetBatch)
}

type CreateBatchRequest struct {
	Name string        `json:"name" validate:"required"`
	Jobs []JobSpecBody `json:"jobs" validate:"required,min=1"`
}

type JobSpecBody struct {
	Type           string         `json:"type" validate:"required"`
	Payload        map[string]any `json:"payload" validate:"required"`
	Priority       int            `json:"priority" validate:"omitempty,min=1,max=10"`
	MaxRetries     *int           `json:"max_retries" validate:"omitempty,min=0"`
	IdempotencyKey string         `json:"idempotency_key" validate:"omitempty"`
}

func (h *Handler) CreateBatch(c *fiber.Ctx) error {
	queueID, err := uuid.Parse(c.Params("queueId"))
	if err != nil {
		return apperrors.InvalidInput("invalid queue ID")
	}

	var req CreateBatchRequest
	if err := h.validate.ParseAndValidate(c, &req); err != nil {
		return err
	}

	specs := make([]job.Job, len(req.Jobs))
	for i, jreq := range req.Jobs {
		specs[i] = job.Job{
			Type:           jreq.Type,
			Payload:        jreq.Payload,
			IdempotencyKey: nil,
		}
		if jreq.Priority != 0 {
			specs[i].Priority = jreq.Priority
		} else {
			specs[i].Priority = 5
		}
		if jreq.MaxRetries != nil {
			specs[i].MaxRetries = *jreq.MaxRetries
		} else {
			specs[i].MaxRetries = 3
		}
		if jreq.IdempotencyKey != "" {
			ik := jreq.IdempotencyKey
			specs[i].IdempotencyKey = &ik
		}
	}

	b, err := h.svc.CreateBatch(c.Context(), queueID, req.Name, specs)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(b)
}

func (h *Handler) GetBatch(c *fiber.Ctx) error {
	batchID, err := uuid.Parse(c.Params("batchId"))
	if err != nil {
		return apperrors.InvalidInput("invalid batch ID")
	}

	b, err := h.svc.Get(c.Context(), batchID)
	if err != nil {
		return err
	}
	return c.JSON(b)
}
