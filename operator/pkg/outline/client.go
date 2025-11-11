package outline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultTimeout    = 15 * time.Second
	documentsListPath = "/api/documents.list"
)

// Client interacts with an Outline instance.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	token      string
}

// Config contains Outline client settings.
type Config struct {
	BaseURL string
	Token   string
	Timeout time.Duration
}

// NewClient creates a new Outline client using the provided config.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("outline: base URL is required")
	}
	if cfg.Token == "" {
		return nil, errors.New("outline: API token is required")
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("outline: parse base url: %w", err)
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &Client{
		baseURL:    u,
		httpClient: &http.Client{Timeout: timeout},
		token:      cfg.Token,
	}, nil
}

// PageSummary represents minimal metadata for a wiki page.
type PageSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	UpdatedAt time.Time `json:"updatedAt"`
	Language  string    `json:"language"`
	HasAssets bool      `json:"hasAssets"`
}

type documentsListResponse struct {
	Data []struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Slug      string    `json:"urlId"`
		UpdatedAt time.Time `json:"updatedAt"`
		IsDraft   bool      `json:"isDraft"`
	} `json:"data"`
}

// ListPages fetches page summaries from Outline.
func (c *Client) ListPages(ctx context.Context) ([]PageSummary, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsListPath})

	payload := map[string]any{
		"direction": "DESC",
		"sort":      "updatedAt",
		"limit":     200,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("outline: marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("outline: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("outline: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("outline: unexpected status code %d", resp.StatusCode)
	}

	var list documentsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w", err)
	}

	pages := make([]PageSummary, 0, len(list.Data))
	for _, item := range list.Data {
		if item.IsDraft {
			continue
		}
		pages = append(pages, PageSummary{
			ID:        item.ID,
			Title:     item.Title,
			Slug:      item.Slug,
			UpdatedAt: item.UpdatedAt,
			// Outline does not expose language directly; default to empty string.
			Language:  "",
			HasAssets: false,
		})
	}

	return pages, nil
}

