// dns-cli is a command-line client for the DNS Manager REST API.
//
// Usage:
//
//	dns-cli list			  all DNS servers
//	dns-cli add 8.8.8.8       Add a DNS server
//	dns-cli del 8.8.8.8       Delete a DNS server
//	dns-cli --server http://host:9090 list  Use custom server address
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// Server response structs mirror the server-side DTOs.
type listDNSResponse struct {
	Servers []string `json:"servers"`
}

type messageResponse struct {
	Message string `json:"message"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Client provides a high-level interface to the DNS Manager HTTP API.
type Client struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
}

// NewClient creates a new Client with the given server address.
func NewClient(addr string) (*Client, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid server address: %w", err)
	}

	return &Client{
		BaseURL: u,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second, // Magic number: 5, in <assign> detected
		},
	}, nil
}

// List returns all configured DNS servers.
func (c *Client) List(ctx context.Context) ([]string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/dns", nil)
	if err != nil {
		return nil, err
	}

	var res listDNSResponse
	if err := c.execute(req, &res); err != nil {
		return nil, err
	}

	return res.Servers, nil
}

// Add adds a new DNS server and returns a confirmation message.
func (c *Client) Add(ctx context.Context, ip string) (string, error) {
	payload := struct {
		IP string `json:"ip"`
	}{IP: ip}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/dns", payload)
	if err != nil {
		return "", err
	}

	var res messageResponse
	if err := c.execute(req, &res); err != nil {
		return "", err
	}

	return res.Message, nil
}

// Delete removes a DNS server and returns a confirmation message.
func (c *Client) Delete(ctx context.Context, ip string) (string, error) {
	payload := struct {
		IP string `json:"ip"`
	}{IP: ip}

	req, err := c.newRequest(ctx, http.MethodDelete, "/api/dns", payload)
	if err != nil {
		return "", err
	}

	var res messageResponse
	if err := c.execute(req, &res); err != nil {
		return "", err
	}

	return res.Message, nil
}

var serverAddr string

func main() {
	rootCmd := &cobra.Command{
		Use:   "dns-cli",
		Short: "CLI client for DNS Manager",
		Long: `dns-cli is a command-line client for managing DNS servers 
via the DNS Manager REST API.

Examples:
  dns-cli list              List all DNS servers
  dns-cli add 8.8.8.8       Add a DNS server
  dns-cli del 8.8.8.8       Delete a DNS server`,
	}

	rootCmd.PersistentFlags().
		StringVarP(&serverAddr, "server", "s", "http://localhost:8080", "DNS Manager server address")

	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newDelCmd())

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured DNS servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := NewClient(serverAddr)
			if err != nil {
				return err
			}

			servers, err := api.List(cmd.Context())
			if err != nil {
				return err
			}

			if len(servers) == 0 {
				fmt.Println("No DNS servers configured.")

				return nil
			}

			for _, s := range servers {
				fmt.Println(s)
			}

			return nil
		},
	}
}

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <ip>",
		Short: "Add a DNS server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := NewClient(serverAddr)
			if err != nil {
				return err
			}

			msg, err := api.Add(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			fmt.Println(msg)

			return nil
		},
	}
}

func newDelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "del <ip>",
		Short: "Delete a DNS server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := NewClient(serverAddr)
			if err != nil {
				return err
			}

			msg, err := api.Delete(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			fmt.Println(msg)

			return nil
		},
	}
}

// newRequest builds an HTTP request with a JSON body.
func (c *Client) newRequest(
	ctx context.Context,
	method, path string,
	body any,
) (*http.Request, error) {
	u := c.BaseURL.JoinPath(path)

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// execute sends the request and decodes the JSON response into v.
func (c *Client) execute(req *http.Request, v any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close() // Unhandled error

	if resp.StatusCode >= http.StatusBadRequest {
		return c.decodeError(resp)
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// decodeError parses an error response from the server.
func (c *Client) decodeError(resp *http.Response) error {
	var errRes errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errRes); err != nil {
		return fmt.Errorf("server returned status %d (failed to decode error)", resp.StatusCode)
	}

	return fmt.Errorf("server: %s", errRes.Error)
}
