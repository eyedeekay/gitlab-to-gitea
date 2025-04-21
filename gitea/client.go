// client.go

// Package gitea provides a client for interacting with the Gitea API
package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client handles communication with the Gitea API
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	token      string
}

// VersionResponse represents the Gitea version response
type VersionResponse struct {
	Version string `json:"version"`
}

// FetchCSRFToken retrieves a CSRF token from Gitea
func (c *Client) FetchCSRFToken() error {
	// Create a request to the Gitea login page to get a CSRF token
	u, err := c.baseURL.Parse("/")
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Use a normal HTTP client without our custom transport for this request
	// to avoid circular dependency (we need the token for the transport)
	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Extract CSRF token from Set-Cookie header
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "_csrf" {
			// Update the transport with the CSRF token
			transport, ok := c.httpClient.Transport.(*CSRFTokenTransport)
			if ok {
				transport.CSRFToken = cookie.Value
				return nil
			}
			return fmt.Errorf("transport is not a CSRFTokenTransport")
		}
	}

	// If we couldn't find a CSRF token, try to extract it from the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Look for <meta name="_csrf" content="..." /> in the HTML
	body := string(bodyBytes)
	csrfMetaStart := strings.Index(body, `<meta name="_csrf" content="`)
	if csrfMetaStart != -1 {
		csrfMetaStart += len(`<meta name="_csrf" content="`)
		csrfMetaEnd := strings.Index(body[csrfMetaStart:], `"`)
		if csrfMetaEnd != -1 {
			csrfToken := body[csrfMetaStart : csrfMetaStart+csrfMetaEnd]

			transport, ok := c.httpClient.Transport.(*CSRFTokenTransport)
			if ok {
				transport.CSRFToken = csrfToken
				return nil
			}
			return fmt.Errorf("transport is not a CSRFTokenTransport")
		}
	}

	return fmt.Errorf("could not find CSRF token in response")
}

func NewClient(baseURL, token string) (*Client, error) {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Create a standard HTTP client without the custom transport
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &Client{
		baseURL:    u,
		httpClient: httpClient,
		token:      token,
	}, nil
}

// Add a custom transport to handle CSRF tokens
type CSRFTokenTransport struct {
	Token     string
	CSRFToken string
	Base      http.RoundTripper
}

func (t *CSRFTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add Content-Type header
	req.Header.Set("Content-Type", "application/json")
	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("token %s", t.Token))
	// Add Accept header
	req.Header.Set("Accept", "application/json")
	// Add X-Gitea-CSRF header with the token to bypass CSRF protection
	req.Header.Set("X-Gitea-CSRF", t.CSRFToken)

	return t.Base.RoundTrip(req)
}

// GetVersion retrieves the Gitea version
func (c *Client) GetVersion() (string, error) {
	versionResp := &VersionResponse{}
	_, err := c.request("GET", "version", nil, versionResp)
	if err != nil {
		return "", err
	}
	return versionResp.Version, nil
}

// Get performs a GET request against the API
func (c *Client) Get(path string, result interface{}) error {
	_, err := c.request("GET", path, nil, result)
	return err
}

// Post performs a POST request against the API
func (c *Client) Post(path string, data, result interface{}) error {
	_, err := c.request("POST", path, data, result)
	return err
}

// Put performs a PUT request against the API
func (c *Client) Put(path string, data, result interface{}) error {
	_, err := c.request("PUT", path, data, result)
	return err
}

// Patch performs a PATCH request against the API
func (c *Client) Patch(path string, data, result interface{}) error {
	_, err := c.request("PATCH", path, data, result)
	return err
}

// Delete performs a DELETE request against the API
func (c *Client) Delete(path string) error {
	_, err := c.request("DELETE", path, nil, nil)
	return err
}

// request sends an HTTP request to the Gitea API
func (c *Client) request(method, path string, data, result interface{}) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/api/v1/" + path
	}

	// Debug output to see what endpoint is being called
	fmt.Printf("Making %s request to: %s%s\n", method, c.baseURL.String(), path)

	u, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewBuffer(jsonData)

		// Debug output for request body
		// fmt.Printf("Request body: %s\n", string(jsonData))
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - simplified approach with just basic auth headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// For debugging: print the raw response
	bodyBytes, _ := io.ReadAll(resp.Body)
	//fmt.Printf("Response: Status=%s, Body=%s\n", resp.Status, string(bodyBytes))

	// Since we've read the body, we need to create a new reader for further processing
	//resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Handle error status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, fmt.Errorf("API returned error: %s - %s", resp.Status, string(bodyBytes))
	}

	if result != nil && len(bodyBytes) > 0 {
		if err := json.NewDecoder(bytes.NewBuffer(bodyBytes)).Decode(result); err != nil {
			return resp, fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return resp, nil
}
