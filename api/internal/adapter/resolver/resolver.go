package resolver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ArtemNik1tin/dns-manager/api/internal/domain"
)

const (
	nameserverPrefix             = "nameserver"
	filePerms        os.FileMode = 0o644
)

// Resolver implements usecase.DNSManager by reading and writing
// a resolv.conf-style configuration file.
type Resolver struct {
	path string
	mu   sync.Mutex
}

// NewResolver creates a new Resolver for the given path.
func NewResolver(path string) *Resolver {
	return &Resolver{path: path}
}

// List returns all nameservers found in the configuration file.
func (r *Resolver) List(_ context.Context) ([]domain.Nameserver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.list()
}

// Add appends a nameserver to the configuration file if it does not already exist.
func (r *Resolver) Add(_ context.Context, ns domain.Nameserver) (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	exists, err := r.isNameserverExist(ns)
	if err != nil || exists {
		return err
	}

	file, openErr := os.OpenFile(
		r.path,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		filePerms,
	)
	if openErr != nil {
		return openErr
	}

	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}()

	writer := bufio.NewWriter(file)
	lineToWrite := nameserverPrefix + " " + ns.String() + "\n"

	_, err = writer.WriteString(lineToWrite)
	if err != nil {
		return err
	}

	return writer.Flush()
}

// Delete removes a nameserver from the configuration file if it exists.
func (r *Resolver) Delete(_ context.Context, targetNS domain.Nameserver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existingServers, err := r.list()
	if err != nil {
		return err
	}

	var serversToKeep []domain.Nameserver

	for _, server := range existingServers {
		if !server.IP().Equal(targetNS.IP()) {
			serversToKeep = append(serversToKeep, server)
		}
	}

	if len(serversToKeep) == len(existingServers) {
		return nil
	}

	return r.rewriteFile(serversToKeep)
}

// rewriteFile atomically replaces the configuration file with new content.
func (r *Resolver) rewriteFile(servers []domain.Nameserver) (err error) {
	dir := filepath.Dir(r.path)
	baseName := filepath.Base(r.path)

	tempFile, err := os.CreateTemp(dir, baseName+".*.tmp")
	if err != nil {
		return err
	}

	tempFilePath := tempFile.Name()
	isRenameSuccessful := false

	defer func() {
		if !isRenameSuccessful {
			removeErr := os.Remove(tempFilePath)
			if err == nil {
				err = removeErr
			}
		}
	}()

	if err = writeNameservers(tempFile, servers); err != nil {
		closeErr := tempFile.Close()

		return errors.Join(err, closeErr)
	}

	if err = tempFile.Sync(); err != nil {
		closeErr := tempFile.Close()

		return errors.Join(err, closeErr)
	}

	if err = tempFile.Close(); err != nil {
		return err
	}

	if err = os.Chmod(tempFilePath, filePerms); err != nil {
		return err
	}

	if err = os.Rename(tempFilePath, r.path); err != nil {
		return err
	}

	isRenameSuccessful = true

	return nil
}

// list reads nameservers from the file without acquiring a lock.
// Must be called with r.mu held.
func (r *Resolver) list() (ns []domain.Nameserver, err error) {
	file, openErr := os.Open(r.path)
	if openErr != nil {
		return nil, openErr
	}

	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}()

	ns, err = extractNameservers(file)

	return ns, err
}

// isNameserverExist checks whether a given nameserver already exists in the file.
// Must be called with r.mu held.
func (r *Resolver) isNameserverExist(ns domain.Nameserver) (bool, error) {
	current, err := r.list()
	if err != nil {
		return false, err
	}

	for _, existing := range current {
		if existing.IP().Equal(ns.IP()) {
			return true, nil
		}
	}

	return false, nil
}

// extractNameservers parses an io.Reader for lines starting with "nameserver".
func extractNameservers(reader io.Reader) ([]domain.Nameserver, error) {
	var ipAddresses []domain.Nameserver

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) >= 2 && fields[0] == nameserverPrefix {
			ns, err := domain.NewNameserver(fields[1])
			if err != nil {
				return nil, err
			}

			ipAddresses = append(ipAddresses, ns)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ipAddresses, nil
}

// writeNameservers writes nameservers to the given writer in resolv.conf format.
func writeNameservers(writer io.Writer, servers []domain.Nameserver) error {
	bufWriter := bufio.NewWriter(writer)

	for _, server := range servers {
		line := fmt.Sprintf("%s %s\n", nameserverPrefix, server.IP().String())
		if _, err := bufWriter.WriteString(line); err != nil {
			return err
		}
	}

	return bufWriter.Flush()
}
