package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ArtemNik1tin/dns-manager/api/internal/dto"
)

func newIntegrationServer(t *testing.T, initialData ...string) *httptest.Server {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	for _, line := range initialData {
		_, _ = f.WriteString(line + "\n")
	}
	f.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(context.Background(), log, Config{ResolvePath: path})
	return httptest.NewServer(handler)
}

func TestIntegration_AddAndList(t *testing.T) {
	ts := newIntegrationServer(t)
	defer ts.Close()

	// Add a DNS server.
	resp, err := http.Post(ts.URL+"/api/dns", "application/json", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("add request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add: expected 201, got %d", resp.StatusCode)
	}

	// List and verify.
	resp, err = http.Get(ts.URL + "/api/dns")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var listResp dto.ListDNSResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Servers) != 1 || listResp.Servers[0] != "8.8.8.8" {
		t.Errorf("expected [8.8.8.8], got %v", listResp.Servers)
	}
}

func TestIntegration_DeleteAndList(t *testing.T) {
	ts := newIntegrationServer(t, "nameserver 8.8.8.8", "nameserver 1.1.1.1")
	defer ts.Close()

	// Delete one server.
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", resp.StatusCode)
	}

	// List and verify one server remains.
	resp, err = http.Get(ts.URL + "/api/dns")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var listResp dto.ListDNSResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Servers) != 1 || listResp.Servers[0] != "1.1.1.1" {
		t.Errorf("expected [1.1.1.1], got %v", listResp.Servers)
	}
}

func TestIntegration_AddDuplicate(t *testing.T) {
	ts := newIntegrationServer(t, "nameserver 8.8.8.8")
	defer ts.Close()

	// Try adding duplicate.
	resp, err := http.Post(ts.URL+"/api/dns", "application/json", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	if err != nil {
		t.Fatalf("add duplicate request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add duplicate: expected 201 (idempotent), got %d", resp.StatusCode)
	}

	// Verify still only one.
	resp, err = http.Get(ts.URL + "/api/dns")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var listResp dto.ListDNSResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Servers) != 1 {
		t.Errorf("expected 1 server (no duplicate), got %d", len(listResp.Servers))
	}
}

func TestIntegration_InvalidIP(t *testing.T) {
	ts := newIntegrationServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/dns", "application/json", bytes.NewReader([]byte(`{"ip":"not-an-ip"}`)))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid IP, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp dto.ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if errResp.Error != "invalid IP address" {
		t.Errorf("expected 'invalid IP address', got '%s'", errResp.Error)
	}
}

func TestIntegration_EmptyBody(t *testing.T) {
	ts := newIntegrationServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/dns", "application/json", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d", resp.StatusCode)
	}
}

func TestIntegration_ListFromInitialData(t *testing.T) {
	ts := newIntegrationServer(t, "nameserver 8.8.8.8", "nameserver 1.1.1.1")
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/dns")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var listResp dto.ListDNSResponse
	json.Unmarshal(body, &listResp)

	if len(listResp.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(listResp.Servers))
	}
}

func TestIntegration_AddAndVerifyDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	os.WriteFile(path, []byte("nameserver 8.8.8.8\n"), 0o644)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(context.Background(), log, Config{ResolvePath: path})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Add a server.
	resp, err := http.Post(ts.URL+"/api/dns", "application/json", bytes.NewReader([]byte(`{"ip":"1.1.1.1"}`)))
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	resp.Body.Close()

	// Verify file on disk has both servers.
	data, _ := os.ReadFile(path)
	expected := "nameserver 8.8.8.8\nnameserver 1.1.1.1\n"
	if string(data) != expected {
		t.Errorf("file content:\ngot:  %q\nwant: %q", string(data), expected)
	}
}

func TestIntegration_DeleteAndVerifyDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	os.WriteFile(path, []byte("nameserver 8.8.8.8\nnameserver 1.1.1.1\n"), 0o644)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(context.Background(), log, Config{ResolvePath: path})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/dns", bytes.NewReader([]byte(`{"ip":"8.8.8.8"}`)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	resp.Body.Close()

	data, _ := os.ReadFile(path)
	expected := "nameserver 1.1.1.1\n"
	if string(data) != expected {
		t.Errorf("file content:\ngot:  %q\nwant: %q", string(data), expected)
	}
}
