package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/ArtemNik1tin/dns-manager/api/internal/domain"
)

// mockStorage implements DNSManager for testing.
type mockStorage struct {
	servers []domain.Nameserver
	addErr  error
	delErr  error
	listErr error
}

func (m *mockStorage) List(_ context.Context) ([]domain.Nameserver, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]domain.Nameserver, len(m.servers))
	copy(result, m.servers)
	return result, nil
}

func (m *mockStorage) Add(_ context.Context, ns domain.Nameserver) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.servers = append(m.servers, ns)
	return nil
}

func (m *mockStorage) Delete(_ context.Context, ns domain.Nameserver) error {
	if m.delErr != nil {
		return m.delErr
	}
	for i, s := range m.servers {
		if s.IP().Equal(ns.IP()) {
			m.servers = append(m.servers[:i], m.servers[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestUsecase_Add_Valid(t *testing.T) {
	store := &mockStorage{}
	uc := NewDNSUseCase(store)

	err := uc.Add(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(store.servers))
	}
}

func TestUsecase_Add_InvalidIP(t *testing.T) {
	store := &mockStorage{}
	uc := NewDNSUseCase(store)

	err := uc.Add(context.Background(), "invalid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidIP) {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

func TestUsecase_Add_Loopback(t *testing.T) {
	store := &mockStorage{}
	uc := NewDNSUseCase(store)

	err := uc.Add(context.Background(), "127.0.0.1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrNotGlobalUnicast) {
		t.Errorf("expected ErrNotGlobalUnicast, got %v", err)
	}
}

func TestUsecase_Add_StorageError(t *testing.T) {
	store := &mockStorage{addErr: errors.New("disk full")}
	uc := NewDNSUseCase(store)

	err := uc.Add(context.Background(), "8.8.8.8")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUsecase_Delete_Valid(t *testing.T) {
	ns, _ := domain.NewNameserver("8.8.8.8")
	store := &mockStorage{servers: []domain.Nameserver{ns}}
	uc := NewDNSUseCase(store)

	err := uc.Delete(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.servers) != 0 {
		t.Fatalf("expected 0 servers, got %d", len(store.servers))
	}
}

func TestUsecase_Delete_InvalidIP(t *testing.T) {
	store := &mockStorage{}
	uc := NewDNSUseCase(store)

	err := uc.Delete(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidIP) {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

func TestUsecase_Delete_StorageError(t *testing.T) {
	store := &mockStorage{delErr: errors.New("permission denied")}
	uc := NewDNSUseCase(store)

	err := uc.Delete(context.Background(), "8.8.8.8")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUsecase_List_ReturnsServers(t *testing.T) {
	ns1, _ := domain.NewNameserver("8.8.8.8")
	ns2, _ := domain.NewNameserver("1.1.1.1")
	store := &mockStorage{servers: []domain.Nameserver{ns1, ns2}}
	uc := NewDNSUseCase(store)

	servers, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestUsecase_List_Empty(t *testing.T) {
	store := &mockStorage{}
	uc := NewDNSUseCase(store)

	servers, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected 0 servers, got %d", len(servers))
	}
}

func TestUsecase_List_StorageError(t *testing.T) {
	store := &mockStorage{listErr: errors.New("connection reset")}
	uc := NewDNSUseCase(store)

	_, err := uc.List(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
