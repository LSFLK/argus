package audit

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// SignPayloadFunc is a strategy for signing payloads.
// It allows the client to provide its own signing logic (e.g., KMS, File-based, etc.)
// without exposing private keys to the audit library.
type SignPayloadFunc func(ctx context.Context, payload []byte) (signature string, err error)

// CanonicalizeRequest serializes the AuditLogRequest deterministically.
// It ensures that signature fields are not included in the payload that gets signed.
func CanonicalizeRequest(event *AuditLogRequest) ([]byte, error) {
	// Create a shallow copy and clear signature fields
	eventCopy := *event
	eventCopy.Signature = ""
	eventCopy.SignatureAlgorithm = ""
	eventCopy.PublicKeyID = ""

	// json.Marshal guarantees struct fields are serialized in declaration order
	// json.RawMessage fields retain their exact bytes, ensuring deterministic hashing.
	// Message field ([]byte) is also included in serialization.
	return json.Marshal(&eventCopy)
}

// SignPayload hashes and signs the canonical payload using the provided signer.
func SignPayload(payload []byte, signer crypto.Signer) (string, string, error) {
	hash := sha256.Sum256(payload)

	switch signer.(type) {
	case *rsa.PrivateKey:
		sig, err := signer.Sign(rand.Reader, hash[:], crypto.SHA256)
		if err != nil {
			return "", "", fmt.Errorf("rsa signing failed: %w", err)
		}
		return base64.StdEncoding.EncodeToString(sig), "RS256", nil
	case ed25519.PrivateKey:
		// Ed25519 ignores the hash if passed, or uses the full message.
		// Standard crypto.Signer.Sign for Ed25519 expects the original message, not a hash.
		sig, err := signer.Sign(rand.Reader, payload, crypto.Hash(0))
		if err != nil {
			return "", "", fmt.Errorf("ed25519 signing failed: %w", err)
		}
		return base64.StdEncoding.EncodeToString(sig), "EdDSA", nil
	default:
		return "", "", errors.New("unsupported signer type (only RSA and Ed25519 are supported)")
	}
}

// VerifyPayload verifies the signature of the payload using the provided public key
func VerifyPayload(payload []byte, signatureBase64 string, alg string, publicKey crypto.PublicKey) error {
	sig, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("invalid base64 signature: %w", err)
	}

	hash := sha256.Sum256(payload)

	switch pk := publicKey.(type) {
	case *rsa.PublicKey:
		if alg != "RS256" {
			return fmt.Errorf("algorithm mismatch: expected RS256 for RSA public key, got %s", alg)
		}
		err := rsa.VerifyPKCS1v15(pk, crypto.SHA256, hash[:], sig)
		if err != nil {
			return fmt.Errorf("rsa verification failed: %w", err)
		}
		return nil
	case ed25519.PublicKey:
		if alg != "EdDSA" {
			return fmt.Errorf("algorithm mismatch: expected EdDSA for Ed25519 public key, got %s", alg)
		}
		if !ed25519.Verify(pk, payload, sig) {
			return errors.New("ed25519 verification failed")
		}
		return nil
	default:
		return errors.New("unsupported public key type (only RSA and Ed25519 are supported)")
	}
}
