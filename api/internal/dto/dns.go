// Package dto provides data transfer objects for the DNS Manager HTTP API.
package dto

import (
	"errors"
	"strings"
)

// NameserverRequest is the request body for add and delete operations.
type NameserverRequest struct {
	IP string `json:"ip"`
}

// Validate checks that the IP field is present.
func (r *NameserverRequest) Validate() error {
	r.IP = strings.TrimSpace(r.IP)
	if r.IP == "" {
		return errors.New("ip is required")
	}

	return nil
}

// ListDNSResponse is the response body for listing DNS servers.
type ListDNSResponse struct {
	Servers []string `json:"servers"`
}

// ErrorResponse is a generic error response for the API.
type ErrorResponse struct {
	Error string `json:"error"`
}

// MessageResponse is a generic success response with a human-readable message.
type MessageResponse struct {
	Message string `json:"message"`
}
