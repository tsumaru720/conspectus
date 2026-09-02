package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"conspectus/internal/domain"
)

type Meta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func WriteData(w http.ResponseWriter, status int, data any, meta *Meta) {
	body := map[string]any{"data": data}
	if meta != nil {
		body["meta"] = meta
	}
	WriteJSON(w, status, body)
}

func WriteError(w http.ResponseWriter, status int, code, message string, details ...string) {
	WriteJSON(w, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message, Details: details}})
}

var ErrorLog func(err error)

func WriteDomainError(w http.ResponseWriter, err error) {
	var conflict *domain.ConflictError
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &conflict):
		body := map[string]any{
			"error": map[string]any{
				"code":    "conflict",
				"message": conflict.Message,
			},
		}
		if conflict.ExistingID != 0 {
			body["error"].(map[string]any)["existing_id"] = conflict.ExistingID
		}
		WriteJSON(w, http.StatusConflict, body)
	case errors.As(err, &validation):
		WriteError(w, http.StatusBadRequest, "validation_error", validation.Error(), validation.Details...)
	case errors.Is(err, domain.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		WriteError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, domain.ErrValidation):
		WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, domain.ErrReadOnly):
		WriteError(w, http.StatusForbidden, "readonly", err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		WriteError(w, http.StatusUnauthorized, "unauthorized", err.Error())
	default:
		if ErrorLog != nil {
			ErrorLog(err)
		}
		WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: invalid JSON body: %v", domain.ErrValidation, err)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return fmt.Errorf("%w: request body must contain a single JSON object", domain.ErrValidation)
	}
	return nil
}
