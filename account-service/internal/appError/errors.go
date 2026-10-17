package appError

import (
	"encoding/json"
	"errors"
	"net/http"
)

type AppError struct {
	Err        error
	Message    string
	StatusCode int
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(err error, message string, statusCode int) *AppError {
	return &AppError{
		Err:        err,
		Message:    message,
		StatusCode: statusCode,
	}
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func WriteError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(appErr.StatusCode)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Message: appErr.Message})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Message: "internal server error"})
}
