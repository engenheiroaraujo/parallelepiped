// Package bridge converts application errors into the envelope the frontend
// receives through the Wails JS bridge.
//
// A bound Go method that returns an error rejects the JS promise with
// err.Error(); Error makes that string a JSON envelope instead of free text.
package bridge

import (
	"encoding/json"
	"errors"
	"log/slog"

	errs "parallelepiped/internal/platform/errors"
)

// Envelope is the error contract shared with the frontend (src/api/client.ts).
type Envelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error returns nil for nil, otherwise an error whose text is the JSON
// envelope. Internal errors are logged and replaced by a generic message.
func Error(err error) error {
	if err == nil {
		return nil
	}

	envelope := Envelope{Code: string(errs.Internal), Message: "unexpected error"}
	var classified *errs.Error
	if errors.As(err, &classified) && classified.Category != errs.Internal {
		envelope.Code = string(classified.Category)
		envelope.Message = classified.Message
		envelope.Details = classified.Details
	} else {
		slog.Error("unhandled error", "error", err)
	}

	body, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		slog.Error("marshal error envelope", "error", marshalErr)
		return errors.New(`{"code":"internal","message":"unexpected error"}`)
	}
	return errors.New(string(body))
}
