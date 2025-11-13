package catalog

import (
	"context"
	"sync"
	"time"
)

// Page represents a discovered wiki page with full tracking information.
type Page struct {
	// Core identification
	ID        string    `json:"id"`        // Outline page ID
	Title     string    `json:"title"`    // Page title
	Slug      string    `json:"slug"`     // URL slug
	URI       string    `json:"uri"`      // Full URI to the page
	WikiTarget string   `json:"wikiTarget"` // WikiTarget name (namespace/name format)
	
	// State tracking
	State     string    `json:"state"`     // State: discovered, translated, failed, etc.
	LastChecked time.Time `json:"lastChecked"` // When we last checked this page
	UpdatedAt time.Time `json:"updatedAt"` // When the page was last updated in the wiki (from Outline)
	
	// Translation tracking
	AutoTranslated bool   `json:"autoTranslated"` // Whether translation has been done
	TranslationURI string `json:"translationURI,omitempty"` // URI to translated page if exists
	
	// Metadata
	Language  string    `json:"language"`  // Language code (EN, FR, ES, etc.)
	HasAssets bool      `json:"hasAssets"` // Whether page has embedded assets
	Collection string   `json:"collection,omitempty"` // Collection name the page belongs to
	Template   string   `json:"template,omitempty"`  // Template type (e.g., "Feature Completion Template")
}

// Store maintains in-memory catalogues of wiki targets with CRUD operations.
type Store struct {
	mu          sync.RWMutex
	pages       map[string]*Page  // Keyed by page URI for fast lookup
	targets     map[string][]*Page // Grouped by target ID
	meta        map[string]Target  // Target metadata
	updateNotifier chan struct{}  // Channel to notify of updates (non-blocking)
}

// NewStore creates a new catalogue store.
func NewStore() *Store {
	return &Store{
		pages:          make(map[string]*Page),
		targets:        make(map[string][]*Page),
		meta:           make(map[string]Target),
		updateNotifier: make(chan struct{}, 1), // Buffered to avoid blocking
	}
}

// NotifyUpdate returns a channel that receives notifications when the store is updated.
func (s *Store) NotifyUpdate() <-chan struct{} {
	return s.updateNotifier
}

// Target contains metadata about a WikiTarget.
type Target struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	URI       string `json:"uri"`
}

// Update sets the catalogue pages and metadata for a given target.
// Pages are indexed by URI for fast lookup and deduplication.
func (s *Store) Update(target string, info Target, pages []Page) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	s.meta[target] = info
	
	// Clear existing pages for this target
	if existing, ok := s.targets[target]; ok {
		for _, page := range existing {
			delete(s.pages, page.URI)
		}
	}
	
	// Add new pages, indexed by URI
	targetPages := make([]*Page, 0, len(pages))
	for _, page := range pages {
		// Check if page already exists (by URI)
		if existing, exists := s.pages[page.URI]; exists {
			// Update existing page but preserve translation state
			existing.Title = page.Title
			existing.Slug = page.Slug
			existing.UpdatedAt = page.UpdatedAt
			existing.LastChecked = now
			existing.Language = page.Language
			existing.HasAssets = page.HasAssets
			existing.Collection = page.Collection
			existing.Template = page.Template
			existing.State = "discovered"
			targetPages = append(targetPages, existing)
		} else {
			// Create new page entry
			newPage := &Page{
				ID:            page.ID,
				Title:         page.Title,
				Slug:          page.Slug,
				URI:           page.URI,
				WikiTarget:    target,
				State:         "discovered",
				LastChecked:   now,
				UpdatedAt:     page.UpdatedAt,
				AutoTranslated: false,
				Language:      page.Language,
				HasAssets:     page.HasAssets,
				Collection:    page.Collection,
				Template:      page.Template,
			}
			s.pages[page.URI] = newPage
			targetPages = append(targetPages, newPage)
		}
	}
	s.targets[target] = targetPages
	
	// Notify listeners of update (non-blocking)
	select {
	case s.updateNotifier <- struct{}{}:
	default:
		// Channel full, skip notification
	}
}

// GetPage retrieves a page by URI.
func (s *Store) GetPage(uri string) (*Page, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	page, ok := s.pages[uri]
	return page, ok
}

// UpdatePage updates a specific page in the store.
func (s *Store) UpdatePage(page *Page) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if page.URI != "" {
		s.pages[page.URI] = page
		// Also update in targets map
		if targetPages, ok := s.targets[page.WikiTarget]; ok {
			for i, p := range targetPages {
				if p.URI == page.URI {
					targetPages[i] = page
					break
				}
			}
		}
	}
}

// DeletePage removes a page from the store.
func (s *Store) DeletePage(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if page, ok := s.pages[uri]; ok {
		delete(s.pages, uri)
		// Remove from targets map
		if targetPages, ok := s.targets[page.WikiTarget]; ok {
			for i, p := range targetPages {
				if p.URI == uri {
					s.targets[page.WikiTarget] = append(targetPages[:i], targetPages[i+1:]...)
					break
				}
			}
		}
	}
}

// Targets returns the list of known target identifiers.
func (s *Store) Targets() []Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Target, 0, len(s.meta))
	for _, meta := range s.meta {
		out = append(out, meta)
	}
	return out
}

// List returns the pages for a target. When the target is empty,
// all pages for all targets are returned.
func (s *Store) List(target string) []*Page {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if target == "" {
		flat := make([]*Page, 0)
		for _, pages := range s.targets {
			flat = append(flat, pages...)
		}
		return flat
	}

	pages := s.targets[target]
	result := make([]*Page, len(pages))
	copy(result, pages)
	return result
}

// WithContext returns a context that carries the catalogue store.
func WithContext(ctx context.Context, store *Store) context.Context {
	return context.WithValue(ctx, storeKey{}, store)
}

// FromContext extracts the store from context.
func FromContext(ctx context.Context) *Store {
	if v := ctx.Value(storeKey{}); v != nil {
		if store, ok := v.(*Store); ok {
			return store
		}
	}
	return nil
}

type storeKey struct{}

