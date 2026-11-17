package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// maxBodyBytes caps request bodies at 1 MB so a client can't exhaust memory.
const maxBodyBytes = 1 << 20

// WriteJSON serializes v as JSON and writes it with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already sent at this point, so all we can do is log.
		slog.Error("write json response", "error", err)
	}
}

// NoContent writes an empty 204 response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// DecodeJSON reads the request body into dst.
// It rejects bodies that are too large, malformed, contain unknown fields
// or contain more than one JSON value. Every failure is returned as a
// 400 *APIError with a message that is safe to show to the client.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return BadRequest(describeDecodeError(err))
	}

	// A second Decode must hit the end of the body; anything else means
	// the client sent extra data, e.g. {"a":1}{"b":2}.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BadRequest("request body must contain a single JSON object")
	}

	return nil
}

// describeDecodeError turns encoding/json errors into readable messages.
func describeDecodeError(err error) string {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxErr *http.MaxBytesError

	switch {
	case errors.As(err, &syntaxErr):
		return fmt.Sprintf("malformed JSON at position %d", syntaxErr.Offset)
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "malformed JSON"
	case errors.As(err, &typeErr):
		return fmt.Sprintf("field %q has the wrong type", typeErr.Field)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return "unknown field " + strings.TrimPrefix(err.Error(), "json: unknown field ")
	case errors.Is(err, io.EOF):
		return "request body is empty"
	case errors.As(err, &maxErr):
		return "request body is too large"
	default:
		return "invalid request body"
	}
}
