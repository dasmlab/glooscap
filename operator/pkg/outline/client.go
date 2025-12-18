package outline

import (
	"bytes"
	"context"
	"crypto/tls"
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
	defaultTimeout        = 15 * time.Second
	documentsListPath     = "/api/documents.list"
	documentsExportPath   = "/api/documents.export"
	documentsCreatePath   = "/api/documents.create"
	documentsUpdatePath   = "/api/documents.update"
	documentsDeletePath   = "/api/documents.delete"
	collectionsListPath   = "/api/collections.list"
	collectionsCreatePath = "/api/collections.create"
)

// Client interacts with an Outline instance.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	token      string
}

// Config contains Outline client settings.
type Config struct {
	BaseURL              string
	Token                string
	Timeout              time.Duration
	InsecureSkipTLSVerify bool
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

	// Configure HTTP client with TLS settings
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipTLSVerify,
		},
	}

	// Log TLS configuration for debugging
	if cfg.InsecureSkipTLSVerify {
		fmt.Printf("[outline] Creating client with InsecureSkipTLSVerify=true for %s\n", cfg.BaseURL)
	}

	return &Client{
		baseURL:    u,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		token: cfg.Token,
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
	IsDraft    bool      `json:"isDraft,omitempty"`    // True if this page is a draft
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

// ListPages fetches page summaries from Outline with pagination support.
// If collectionID is provided, only fetches pages from that collection.
func (c *Client) ListPages(ctx context.Context, collectionID ...string) ([]PageSummary, error) {
	var allPages []PageSummary
	offset := 0
	limit := 100 // Outline API maximum is 100 per request
	
	// If collectionID is provided, use it
	var targetCollectionID string
	if len(collectionID) > 0 && collectionID[0] != "" {
		targetCollectionID = collectionID[0]
		fmt.Printf("[outline] ListPages: filtering by collection ID: %s\n", targetCollectionID)
	}

	for {
		reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsListPath})

		payload := map[string]any{
			"direction": "DESC",
			"sort":      "updatedAt",
			"limit":     limit,
			"offset":    offset,
		}
		
		// Add collection filter if specified
		if targetCollectionID != "" {
			payload["collectionId"] = targetCollectionID
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

		// If no data returned, we've reached the end
		if len(list.Data) == 0 {
			break
		}

		pages := make([]PageSummary, 0, len(list.Data))

		// Fetch collections map to get collection names
		collectionsMap := make(map[string]string)
		if len(list.Data) > 0 {
			// Fetch all collections to map IDs to names
			collections, collErr := c.ListCollections(ctx)
			if collErr == nil {
				for _, coll := range collections {
					collectionsMap[coll.ID] = coll.Name
				}
			}
			// Fallback: use collection ID as name if we couldn't fetch collections
			for _, item := range list.Data {
				if item.CollectionID != "" && collectionsMap[item.CollectionID] == "" {
					collectionsMap[item.CollectionID] = item.CollectionID
				}
			}
		}

		for _, item := range list.Data {
		// Include both drafts and published pages (removed draft filter)
		// This allows diagnostic jobs to find and update existing draft pages

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
				IsDraft:    item.IsDraft,
			})
		}

		allPages = append(allPages, pages...)
		
		// If we got fewer than the limit, we've reached the end
		if len(list.Data) < limit {
			break
		}
		
		// Increment offset for next page
		offset += limit
		fmt.Printf("[outline] ListPages: fetched %d pages so far (offset: %d)\n", len(allPages), offset)
	}

	fmt.Printf("[outline] ListPages: total pages fetched: %d\n", len(allPages))
	return allPages, nil
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
	Title            string `json:"title"`
	Text             string `json:"text"`                       // Markdown content
	CollectionID     string `json:"collectionId,omitempty"`     // Optional collection ID
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

	// Read response body for debugging
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("outline: read response body: %w", readErr)
	}

	bodyStr := string(bodyBytes)
	if resp.StatusCode != http.StatusOK {
		// Log first 500 chars of error response
		errorPreview := bodyStr
		if len(errorPreview) > 500 {
			errorPreview = errorPreview[:500] + "..."
		}
		fmt.Printf("[outline] CreatePage error response (status=%d): %q\n", resp.StatusCode, errorPreview)
		return nil, fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, errorPreview)
	}

	// Log successful response for debugging
	responsePreview := bodyStr
	if len(responsePreview) > 500 {
		responsePreview = responsePreview[:500] + "..."
	}
	fmt.Printf("[outline] CreatePage raw response (status=%d): %q\n", resp.StatusCode, responsePreview)

	var createResp CreatePageResponse
	if err := json.Unmarshal(bodyBytes, &createResp); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w (body: %s)", err, responsePreview)
	}

	fmt.Printf("[outline] CreatePage parsed response: id=%s, title=%s, slug=%s\n",
		createResp.Data.ID, createResp.Data.Title, createResp.Data.Slug)

	return &createResp, nil
}

