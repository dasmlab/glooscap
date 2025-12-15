package outline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout      = 15 * time.Second
	documentsListPath   = "/api/documents.list"
	documentsExportPath = "/api/documents.export"
	documentsCreatePath = "/api/documents.create"
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
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Slug       string    `json:"slug"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Language   string    `json:"language"`
	HasAssets  bool      `json:"hasAssets"`
	Collection string    `json:"collection,omitempty"`
	Template   string    `json:"template,omitempty"`
	IsTemplate bool      `json:"isTemplate,omitempty"` // True if this is a template definition
}

type documentsListResponse struct {
	Data []struct {
		ID           string    `json:"id"`
		Title        string    `json:"title"`
		Slug         string    `json:"urlId"`
		UpdatedAt    time.Time `json:"updatedAt"`
		IsDraft      bool      `json:"isDraft"`
		CollectionID string    `json:"collectionId,omitempty"`
		TemplateID   string    `json:"templateId,omitempty"`
	} `json:"data"`
}

type collectionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListPages fetches page summaries from Outline.
func (c *Client) ListPages(ctx context.Context) ([]PageSummary, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsListPath})

	payload := map[string]any{
		"direction": "DESC",
		"sort":      "updatedAt",
		"limit":     100, // Outline API maximum is 100
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("outline: marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("outline: new request: %w", err)
	}

	// Ensure token is trimmed of any whitespace
	token := strings.TrimSpace(c.token)

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("outline: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		bodyBytes, readErr := io.ReadAll(resp.Body)
		bodyStr := ""
		if readErr == nil {
			bodyStr = string(bodyBytes)
		}
		return nil, fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, bodyStr)
	}

	var list documentsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w", err)
	}

	pages := make([]PageSummary, 0, len(list.Data))

	// Fetch collections map if we have collection IDs
	collectionsMap := make(map[string]string)
	if len(list.Data) > 0 {
		// Try to fetch collection info (Outline API may require separate call)
		// For now, we'll extract from collectionId if available
		for _, item := range list.Data {
			if item.CollectionID != "" && collectionsMap[item.CollectionID] == "" {
				// Collection name would need to be fetched separately
				// For now, use the ID as placeholder
				collectionsMap[item.CollectionID] = item.CollectionID
			}
		}
	}

	for _, item := range list.Data {
		if item.IsDraft {
			continue
		}

		collectionName := ""
		if item.CollectionID != "" {
			collectionName = collectionsMap[item.CollectionID]
		}

		// Try to detect template from title (e.g., "Feature Completion Template (EN)")
		template := ""
		isTemplate := false
		if strings.Contains(item.Title, "Template") {
			// Extract template name (e.g., "Feature Completion Template" from "Feature Completion Template (EN)")
			parts := strings.Split(item.Title, "(")
			if len(parts) > 0 {
				template = strings.TrimSpace(parts[0])
			}
			// Mark as template if title contains "Template" - this is a heuristic
			// TODO: Check Outline API for actual template metadata if available
			isTemplate = true
		}

		pages = append(pages, PageSummary{
			ID:        item.ID,
			Title:     item.Title,
			Slug:      item.Slug,
			UpdatedAt: item.UpdatedAt,
			// Outline does not expose language directly; try to extract from title
			Language:   extractLanguageFromTitle(item.Title),
			HasAssets:  false,
			Collection: collectionName,
			Template:   template,
			IsTemplate: isTemplate,
		})
	}

	return pages, nil
}

// extractLanguageFromTitle tries to extract language code from page title
// e.g., "Feature Completion Template (EN)" -> "EN"
func extractLanguageFromTitle(title string) string {
	// Look for pattern like "(EN)", "(FR)", etc.
	parts := strings.Split(title, "(")
	if len(parts) < 2 {
		return ""
	}
	langPart := strings.TrimSpace(parts[len(parts)-1])
	langPart = strings.TrimSuffix(langPart, ")")
	langPart = strings.TrimSpace(langPart)

	// Validate it's a language code (2-3 uppercase letters)
	if len(langPart) >= 2 && len(langPart) <= 3 {
		for _, r := range langPart {
			if r < 'A' || r > 'Z' {
				return ""
			}
		}
		return langPart
	}
	return ""
}

// PageContent represents the full content of a page.
type PageContent struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Slug     string `json:"slug"`
	Markdown string `json:"markdown"`
}

type documentsExportResponse struct {
	Data string `json:"data"` // Markdown content
}

// GetPageContent fetches the full content of a page as Markdown.
// Uses POST /api/documents.export endpoint.
func (c *Client) GetPageContent(ctx context.Context, pageID string) (*PageContent, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsExportPath})

	payload := map[string]string{
		"id": pageID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("outline: marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("outline: new request: %w", err)
	}

	token := strings.TrimSpace(c.token)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("outline: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		bodyStr := ""
		if readErr == nil {
			bodyStr = string(bodyBytes)
		}
		return nil, fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, bodyStr)
	}

	// Read the full response body first to debug
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("outline: read response body: %w", err)
	}

	// Log raw response for debugging (first 1000 chars)
	bodyPreview := string(bodyBytes)
	if len(bodyPreview) > 1000 {
		bodyPreview = bodyPreview[:1000] + "..."
	}
	fmt.Printf("[outline] GetPageContent raw response for pageID=%s (status=%d): %q\n",
		pageID, resp.StatusCode, bodyPreview)

	var exportResp documentsExportResponse
	if err := json.Unmarshal(bodyBytes, &exportResp); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w (body: %s)", err, bodyPreview)
	}

	// Log the response for debugging (first 500 chars to avoid huge logs)
	markdownPreview := exportResp.Data
	if len(markdownPreview) > 500 {
		markdownPreview = markdownPreview[:500] + "..."
	}
	fmt.Printf("[outline] GetPageContent response for pageID=%s: markdown length=%d, preview=%q\n",
		pageID, len(exportResp.Data), markdownPreview)

	// We need to get page metadata separately to get title and slug
	// For now, we'll return what we have and the caller can enrich it
	return &PageContent{
		ID:       pageID,
		Markdown: exportResp.Data,
		// Title and Slug will need to be populated from PageSummary if available
	}, nil
}



// CreatePageRequest represents the request to create a new page.
type CreatePageRequest struct {
	Title      string `json:"title"`
	Text       string `json:"text"` // Markdown content
	CollectionID string `json:"collectionId,omitempty"` // Optional collection ID
	ParentDocumentID string `json:"parentDocumentId,omitempty"` // Optional parent document ID
}

// CreatePageResponse represents the response from creating a page.
type CreatePageResponse struct {
	Data struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Slug  string `json:"urlId"`
	} `json:"data"`
}

// CreatePage creates a new page in Outline with the given title and markdown content.
// Returns the created page ID, title, and slug.
// SAFETY: This method only creates new pages - it never modifies existing pages.
func (c *Client) CreatePage(ctx context.Context, req CreatePageRequest) (*CreatePageResponse, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsCreatePath})

	payload := map[string]any{
		"title": req.Title,
		"text":  req.Text,
	}
	if req.CollectionID != "" {
		payload["collectionId"] = req.CollectionID
	}
	if req.ParentDocumentID != "" {
		payload["parentDocumentId"] = req.ParentDocumentID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("outline: marshal request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("outline: new request: %w", err)
	}

	token := strings.TrimSpace(c.token)
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("outline: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		bodyStr := ""
		if readErr == nil {
			bodyStr = string(bodyBytes)
		}
		return nil, fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, bodyStr)
	}

	var createResp CreatePageResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w", err)
	}

	return &createResp, nil
}

// GetTemplate fetches a template document by ID.
// This is the same as GetPageContent but semantically indicates it's a template.
func (c *Client) GetTemplate(ctx context.Context, templateID string) (*PageContent, error) {
	return c.GetPageContent(ctx, templateID)
}
