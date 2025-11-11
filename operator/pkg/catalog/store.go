package catalog

import (
	"context"
	"sync"
	"time"
)

// Page represents a discovered wiki page summary.
type Page struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	UpdatedAt time.Time `json:"updatedAt"`
	Language  string    `json:"language"`
	HasAssets bool      `json:"hasAssets"`
}

// Store maintains in-memory catalogues of wiki targets.
type Store struct {
	mu      sync.RWMutex
	targets map[string][]Page
	meta    map[string]Target
}

// NewStore creates a new catalogue store.
func NewStore() *Store {
	return &Store{
		targets: make(map[string][]Page),
		meta:    make(map[string]Target),
	}
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
func (s *Store) Update(target string, info Target, pages []Page) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta[target] = info
	s.targets[target] = append([]Page(nil), pages...)
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
func (s *Store) List(target string) []Page {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if target == "" {
		flat := make([]Page, 0)
		for _, pages := range s.targets {
			flat = append(flat, pages...)
		}
		return flat
	}

	pages := s.targets[target]
	return append([]Page(nil), pages...)
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

