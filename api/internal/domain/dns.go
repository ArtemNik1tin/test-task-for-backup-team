package domain

import (
	"errors"
	"net"
)

var (
	ErrInvalidIP        = errors.New("invalid IP")
	ErrNotGlobalUnicast = errors.New("IP must be a global unicast address")
)

// Nameserver represents a DNS server record that contains a verified IP address.
type Nameserver struct {
	ip net.IP
}

// NewNameserver creates a new Nameserver instance.
// Returns ErrInvalidIP or ErrNotGlobalUnicast if validation fails.
func NewNameserver(ip string) (Nameserver, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return Nameserver{}, ErrInvalidIP
	}

	if !parsedIP.IsGlobalUnicast() {
		return Nameserver{}, ErrNotGlobalUnicast
	}

	return Nameserver{ip: parsedIP}, nil
}

// IP returns a copy of net.IP.
func (n Nameserver) IP() net.IP {
	return n.ip
}

// String returns a string representation.
func (n Nameserver) String() string {
	return n.ip.String()
}
