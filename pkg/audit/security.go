package audit

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
)

// SignPayloadFunc is a strategy for signing payloads.
// It allows the client to provide its own signing logic (e.g. KMS, File-based, etc.)
// without exposing private keys to the audit library.
type SignPayloadFunc func(ctx context.Context, payload []byte) (signature string, err error)

// CanonicalizeRequest creates a deterministic byte representation of an AuditLogRequest
// for cryptographic signing and verification.
//
// IMPORTANT: This uses a pipe-delimited format instead of JSON serialization.
// json.Marshal output is Go-specific (spacing, key ordering of maps, encoding of
// special characters) and is extremely difficult to reproduce byte-for-byte in other
// languages like Python or Node.js. By using a simple pipe-delimited format, any
// language in NSW's polyglot ecosystem can trivially compute the same canonical payload.
//
// Canonical format (fields separated by "|"):
//
//	TraceID|Timestamp|EventType|Action|Status|ActorType|ActorID|TargetType|TargetID|Message|MetadataJSON
//
// Rules:
//   - nil/empty pointer fields use the empty string ""
//   - Message bytes are base64-encoded (standard encoding, no padding trimming)
//   - Metadata map is serialized via json.Marshal (maps have sorted keys in Go 1.8+),
//     but since this is a simple map[string]interface{}, most languages can reproduce it.
//     An empty/nil map serializes as "{}"
func CanonicalizeRequest(event *AuditLogRequest) ([]byte, error) {
	traceID := ""
	if event.TraceID != nil {
		traceID = *event.TraceID
	}

	targetID := ""
	if event.TargetID != nil {
		targetID = *event.TargetID
	}

	// Base64-encode message bytes for safe textual representation
	msgEncoded := base64.StdEncoding.EncodeToString(event.Message)

	// Serialize metadata deterministically
	metadataJSON := "{}"
	if event.Metadata != nil {
		metaBytes, err := json.Marshal(event.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata for canonicalization: %w", err)
		}
		metadataJSON = string(metaBytes)
	}

	canonical := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		traceID,
		event.Timestamp,
		event.EventType,
		event.Action,
		event.Status,
		event.ActorType,
		event.ActorID,
		event.TargetType,
		targetID,
		msgEncoded,
		metadataJSON,
	)

	return []byte(canonical), nil
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

// SignPayloadPEM signs arbitrary bytes using a PEM-encoded RSA or Ed25519 private key.
// It returns the base64-encoded signature, the signature algorithm ("RS256" or "EdDSA"), and any error.
func SignPayloadPEM(payload []byte, privateKeyPEM []byte) (string, string, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return "", "", errors.New("failed to decode PEM block")
	}

	var key crypto.Signer
	var err error

	// Try parsing PKCS#8 first (standard Go key generation output)
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		var ok bool
		key, ok = parsedKey.(crypto.Signer)
		if !ok {
			return "", "", errors.New("parsed key does not implement crypto.Signer")
		}
	} else {
		// Try parsing PKCS#1 (RSA specific)
		rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err == nil {
			key = rsaKey
		} else {
			// Try EC private key (e.g. Ed25519 standard DER representation if formatted as PKCS8)
			return "", "", fmt.Errorf("unsupported or malformed private key format: %w", err)
		}
	}

	return SignPayload(payload, key)
}

// VerifyPayloadPEM verifies a base64-encoded signature of a payload using a PEM-encoded RSA or Ed25519 public key.
func VerifyPayloadPEM(payload []byte, signatureBase64 string, algorithm string, publicKeyPEM []byte) error {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return errors.New("failed to decode PEM block")
	}

	var key crypto.PublicKey
	var err error

	// Try standard PKIX parsing first
	key, err = x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try PKCS#1 (RSA public key specific)
		key, err = x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return VerifyPayload(payload, signatureBase64, algorithm, key)
}
