package model

import "fmt"

type APIError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func (err *APIError) Error() string {
	return fmt.Sprintf("%s: %s", err.Code, err.Message)
}

func NewNotFoundError(resource string, identifier string) *APIError {
	return &APIError{
		Status:  404,
		Code:    "not_found",
		Message: fmt.Sprintf("%s %q was not found", resource, identifier),
	}
}

func NewInvalidError(message string, details map[string]string) *APIError {
	return &APIError{
		Status:  400,
		Code:    "invalid_request",
		Message: message,
		Details: details,
	}
}

func NewUnavailableError(component string, cause error) *APIError {
	details := map[string]string{}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return &APIError{
		Status:  503,
		Code:    "dependency_unavailable",
		Message: fmt.Sprintf("%s is unavailable", component),
		Details: details,
	}
}

func NewConflictError(message string) *APIError {
	return &APIError{
		Status:  409,
		Code:    "conflict",
		Message: message,
	}
}