// GetTemplate fetches a template document by ID.
// This is the same as GetPageContent but semantically indicates it's a template.
func (c *Client) GetTemplate(ctx context.Context, templateID string) (*PageContent, error) {
	return c.GetPageContent(ctx, templateID)
}

// PublishPageRequest represents the request to publish a draft page.
type PublishPageRequest struct {
	ID string `json:"id"` // Document ID
}

// PublishPageResponse represents the response from publishing a page.
type PublishPageResponse struct {
	Data struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Slug  string `json:"urlId"`
	} `json:"data"`
}

// PublishPage publishes a draft page in Outline.
// This converts a draft document to a published document.
func (c *Client) PublishPage(ctx context.Context, req PublishPageRequest) (*PublishPageResponse, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsUpdatePath})

	payload := map[string]any{
		"id":      req.ID,
		"publish": true, // Publish the document
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

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("outline: read response body: %w", readErr)
	}

	bodyStr := string(bodyBytes)
	if resp.StatusCode != http.StatusOK {
		errorPreview := bodyStr
		if len(errorPreview) > 500 {
			errorPreview = errorPreview[:500] + "..."
		}
		fmt.Printf("[outline] PublishPage error response (status=%d): %q\n", resp.StatusCode, errorPreview)
		return nil, fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, errorPreview)
	}

	var publishResp PublishPageResponse
	if err := json.Unmarshal(bodyBytes, &publishResp); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w (body: %s)", err, bodyStr)
	}

	fmt.Printf("[outline] PublishPage success: id=%s, title=%s, slug=%s\n",
		publishResp.Data.ID, publishResp.Data.Title, publishResp.Data.Slug)

	return &publishResp, nil
}

// Collection represents a collection in Outline.
type Collection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListCollectionsResponse represents the response from listing collections.
type ListCollectionsResponse struct {
	Data []Collection `json:"data"`
}

// ListCollections fetches all collections from Outline.
func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: collectionsListPath})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader([]byte("{}")))
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

	var listResp ListCollectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w", err)
	}

	return listResp.Data, nil
}

// CreateCollectionRequest represents the request to create a collection.
type CreateCollectionRequest struct {
	Name string `json:"name"`
}

// CreateCollectionResponse represents the response from creating a collection.
type CreateCollectionResponse struct {
	Data Collection `json:"data"`
}

// CreateCollection creates a new collection in Outline.
func (c *Client) CreateCollection(ctx context.Context, req CreateCollectionRequest) (*CreateCollectionResponse, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: collectionsCreatePath})

	payload := map[string]any{
		"name": req.Name,
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

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("outline: read response body: %w", readErr)
	}

	bodyStr := string(bodyBytes)
	if resp.StatusCode != http.StatusOK {
		errorPreview := bodyStr
		if len(errorPreview) > 500 {
			errorPreview = errorPreview[:500] + "..."
		}
		fmt.Printf("[outline] CreateCollection error response (status=%d): %q\n", resp.StatusCode, errorPreview)
		return nil, fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, errorPreview)
	}

	var createResp CreateCollectionResponse
	if err := json.Unmarshal(bodyBytes, &createResp); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w (body: %s)", err, bodyStr)
	}

	fmt.Printf("[outline] CreateCollection success: id=%s, name=%s\n", createResp.Data.ID, createResp.Data.Name)

	return &createResp, nil
}

