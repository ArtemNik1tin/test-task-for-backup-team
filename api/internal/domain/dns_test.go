package domain

import (
	"errors"
	"net"
	"testing"
)

func TestNewNameserver_ValidIPv4(t *testing.T) {
	ns, err := NewNameserver("8.8.8.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ns.IP().Equal(net.ParseIP("8.8.8.8")) {
		t.Errorf("expected 8.8.8.8, got %s", ns.IP())
	}
}

func TestNewNameserver_ValidIPv6(t *testing.T) {
	ns, err := NewNameserver("2001:4860:4860::8888")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ns.IP().Equal(net.ParseIP("2001:4860:4860::8888")) {
		t.Errorf("expected 2001:4860:4860::8888, got %s", ns.IP())
	}
}

func TestNewNameserver_EmptyString(t *testing.T) {
	_, err := NewNameserver("")
	if !errors.Is(err, ErrInvalidIP) {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

func TestNewNameserver_InvalidFormat(t *testing.T) {
	_, err := NewNameserver("not-an-ip")
	if !errors.Is(err, ErrInvalidIP) {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

func TestNewNameserver_IPv4WithPort(t *testing.T) {
	_, err := NewNameserver("8.8.8.8:53")
	if !errors.Is(err, ErrInvalidIP) {
		t.Errorf("expected ErrInvalidIP for address with port, got %v", err)
	}
}

func TestNewNameserver_LoopbackIPv4(t *testing.T) {
	_, err := NewNameserver("127.0.0.1")
	if !errors.Is(err, ErrNotGlobalUnicast) {
		t.Errorf("expected ErrNotGlobalUnicast, got %v", err)
	}
}

func TestNewNameserver_LoopbackIPv6(t *testing.T) {
	_, err := NewNameserver("::1")
	if !errors.Is(err, ErrNotGlobalUnicast) {
		t.Errorf("expected ErrNotGlobalUnicast, got %v", err)
	}
}

func TestNameserver_IP_ReturnsCopy(t *testing.T) {
	ns, _ := NewNameserver("8.8.8.8")
	ip := ns.IP()
	ip[0] = 0

	original := ns.IP()
	if !original.Equal(net.ParseIP("8.8.8.8")) {
		t.Error("IP() should return a copy")
	}
}

func TestNameserver_String(t *testing.T) {
	ns, _ := NewNameserver("8.8.8.8")
	if ns.String() != "8.8.8.8" {
		t.Errorf("expected 8.8.8.8, got %s", ns.String())
	}
}

func TestNameserver_String_IPv6(t *testing.T) {
	ns, _ := NewNameserver("2001:4860:4860::8888")
	if ns.String() != "2001:4860:4860::8888" {
		t.Errorf("expected 2001:4860:4860::8888, got %s", ns.String())
	}
}

func TestErrInvalidIP_Sentinel(t *testing.T) {
	if ErrInvalidIP == nil {
		t.Fatal("sentinel error must not be nil")
	}
}

func TestErrNotGlobalUnicast_Sentinel(t *testing.T) {
	if ErrNotGlobalUnicast == nil {
		t.Fatal("sentinel error must not be nil")
	}
}
