package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client wraps HTTP calls to miraeboy.
type Client struct {
	cfg    *Config
	http   *http.Client
}

func NewClient(cfg *Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// ServerURL returns the configured server URL (without trailing slash).
func (c *Client) ServerURL() string {
	return strings.TrimRight(c.cfg.ServerURL, "/")
}

// do performs an HTTP request and returns the response body.
// On non-2xx status it returns an error containing the response body.
func (c *Client) do(method, path string, body any) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.ServerURL()+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to extract error message from JSON body.
		var apiErr struct{ Error string `json:"error"` }
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			return nil, resp.StatusCode, fmt.Errorf("%s", apiErr.Error)
		}
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	return data, resp.StatusCode, nil
}

func (c *Client) Get(path string) ([]byte, error) {
	b, _, err := c.do("GET", path, nil)
	return b, err
}

func (c *Client) Post(path string, body any) ([]byte, error) {
	b, _, err := c.do("POST", path, body)
	return b, err
}

func (c *Client) Patch(path string, body any) ([]byte, error) {
	b, _, err := c.do("PATCH", path, body)
	return b, err
}

func (c *Client) Put(path string, body any) ([]byte, error) {
	b, _, err := c.do("PUT", path, body)
	return b, err
}

func (c *Client) Delete(path string) ([]byte, error) {
	b, _, err := c.do("DELETE", path, nil)
	return b, err
}

func (c *Client) DeleteQuery(path, query string) ([]byte, error) {
	if query != "" {
		path = path + "?" + query
	}
	b, _, err := c.do("DELETE", path, nil)
	return b, err
}

// LoginBasic authenticates with username/password and returns a token.
func (c *Client) LoginBasic(repo, username, password string) (string, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s/api/conan/%s/v2/users/authenticate", c.ServerURL(), repo), nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(username, password)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		var apiErr struct{ Error string `json:"error"` }
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			return "", fmt.Errorf("%s", apiErr.Error)
		}
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct{ Token string `json:"token"` }
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.Token, nil
}

// WebLogin authenticates via the web UI endpoint (admin users).
func (c *Client) WebLogin(username, password string) (string, error) {
	b, err := c.Post("/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	var result struct{ Token string `json:"token"` }
	if err := json.Unmarshal(b, &result); err != nil {
		return "", err
	}
	return result.Token, nil
}
