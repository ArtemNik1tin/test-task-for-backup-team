package usecase

import (
	"context"
	"fmt"

	"github.com/ArtemNik1tin/dns-manager/api/internal/domain"
)

// DNSReader provides read access to DNS server storage.
type DNSReader interface {
	List(ctx context.Context) ([]domain.Nameserver, error)
}

// DNSWriter provides write access to DNS server storage.
type DNSWriter interface {
	Add(ctx context.Context, ns domain.Nameserver) error
	Delete(ctx context.Context, ns domain.Nameserver) error
}

// DNSManager combines read and write operations for DNS server storage.
type DNSManager interface {
	DNSReader
	DNSWriter
}

// DNSUseCase implements the business logic for DNS server management.
type DNSUseCase struct {
	storage DNSManager
}

// NewDNSUseCase creates a new DNSUseCase with the given storage backend.
func NewDNSUseCase(storage DNSManager) *DNSUseCase {
	return &DNSUseCase{storage: storage}
}

// Add validates and adds a new DNS server identified by ip.
func (uc *DNSUseCase) Add(ctx context.Context, ip string) error {
	ns, err := domain.NewNameserver(ip)
	if err != nil {
		return fmt.Errorf("add: %w", err)
	}

	return uc.storage.Add(ctx, ns)
}

// Delete validates and removes a DNS server identified by ip.
func (uc *DNSUseCase) Delete(ctx context.Context, ip string) error {
	ns, err := domain.NewNameserver(ip)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return uc.storage.Delete(ctx, ns)
}

// List returns all configured DNS servers.
func (uc *DNSUseCase) List(ctx context.Context) ([]domain.Nameserver, error) {
	ns, err := uc.storage.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	return ns, nil
}
