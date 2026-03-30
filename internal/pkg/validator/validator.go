package validator

import (
	"fmt"

	goValidator "github.com/go-playground/validator/v10"
)

type Validator struct {
	validate *goValidator.Validate
}

func New() *Validator {
	return &Validator{validate: goValidator.New()}
}

func (v *Validator) Struct(input interface{}) []map[string]string {
	err := v.validate.Struct(input)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(goValidator.ValidationErrors)
	if !ok {
		return []map[string]string{{"field": "unknown", "message": err.Error()}}
	}

	details := make([]map[string]string, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		details = append(details, map[string]string{
			"field":   fieldErr.Field(),
			"message": fmt.Sprintf("failed on %s", fieldErr.Tag()),
		})
	}
	return details
}
