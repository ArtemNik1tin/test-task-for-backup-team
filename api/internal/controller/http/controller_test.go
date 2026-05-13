// Package http contains HTTP handlers and tests for the DNS Manager API.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ArtemNik1tin/dns-manager/api/internal/domain"
	"github.com/ArtemNik1tin/dns-manager/api/internal/dto"
)

// mockUsecase implements DNSUseCase for testing.
type mockUsecase struct {
	addErr  error
	delErr  error
	listRes []domain.Nameserver
	listErr error
}

func (m *mockUsecase) Add(_ context.Context, _ string) error {
	return m.addErr
}

func (m *mockUsecase) Delete(_ context.Context, _ string) error {
	return m.delErr
}

func (m *mockUsecase) List(_ context.Context) ([]domain.Nameserver, error) {
	return m.listRes, m.listErr
}

func newTestController(mock *mockUsecase) *DNSController {
	log := slog.New(slog.DiscardHandler)
	return NewDNSController(log, mock)
}

func register(t *testing.T, ctrl *DNSController) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /api/dns", http.HandlerFunc(ctrl.Add))
	mux.Handle("DELETE /api/dns", http.HandlerFunc(ctrl.Delete))
	mux.Handle("GET /api/dns", http.HandlerFunc(ctrl.List))
	return httptest.NewServer(mux)
}

// ---- Add tests ----

func TestAdd_Success(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestAdd_EmptyBody(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAdd_InvalidJSON(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte(`not json`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAdd_EmptyIP(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":""}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAdd_InvalidIPFromDomain(t *testing.T) {
	ctrl := newTestController(&mockUsecase{addErr: domain.ErrInvalidIP})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAdd_NotGlobalUnicast(t *testing.T) {
	ctrl := newTestController(&mockUsecase{addErr: domain.ErrNotGlobalUnicast})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var errResp dto.ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error != "IP must be a global unicast address" {
		t.Errorf("unexpected error message: %s", errResp.Error)
	}
}

func TestAdd_InternalError(t *testing.T) {
	ctrl := newTestController(&mockUsecase{addErr: io.ErrUnexpectedEOF})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// ---- Delete tests ----

func TestDelete_Success(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDelete_EmptyBody(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/api/dns", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDelete_InvalidJSON(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/api/dns", bytes.NewReader([]byte(`not json`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDelete_DomainError(t *testing.T) {
	ctrl := newTestController(&mockUsecase{delErr: domain.ErrInvalidIP})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"bad"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// ---- List tests ----

func TestList_Success(t *testing.T) {
	ns1, _ := domain.NewNameserver("8.8.8.8")
	ns2, _ := domain.NewNameserver("1.1.1.1")
	ctrl := newTestController(&mockUsecase{listRes: []domain.Nameserver{ns1, ns2}})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/dns", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var listResp dto.ListDNSResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(listResp.Servers))
	}
}

func TestList_Empty(t *testing.T) {
	ctrl := newTestController(&mockUsecase{listRes: []domain.Nameserver{}})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/dns", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var listResp dto.ListDNSResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(listResp.Servers))
	}
}

func TestList_InternalError(t *testing.T) {
	ctrl := newTestController(&mockUsecase{listErr: io.ErrClosedPipe})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/dns", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// ---- Response content tests ----

func TestAdd_ResponseBody(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var msg dto.MessageResponse
	if err := json.Unmarshal(body, &msg); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msg.Message != "added" {
		t.Errorf("expected 'added', got '%s'", msg.Message)
	}
}

func TestDelete_ResponseBody(t *testing.T) {
	ctrl := newTestController(&mockUsecase{})
	ts := register(t, ctrl)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var msg dto.MessageResponse
	if err := json.Unmarshal(body, &msg); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msg.Message != "deleted" {
		t.Errorf("expected 'deleted', got '%s'", msg.Message)
	}
}
