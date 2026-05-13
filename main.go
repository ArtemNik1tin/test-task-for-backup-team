package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// nameserverPrefix is a keyword in the resolv.conf configuration file.
	nameserverPrefix                            = "nameserver"
	// FilePermsForNameserverRecording defines the default permissions (rw-r--r--)
	// for DNS configuration files on Unix systems.
	FilePermsForNameserverRecording os.FileMode = 0o644
	NumberOfExpectedCLArguments                 = 2
)

// Nameserver represents a DNS server record that contains a verified IP address.
type Nameserver struct {
	ip net.IP
}

// NewNameserver creates a new Nameserver instance.
// Returns an error if an incorrect IP address is passed.
func NewNameserver(ip string) (Nameserver, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return Nameserver{}, errors.New("invalid IP")
	}

	return Nameserver{ip: parsedIP}, nil
}

// DNSReader provides a method for obtaining a list of DNS servers.
type DNSReader interface {
	// List returns the current list of configured DNS servers.
	List() ([]Nameserver, error)
}

// DNSWriter provides methods for modifying the list of DNS servers.
type DNSWriter interface {
	// Add adds a new DNS server to the configuration.
	Add(ip Nameserver) error
	// Delete removes an existing DNS server from the configuration.
	Delete(ip Nameserver) error
}

// DNSManager combines read and write interfaces for DNS management.
type DNSManager interface {
	DNSReader
	DNSWriter
}

// FileDNSManager implements the DNSManager interface using a text file
// (for example, /etc/resolv.conf) as storage.
type FileDNSManager struct {
	// path to the configuration file.
	path string
	// mu provides thread safety for concurrent access to a file.
	mu   sync.Mutex
}

// List returns a slice of Nameserver objects found in the configuration file.
// The method is safe to use simultaneously from multiple goroutines.
func (m *FileDNSManager) List() ([]Nameserver, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.list()
}

// Add adds a DNS server to the end of the file if it does not already exist.
// Uses a mutex to prevent a race condition during writing.
func (m *FileDNSManager) Add(ns Nameserver) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	isNameserverExist, err := m.isNameserverExist(ns)
	if err != nil || isNameserverExist {
		return err
	}

	file, openErr := os.OpenFile(
		m.path,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		FilePermsForNameserverRecording,
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
	lineToWrite := nameserverPrefix + " " + ns.ip.String() + "\n"

	_, err = writer.WriteString(lineToWrite)
	if err != nil {
		return err
	}

	return writer.Flush()
}

// Delete removes the server from the configuration file.
// The operation is performed by overwriting the file to a temporary buffer and then replacing it (atomic rename).
func (m *FileDNSManager) Delete(targetNS Nameserver) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existingServers, err := m.list()
	if err != nil {
		return err
	}

	var serversToKeep []Nameserver

	for _, server := range existingServers {
		if !server.ip.Equal(targetNS.ip) {
			serversToKeep = append(serversToKeep, server)
		}
	}

	if len(serversToKeep) == len(existingServers) {
		return nil
	}

	return m.rewriteFile(serversToKeep)
}

// rewriteFile creates a temporary file, writes new data to it, and atomically replaces
// the original file. This ensures data integrity in case of failures.
func (m *FileDNSManager) rewriteFile(servers []Nameserver) (err error) {
	dir := filepath.Dir(m.path)
	baseName := filepath.Base(m.path)

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

	if err = os.Chmod(tempFilePath, FilePermsForNameserverRecording); err != nil {
		return err
	}

	if err = os.Rename(tempFilePath, m.path); err != nil {
		return err
	}

	isRenameSuccessful = true

	return nil
}

// writeNameservers writes a list of servers to any io.Writer.
func writeNameservers(writer io.Writer, servers []Nameserver) error {
	bufWriter := bufio.NewWriter(writer)

	for _, server := range servers {
		line := fmt.Sprintf("%s %s\n", nameserverPrefix, server.ip.String())
		if _, err := bufWriter.WriteString(line); err != nil {
			return err
		}
	}

	return bufWriter.Flush()
}

// list is an internal method for reading a file without acquiring a mutex.
// It is used inside methods that have already acquired a Lock.
func (m *FileDNSManager) list() (ns []Nameserver, err error) {
	file, openErr := os.Open(m.path)
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

// extractNameservers analyzes the data stream and extracts the IP addresses that follow the 'nameserver' prefix.
func extractNameservers(reader io.Reader) ([]Nameserver, error) {
	var ipAddresses []Nameserver

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) >= 2 && fields[0] == nameserverPrefix {
			ip, err := NewNameserver(fields[1])
			if err != nil {
				return nil, err
			}

			ipAddresses = append(ipAddresses, ip)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ipAddresses, nil
}

// isNameserverExist checks whether a specific IP is present in the current list of servers.
func (m *FileDNSManager) isNameserverExist(ns Nameserver) (bool, error) {
	current, err := m.list()
	if err != nil {
		return false, err
	}

	for _, existing := range current {
		if existing.ip.Equal(ns.ip) {
			return true, nil
		}
	}

	return false, nil
}

func main() {
	flag.Usage = func() {
		fmt.Println("Использование: dns-cli <команда> [аргументы]")
		fmt.Println("\nКоманды:")
		fmt.Println("  list      Получить список всех DNS-серверов")
		fmt.Println("  add       Добавить DNS-сервер")
		fmt.Println("  del       Удалить DNS-сервер")
		fmt.Println("\nДля справки по конкретной команде используйте:")
		fmt.Println("  dns-cli <команда> --help")
	}

	if len(os.Args) < NumberOfExpectedCLArguments {
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		handleList()
	case "add":
		handleAdd(os.Args[2:])
	case "del":
		handleDel(os.Args[2:])
	default:
		fmt.Printf("Неизвестная команда: %s\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
	}
}

func handleList() {
	// TODO
}

func handleAdd(args []string) {
	// TODO
}

func handleDel(args []string) {
	// TODO
}
