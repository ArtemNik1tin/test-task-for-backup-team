package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ArtemNik1tin/dns-manager/api/internal/domain"
	"github.com/ArtemNik1tin/dns-manager/api/internal/dto"
)

// DNSUseCase defines the business logic contract that the controller needs.
// This allows the http package to be independent of the specific implementation of the UseCase.
type DNSUseCase interface {
	Add(ctx context.Context, ip string) error
	Delete(ctx context.Context, ip string) error
	List(ctx context.Context) ([]domain.Nameserver, error)
}

// validator describes objects that can check their own correctness.
type validator interface {
	Validate() error
}

// DNSController implements HTTP handlers for managing DNS servers.
type DNSController struct {
	log *slog.Logger
	uc  DNSUseCase
}

// NewDNSController creates a new controller instance with a logger and a use case.
func NewDNSController(log *slog.Logger, uc DNSUseCase) *DNSController {
	return &DNSController{
		log: log,
		uc:  uc,
	}
}

// Add handles POST /api/dns to add a new DNS server.
func (c *DNSController) Add(w http.ResponseWriter, r *http.Request) {
	var req dto.NameserverRequest
	if err := c.decode(w, r, &req); err != nil {
		return
	}

	if err := c.uc.Add(r.Context(), req.IP); err != nil {
		c.writeError(w, err, "add", req.IP)

		return
	}

	c.respond(w, http.StatusCreated, dto.MessageResponse{Message: "added"})
}

// Delete handles DELETE /api/dns to delete a DNS server.
func (c *DNSController) Delete(w http.ResponseWriter, r *http.Request) {
	var req dto.NameserverRequest
	if err := c.decode(w, r, &req); err != nil {
		return
	}

	if err := c.uc.Delete(r.Context(), req.IP); err != nil {
		c.writeError(w, err, "delete", req.IP)

		return
	}

	c.respond(w, http.StatusOK, dto.MessageResponse{Message: "deleted"})
}

// List handles GET /api/dns and returns a list of all DNS servers.
func (c *DNSController) List(w http.ResponseWriter, r *http.Request) {
	servers, err := c.uc.List(r.Context())
	if err != nil {
		c.log.Error("failed to list servers", "error", err)
		c.respond(w, http.StatusInternalServerError, dto.ErrorResponse{Error: "internal error"})

		return
	}

	ips := make([]string, len(servers))
	for i, s := range servers {
		ips[i] = s.String()
	}

	c.respond(w, http.StatusOK, dto.ListDNSResponse{Servers: ips})
}

// decode is an auxiliary method for deserializing JSON and triggering DTO validation.
func (c *DNSController) decode(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		c.log.Warn("failed to decode request body", "error", err)
		c.respond(w, http.StatusBadRequest, dto.ErrorResponse{Error: "invalid request body"})

		return err
	}

	if val, ok := v.(validator); ok {
		if err := val.Validate(); err != nil {
			c.respond(w, http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})

			return err
		}
	}

	return nil
}

// writeError maps domain layer errors to corresponding HTTP responses and logs them.
func (c *DNSController) writeError(w http.ResponseWriter, err error, op, ip string) {
	var status int

	var message string

	switch {
	case errors.Is(err, domain.ErrInvalidIP):
		status = http.StatusBadRequest
		message = "invalid IP address"

		c.log.Warn(op+": invalid ip provided", "ip", ip)
	case errors.Is(err, domain.ErrNotGlobalUnicast):
		status = http.StatusBadRequest
		message = "IP must be a global unicast address"

		c.log.Warn(op+": non-global ip provided", "ip", ip)
	default:
		status = http.StatusInternalServerError
		message = "internal error"

		c.log.Error(op+": unexpected error", "ip", ip, "error", err)
	}

	c.respond(w, status, dto.ErrorResponse{Error: message})
}

// respond generates a JSON response with the specified status code.
func (c *DNSController) respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			c.log.Error("failed to encode response", "error", err)
		}
	}
}
