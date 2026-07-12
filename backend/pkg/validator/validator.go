// Package validator provides a centralized request validation layer
// using go-playground/validator with custom validation rules.
//
// Design: All HTTP handler validation flows through this package.
// Validation errors are translated into structured field-level detail objects
// that the frontend can use to highlight specific form fields.
package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
)

// Validator is a thin wrapper around the playground validator.
type Validator struct {
	v *validator.Validate
}

// FieldError describes a single validation failure on a request field.
type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

// New creates a Validator with custom rules registered.
func New() *Validator {
	v := validator.New()

	// Use JSON tag names in error messages instead of struct field names.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Register custom validation rules.
	_ = v.RegisterValidation("slug", validateSlug)
	_ = v.RegisterValidation("cron", validateCronExpr)

	return &Validator{v: v}
}

// Validate validates the given struct and returns a slice of FieldErrors.
// Returns nil if validation passes.
func (val *Validator) Validate(s any) []FieldError {
	err := val.v.Struct(s)
	if err == nil {
		return nil
	}

	var errs []FieldError
	for _, e := range err.(validator.ValidationErrors) {
		errs = append(errs, FieldError{
			Field:   e.Field(),
			Tag:     e.Tag(),
			Message: humanize(e),
		})
	}
	return errs
}

// ParseAndValidate parses the request body and validates it.
func (val *Validator) ParseAndValidate(c *fiber.Ctx, out any) error {
	if err := c.BodyParser(out); err != nil {
		return apperrors.InvalidInput("invalid JSON body")
	}
	if errs := val.Validate(out); errs != nil {
		return apperrors.InvalidInputWithDetails("validation failed", errs)
	}
	return nil
}

// humanize converts a validator.FieldError into a readable message.
func humanize(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", e.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
	case "slug":
		return fmt.Sprintf("%s must contain only lowercase letters, numbers, and hyphens", e.Field())
	case "cron":
		return fmt.Sprintf("%s must be a valid cron expression", e.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", e.Field(), e.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", e.Field(), e.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", e.Field(), e.Param())
	default:
		return fmt.Sprintf("%s failed validation: %s", e.Field(), e.Tag())
	}
}

// validateSlug checks that a value matches the slug pattern: lowercase alphanumeric + hyphens.
func validateSlug(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return s[0] != '-' && s[len(s)-1] != '-'
}

// validateCronExpr performs a basic cron expression sanity check.
// Full validation is delegated to the cron parser at scheduling time.
func validateCronExpr(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	parts := strings.Fields(s)
	// Accept 5-field (standard) or 6-field (with seconds) expressions.
	return len(parts) == 5 || len(parts) == 6
}
