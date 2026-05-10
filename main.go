package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

const nameserverPrefix = "nameserver"

// ExtractNameservers extracts valid IP addresses from the resolv.conf format.
func ExtractNameservers(r io.Reader) ([]string, error) {
	var ipAddresses []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) >= 2 && fields[0] == nameserverPrefix {
			ip := fields[1]

			if net.ParseIP(ip) != nil {
				ipAddresses = append(ipAddresses, ip)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ipAddresses, nil
}

func main() {
	file, err := os.Open("test_resolv.conf")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	_, err = ExtractNameservers(file)
	if err != nil {
		log.Fatal(err)
	}
}
