// Package auth provides HTTP handlers for authentication endpoints.
// Handlers are thin: they parse input, validate it, call the service, and return JSON.
// All business logic lives in the service layer.
package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/your-org/job-scheduler/internal/middleware"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	authsvc "github.com/your-org/job-scheduler/internal/service/auth"
	"github.com/your-org/job-scheduler/pkg/validator"
)

// Handler holds the dependencies for auth HTTP handlers.
type Handler struct {
	svc       authsvc.Service
	validator *validator.Validator
}

// New creates a new auth Handler.
func New(svc authsvc.Service, v *validator.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// RegisterRoutes mounts all auth routes onto the given Fiber router group.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
	r.Delete("/logout", h.Logout)
}

// Register godoc
// @Summary Register a new user account
// @Tags    auth
// @Accept  json
// @Produce json
// @Param   body body authsvc.RegisterRequest true "Registration data"
// @Success 201 {object} authsvc.UserDTO
// @Router  /api/v1/auth/register [post]
func (h *Handler) Register(c *fiber.Ctx) error {
	var req authsvc.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("request body is malformed JSON"))
	}

	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    apperrors.CodeInvalidInput,
				"message": "validation failed",
				"details": fieldErrs,
			},
		})
	}

	user, err := h.svc.Register(c.Context(), req)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": user})
}

// Login godoc
// @Summary Authenticate a user and obtain JWT tokens
// @Tags    auth
// @Accept  json
// @Produce json
// @Param   body body authsvc.LoginRequest true "Credentials"
// @Success 200 {object} authsvc.LoginResponse
// @Router  /api/v1/auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
	var req authsvc.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("request body is malformed JSON"))
	}

	if fieldErrs := h.validator.Validate(req); fieldErrs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    apperrors.CodeInvalidInput,
				"message": "validation failed",
				"details": fieldErrs,
			},
		})
	}

	userAgent := c.Get("User-Agent")
	ip := c.IP()

	resp, err := h.svc.Login(c.Context(), req, userAgent, ip)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": resp})
}

// Refresh godoc
// @Summary Exchange a refresh token for a new access token
// @Tags    auth
// @Accept  json
// @Produce json
// @Param   body body refreshRequest true "Refresh token"
// @Success 200 {object} authsvc.RefreshResponse
// @Router  /api/v1/auth/refresh [post]
func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req refreshRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("request body is malformed JSON"))
	}

	if req.RefreshToken == "" {
		return middleware.RespondError(c, apperrors.InvalidInput("refresh_token is required"))
	}

	resp, err := h.svc.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		return middleware.RespondError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": resp})
}

// Logout godoc
// @Summary Revoke the current refresh token (single-device logout)
// @Tags    auth
// @Accept  json
// @Produce json
// @Param   body body refreshRequest true "Refresh token to revoke"
// @Success 204
// @Router  /api/v1/auth/logout [delete]
func (h *Handler) Logout(c *fiber.Ctx) error {
	var req refreshRequest
	if err := c.BodyParser(&req); err != nil {
		return middleware.RespondError(c, apperrors.InvalidInput("request body is malformed JSON"))
	}

	// Logout is idempotent — we don't fail if the token is already invalid.
	_ = h.svc.Logout(c.Context(), req.RefreshToken)

	return c.SendStatus(fiber.StatusNoContent)
}

// refreshRequest is the shared request body for refresh and logout endpoints.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}
