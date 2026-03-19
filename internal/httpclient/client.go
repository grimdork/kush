// Package httpclient provides a shared HTTP client for builtins and Tengo scripts.
package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultTimeout for HTTP requests.
const DefaultTimeout = 30 * time.Second

var client = &http.Client{Timeout: DefaultTimeout}

// Response holds the result of an HTTP request.
type Response struct {
	Status     int
	StatusText string
	Headers    http.Header
	Body       []byte
}

// Request performs an HTTP request and returns a Response.
func Request(method, url string, body io.Reader, headers map[string]string) (*Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// Set a default User-Agent if not provided.
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "kush/1.0")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    resp.Header,
		Body:       data,
	}, nil
}

// Get performs an HTTP GET request.
func Get(url string, headers map[string]string) (*Response, error) {
	return Request("GET", url, nil, headers)
}

// Post performs an HTTP POST request.
func Post(url string, body io.Reader, headers map[string]string) (*Response, error) {
	return Request("POST", url, body, headers)
}

// Put performs an HTTP PUT request.
func Put(url string, body io.Reader, headers map[string]string) (*Response, error) {
	return Request("PUT", url, body, headers)
}

// Delete performs an HTTP DELETE request.
func Delete(url string, headers map[string]string) (*Response, error) {
	return Request("DELETE", url, nil, headers)
}

// Head performs an HTTP HEAD request.
func Head(url string, headers map[string]string) (*Response, error) {
	return Request("HEAD", url, nil, headers)
}

// Download fetches a URL and writes the body to a file. If outPath is "",
// it writes to stdout.
func Download(url, outPath string, headers map[string]string) error {
	resp, err := Get(url, headers)
	if err != nil {
		return err
	}
	if resp.Status >= 400 {
		return fmt.Errorf("HTTP %s", resp.StatusText)
	}

	if outPath == "" {
		_, err = os.Stdout.Write(resp.Body)
		return err
	}

	return os.WriteFile(outPath, resp.Body, 0644)
}

// PrintHeaders writes response headers to stdout in a readable format.
func PrintHeaders(h http.Header) {
	for k, vals := range h {
		for _, v := range vals {
			fmt.Printf("%s: %s\n", k, v)
		}
	}
}

// PrettyJSON formats JSON bytes with indentation and writes to stdout.
func PrettyJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		// Not JSON, just print raw
		os.Stdout.Write(data)
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// IsJSON checks if a content-type header indicates JSON.
func IsJSON(headers http.Header) bool {
	ct := headers.Get("Content-Type")
	return strings.Contains(ct, "json")
}
