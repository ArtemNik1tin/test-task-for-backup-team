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
	nameserverPrefix                            = "nameserver"
	FilePermsForNameserverRecording os.FileMode = 0o644
	NumberOfExpectedCLArguments                 = 2
)

// Nameserver is a record about a DNS server.
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

type DNSReader interface {
	List() ([]Nameserver, error)
}

type DNSWriter interface {
	Add(ip Nameserver) error
	Delete(ip Nameserver) error
}

type DNSManager interface {
	DNSReader
	DNSWriter
}

type FileDNSManager struct {
	path string
	mu   sync.Mutex
}

func (m *FileDNSManager) List() ([]Nameserver, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.list()
}

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

// Delete removes the specified DNS server from the configuration.
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

// rewriteFile safely overwrites the configuration file through an atomic rename operation.
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

const serverURL = "http://localhost:8080/api/v1/dns"

// dnsRequest describes the JSON structure for sending to the server.
type dnsRequest struct {
	IP string `json:"ip"`
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