// GetOrCreateCollection gets a collection by name, or creates it if it doesn't exist.
// Retries on network errors with exponential backoff.
func (c *Client) GetOrCreateCollection(ctx context.Context, name string) (string, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			fmt.Printf("[outline] Retrying GetOrCreateCollection (attempt %d/%d) after %v...\n", attempt+1, maxRetries, backoff)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		// List all collections
		collections, err := c.ListCollections(ctx)
		if err != nil {
			lastErr = fmt.Errorf("outline: list collections: %w", err)
			// Check if it's a network error that we should retry
			if strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "EOF") ||
				strings.Contains(err.Error(), "connection") {
				continue // Retry
			}
			return "", lastErr
		}

		// Check if collection exists
		for _, coll := range collections {
			if coll.Name == name {
				fmt.Printf("[outline] Collection '%s' already exists with ID: %s\n", name, coll.ID)
				return coll.ID, nil
			}
		}

		// Create collection if it doesn't exist
		fmt.Printf("[outline] Collection '%s' not found, creating...\n", name)
		createResp, err := c.CreateCollection(ctx, CreateCollectionRequest{Name: name})
		if err != nil {
			lastErr = fmt.Errorf("outline: create collection: %w", err)
			// Check if it's a network error that we should retry
			if strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "EOF") ||
				strings.Contains(err.Error(), "connection") {
				continue // Retry
			}
			return "", lastErr
		}

		return createResp.Data.ID, nil
	}

	return "", fmt.Errorf("outline: failed after %d attempts: %w", maxRetries, lastErr)
}

// UpdatePageRequest represents the request to update an existing page.
type UpdatePageRequest struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text,omitempty"`
}

// UpdatePageResponse represents the response from updating a page.
type UpdatePageResponse struct {
	Data struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Slug  string `json:"urlId"`
	} `json:"data"`
}

// UpdatePage updates an existing page in Outline.
func (c *Client) UpdatePage(ctx context.Context, req UpdatePageRequest) (*UpdatePageResponse, error) {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsUpdatePath})

	payload := map[string]any{
		"id": req.ID,
	}
	if req.Title != "" {
		payload["title"] = req.Title
	}
	if req.Text != "" {
		payload["text"] = req.Text
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

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("outline: read response body: %w", readErr)
	}

	bodyStr := string(bodyBytes)
	if resp.StatusCode != http.StatusOK {
		errorPreview := bodyStr
		if len(errorPreview) > 500 {
			errorPreview = errorPreview[:500] + "..."
		}
		fmt.Printf("[outline] UpdatePage error response (status=%d): %q\n", resp.StatusCode, errorPreview)
		return nil, fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, errorPreview)
	}

	var updateResp UpdatePageResponse
	if err := json.Unmarshal(bodyBytes, &updateResp); err != nil {
		return nil, fmt.Errorf("outline: decode response: %w (body: %s)", err, bodyStr)
	}

	fmt.Printf("[outline] UpdatePage success: id=%s, title=%s, slug=%s\n",
		updateResp.Data.ID, updateResp.Data.Title, updateResp.Data.Slug)

	return &updateResp, nil
}

// DeletePage deletes a page in Outline.
func (c *Client) DeletePage(ctx context.Context, pageID string) error {
	reqURL := c.baseURL.ResolveReference(&url.URL{Path: documentsDeletePath})

	payload := map[string]any{
		"id": pageID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("outline: marshal request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("outline: new request: %w", err)
	}

	token := strings.TrimSpace(c.token)
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("outline: request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("outline: read response body: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		bodyStr := string(bodyBytes)
		errorPreview := bodyStr
		if len(errorPreview) > 500 {
			errorPreview = errorPreview[:500] + "..."
		}
		fmt.Printf("[outline] DeletePage error response (status=%d): %q\n", resp.StatusCode, errorPreview)
		return fmt.Errorf("outline: unexpected status code %d: %s", resp.StatusCode, errorPreview)
	}

	// Outline API returns success even if the page doesn't exist
	return nil
}
