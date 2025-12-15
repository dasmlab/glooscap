package server

import (
	"sync"
)

// TranslationServiceConfig holds runtime configuration for the translation service.
type TranslationServiceConfig struct {
	Address string `json:"address"` // gRPC address (e.g., "iskoces-service.iskoces.svc:50051")
	Type    string `json:"type"`    // Service type: "nanabush" or "iskoces"
	Secure  bool   `json:"secure"`  // Whether to use TLS/mTLS
}

// ConfigStore manages runtime configuration for the translation service.
type ConfigStore struct {
	mu                      sync.RWMutex
	translationServiceConfig *TranslationServiceConfig
}

// NewConfigStore creates a new ConfigStore.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{}
}

// GetTranslationServiceConfig returns the current translation service configuration.
// Returns Iskoces defaults if no configuration has been set.
func (s *ConfigStore) GetTranslationServiceConfig() *TranslationServiceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.translationServiceConfig == nil {
		// Return Iskoces defaults if no config has been set
		return &TranslationServiceConfig{
			Address: "iskoces-service.iskoces.svc:50051",
			Type:    "iskoces",
			Secure:  false,
		}
	}
	// Return a copy to prevent external modifications
	config := *s.translationServiceConfig
	return &config
}

// SetTranslationServiceConfig sets the translation service configuration.
func (s *ConfigStore) SetTranslationServiceConfig(config *TranslationServiceConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if config == nil {
		s.translationServiceConfig = nil
		return
	}
	// Store a copy
	cfg := *config
	s.translationServiceConfig = &cfg
}

