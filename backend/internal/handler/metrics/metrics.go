package metrics

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	metricssvc "github.com/your-org/job-scheduler/internal/service/metrics"
)

type Handler struct {
	svc *metricssvc.Service
}

func New(svc *metricssvc.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/queues/:queueId", h.GetQueueMetrics)
	router.Get("/system", h.GetSystemMetrics)
}

func (h *Handler) GetQueueMetrics(c *fiber.Ctx) error {
	queueID, err := uuid.Parse(c.Params("queueId"))
	if err != nil {
		return apperrors.InvalidInput("invalid queue ID")
	}

	sinceStr := c.Query("since")
	var since time.Time
	if sinceStr != "" {
		parsed, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return apperrors.InvalidInput("invalid since format (use RFC3339)")
		}
		since = parsed
	} else {
		// default to last 24 hours
		since = time.Now().Add(-24 * time.Hour)
	}

	snaps, err := h.svc.GetQueueMetrics(c.Context(), queueID, since)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": snaps})
}

func (h *Handler) GetSystemMetrics(c *fiber.Ctx) error {
	sys, err := h.svc.GetSystemMetrics(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"data": sys})
}
