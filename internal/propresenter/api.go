package propresenter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
)

// Client handles communication with ProPresenter REST API.
type Client struct {
	baseURL  string
	password string
	client   *http.Client
}

// NewClient creates a new ProPresenter API client.
func NewClient(baseURL, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		password: password,
		client:   &http.Client{},
	}
}

// TriggerPresentation adds/triggers a presentation by its full path.
// path should be the absolute path to the .pro file, e.g., /path/to/Libraries/Message.pro
func (c *Client) TriggerPresentation(path string) error {
	logger := slog.Default()

	// Extract the filename without extension (used as the presentation name)
	filename := filepath.Base(path)
	presentationName := filename

	// Construct request body
	reqBody := map[string]interface{}{
		"path": path,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	// Create request to load/trigger presentation
	endpoint := fmt.Sprintf("%s/api/v1/presentations/open", c.baseURL)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.password != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.password))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("api error: status %d, response: %s", resp.StatusCode, string(respBody))
	}

	logger.Info("triggered presentation", "path", path, "name", presentationName)
	return nil
}
