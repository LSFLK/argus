package services

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
)

// PublicKeyRegistry manages the trusted public keys for log verification
type PublicKeyRegistry struct {
	keys map[string]crypto.PublicKey
	mu   sync.RWMutex
}

// NewPublicKeyRegistry creates a new registry
func NewPublicKeyRegistry() *PublicKeyRegistry {
	return &PublicKeyRegistry{
		keys: make(map[string]crypto.PublicKey),
	}
}

// RegisterKey registers a public key with an ID
func (r *PublicKeyRegistry) RegisterKey(id string, key crypto.PublicKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[id] = key
}

// GetKey retrieves a public key by ID
func (r *PublicKeyRegistry) GetKey(id string) (crypto.PublicKey, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key, ok := r.keys[id]
	return key, ok
}

// LoadKeyFromFile loads a PEM-encoded public key from a file
func (r *PublicKeyRegistry) LoadKeyFromFile(id, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block from %s", path)
	}

	var key crypto.PublicKey
	var parseErr error

	key, parseErr = x509.ParsePKIXPublicKey(block.Bytes)
	if parseErr != nil {
		// Try parsing as RSA public key specifically if PKIX fails
		key, parseErr = x509.ParsePKCS1PublicKey(block.Bytes)
		if parseErr != nil {
			return fmt.Errorf("failed to parse public key: %w", parseErr)
		}
	}

	r.RegisterKey(id, key)
	return nil
}
