// Package validation accumulates per-field errors so the UI can show every
// problem at once instead of one round-trip per field.
package validation

import errs "parallelepiped/internal/platform/errors"

// FormValidator collects field -> message pairs. The key must be the public
// (JSON) field name, since it reaches the frontend unchanged.
type FormValidator struct {
	FieldErrors map[string]string
}

func (fv *FormValidator) Valid() bool { return len(fv.FieldErrors) == 0 }

func (fv *FormValidator) AddFieldError(field, message string) {
	if fv.FieldErrors == nil {
		fv.FieldErrors = make(map[string]string)
	}
	if _, exists := fv.FieldErrors[field]; !exists {
		fv.FieldErrors[field] = message
	}
}

func (fv *FormValidator) Details() map[string]any {
	if len(fv.FieldErrors) == 0 {
		return nil
	}
	out := make(map[string]any, len(fv.FieldErrors))
	for field, message := range fv.FieldErrors {
		out[field] = message
	}
	return out
}

// Err converts the accumulated errors into a typed error, or nil when valid.
func (fv *FormValidator) Err() error {
	if fv.Valid() {
		return nil
	}
	return errs.Validation("invalid data", fv.Details())
}

// Field is the shortcut for a single field error.
func Field(field, message string) error {
	var form FormValidator
	form.AddFieldError(field, message)
	return form.Err()
}
