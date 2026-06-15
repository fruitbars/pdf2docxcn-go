package pdfconv

import (
	"encoding/json"
	"fmt"
)

// APIError represents an error returned by the API.
type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d (%s): %s", e.HTTPStatus, e.Code, e.Message)
}

func parseAPIError(body []byte, status int) error {
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return &APIError{Code: e.Error, Message: e.Message, HTTPStatus: status}
	}
	return fmt.Errorf("http %d: %s", status, string(body))
}
