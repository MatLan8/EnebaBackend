package requests

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"eneba-backend/internal/httpx"
)

// Created once: it caches struct metadata and is safe for concurrent use.
var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	// Report fields by their JSON name ("email") instead of the Go name ("Email").
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	return v
}

func Validate(req any) error {
	err := validate.Struct(req)
	if err == nil {
		return nil
	}

	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return fmt.Errorf("validate request: %w", err) // programming error, becomes a 500
	}

	details := make(map[string]string, len(fieldErrors))
	for _, fe := range fieldErrors {
		details[fe.Field()] = messageFor(fe)
	}
	return httpx.ValidationFailed(details)
}

func messageFor(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	default:
		return "is invalid"
	}
}
