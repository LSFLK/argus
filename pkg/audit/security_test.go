package audit

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"
)

func TestCanonicalizeRequest(t *testing.T) {
	req := &AuditLogRequest{
		TraceID:            func(s string) *string { return &s }("trace-123"),
		Timestamp:          "2023-01-01T00:00:00Z",
		Status:             StatusSuccess,
		ActorType:          "SERVICE",
		ActorID:            "actor-1",
		TargetType:         "SERVICE",
		TargetID:           func(s string) *string { return &s }("target-1"),
		Metadata:           map[string]interface{}{"key": "value"},
		Signature:          "should-be-stripped",
		SignatureAlgorithm: "RS256",
		PublicKeyID:        "key-1",
	}

	b, err := CanonicalizeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	canonical := string(b)

	// The canonical format is pipe-delimited and should never contain signature fields
	if bytes.Contains(b, []byte("should-be-stripped")) {
		t.Errorf("expected signature value to not appear in canonical output")
	}
	if bytes.Contains(b, []byte("RS256")) {
		t.Errorf("expected signatureAlgorithm to not appear in canonical output")
	}
	if bytes.Contains(b, []byte("key-1")) {
		t.Errorf("expected publicKeyId to not appear in canonical output")
	}

	// Verify key fields ARE present
	if !bytes.Contains(b, []byte("trace-123")) {
		t.Errorf("expected traceId in canonical output, got: %s", canonical)
	}
	if !bytes.Contains(b, []byte("actor-1")) {
		t.Errorf("expected actorId in canonical output, got: %s", canonical)
	}
	if !bytes.Contains(b, []byte("target-1")) {
		t.Errorf("expected targetId in canonical output, got: %s", canonical)
	}
}

func TestSignAndVerify_RSA(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	payload := []byte(`{"event":"test"}`)

	sigBase64, alg, err := SignPayload(payload, privateKey)
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}

	if alg != "RS256" {
		t.Errorf("expected algorithm RS256, got %s", alg)
	}

	err = VerifyPayload(payload, sigBase64, alg, &privateKey.PublicKey)
	if err != nil {
		t.Errorf("verification failed: %v", err)
	}
}

func TestSignAndVerify_Ed25519(t *testing.T) {
	// Generate an Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 key: %v", err)
	}

	payload := []byte(`{"event":"test"}`)

	// Using the private key which implements crypto.Signer
	sigBase64, alg, err := SignPayload(payload, priv)
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}

	if alg != "EdDSA" {
		t.Errorf("expected algorithm EdDSA, got %s", alg)
	}

	err = VerifyPayload(payload, sigBase64, alg, pub)
	if err != nil {
		t.Errorf("verification failed: %v", err)
	}
}

func TestVerifyPayload_Mismatch(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	t.Log("Successfully generated first RSA key")

	payload := []byte(`{"event":"test"}`)
	sigBase64, alg, err := SignPayload(payload, privateKey)
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}
	t.Logf("Successfully signed payload with %s", alg)

	privateKey2, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate second RSA key: %v", err)
	}
	t.Log("Successfully generated second RSA key")

	err = VerifyPayload(payload, sigBase64, alg, &privateKey2.PublicKey)
	if err == nil {
		t.Errorf("expected verification to fail with wrong key")
	} else {
		t.Logf("Verification failed as expected: %v", err)
	}
}

func TestVerifyPayload_Tampering(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	payload := []byte(`{"event":"test"}`)
	sigBase64, alg, _ := SignPayload(payload, privateKey)

	// Tamper with payload
	tamperedPayload := []byte(`{"event":"test!"}`)
	err := VerifyPayload(tamperedPayload, sigBase64, alg, &privateKey.PublicKey)
	if err == nil {
		t.Errorf("expected verification to fail for tampered payload")
	}
}

func TestVerifyPayload_InvalidSignature(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	payload := []byte(`{"event":"test"}`)

	// Completely invalid base64
	err := VerifyPayload(payload, "!!!", "EdDSA", publicKey)
	if err == nil {
		t.Errorf("expected error for invalid base64 signature")
	}

	// Valid base64 but invalid signature data
	err = VerifyPayload(payload, "bm90IGEgc2lnbmF0dXJl", "EdDSA", publicKey)
	if err == nil {
		t.Errorf("expected verification to fail for invalid signature data")
	}
}

func TestVerifyPayload_AlgorithmMismatch(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	payload := []byte(`{"event":"test"}`)
	sigBase64, _, _ := SignPayload(payload, privateKey)

	// Try to verify RSA signature using EdDSA algorithm identifier
	err := VerifyPayload(payload, sigBase64, "EdDSA", &privateKey.PublicKey)
	if err == nil {
		t.Errorf("expected error for algorithm mismatch (RSA key with EdDSA alg)")
	}

	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	// Try to verify with Ed25519 key but RS256 algorithm
	err = VerifyPayload(payload, sigBase64, "RS256", pub)
	if err == nil {
		t.Errorf("expected error for algorithm mismatch (Ed25519 key with RS256 alg)")
	}
}

func TestSignPayload_UnsupportedKey(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	// A mock signer that is not RSA or Ed25519
	type unsupportedSigner struct{ crypto.Signer }
	_, _, err := SignPayload(payload, unsupportedSigner{})
	if err == nil {
		t.Errorf("expected error for unsupported signer type")
	}
}

func TestCanonicalizeRequest_Consistency(t *testing.T) {
	req1 := &AuditLogRequest{ActorID: "actor", Status: StatusSuccess}
	req2 := &AuditLogRequest{ActorID: "actor", Status: StatusSuccess, Signature: "sig"}

	b1, _ := CanonicalizeRequest(req1)
	b2, _ := CanonicalizeRequest(req2)

	if string(b1) != string(b2) {
		t.Errorf("canonicalization should be invariant to signature fields")
	}
}

func TestClient_InterfaceImplementation(t *testing.T) {
	// This test ensures Client implements Auditor interface
	var _ Auditor = (*Client)(nil)
}

func TestClient_SignAndVerify(t *testing.T) {
	ctx := context.Background()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	signer := func(ctx context.Context, payload []byte) (string, error) {
		sig, _, err := SignPayload(payload, priv)
		return sig, err
	}

	client := NewClient(Config{
		BaseURL:            "http://localhost:8080",
		Signer:             signer,
		PublicKeyID:        "test-key-1",
		SignatureAlgorithm: "RS256",
	})
	defer client.Close(ctx)

	req := &AuditLogRequest{
		ActorID: "test-actor",
		Status:  StatusSuccess,
	}

	// Sign
	err = client.SignEvent(req)
	if err != nil {
		t.Fatalf("SignEvent failed: %v", err)
	}

	if req.Signature == "" {
		t.Error("expected signature to be populated")
	}
	if req.PublicKeyID != "test-key-1" {
		t.Errorf("expected PublicKeyID test-key-1, got %s", req.PublicKeyID)
	}
	if req.SignatureAlgorithm != "RS256" {
		t.Errorf("expected SignatureAlgorithm RS256, got %s", req.SignatureAlgorithm)
	}

	// Verify
	ok, err := client.VerifyIntegrity(req, &priv.PublicKey)
	if err != nil {
		t.Errorf("VerifyIntegrity failed: %v", err)
	}
	if !ok {
		t.Error("expected integrity to be verified")
	}

	// Tamper
	req.ActorID = "hacker"
	ok, err = client.VerifyIntegrity(req, &priv.PublicKey)
	if err == nil && ok {
		t.Error("expected verification to fail after tampering")
	}
}
func TestCanonicalizeRequest_DeepSorting(t *testing.T) {
	// Nested JSON in Metadata should be sorted canonically
	req1 := &AuditLogRequest{
		Metadata: map[string]interface{}{"z": "last", "a": "first", "m": "middle"},
	}
	req2 := &AuditLogRequest{
		Metadata: map[string]interface{}{"a": "first", "m": "middle", "z": "last"},
	}

	b1, err := CanonicalizeRequest(req1)
	if err != nil {
		t.Fatalf("unexpected error req1: %v", err)
	}
	b2, err := CanonicalizeRequest(req2)
	if err != nil {
		t.Fatalf("unexpected error req2: %v", err)
	}

	if string(b1) != string(b2) {
		t.Errorf("expected canonicalization to sort keys in Metadata")
	}

	// The metadata JSON segment within the pipe-delimited string should have sorted keys
	if !bytes.Contains(b1, []byte(`"a":"first","m":"middle","z":"last"`)) {
		t.Errorf("expected sorted metadata in output, got %s", string(b1))
	}
}

func TestClient_AlgorithmValidation(t *testing.T) {
	signer := func(ctx context.Context, payload []byte) (string, error) {
		return "sig", nil
	}

	t.Run("Valid RS256", func(t *testing.T) {
		client := NewClient(Config{
			BaseURL:            "http://localhost:8080",
			Signer:             signer,
			SignatureAlgorithm: "RS256",
		})
		if !client.IsEnabled() {
			t.Error("expected client to be enabled with RS256")
		}
	})

	t.Run("Valid EdDSA", func(t *testing.T) {
		client := NewClient(Config{
			BaseURL:            "http://localhost:8080",
			Signer:             signer,
			SignatureAlgorithm: "EdDSA",
		})
		if !client.IsEnabled() {
			t.Error("expected client to be enabled with EdDSA")
		}
	})

	t.Run("Invalid Algorithm", func(t *testing.T) {
		client := NewClient(Config{
			BaseURL:            "http://localhost:8080",
			Signer:             signer,
			SignatureAlgorithm: "MD5", // Unsupported/Insecure
		})
		if client.IsEnabled() {
			t.Error("expected client to be disabled with unsupported algorithm")
		}
	})
}

func TestClient_SignRetry(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	signer := func(ctx context.Context, payload []byte) (string, error) {
		attempts++
		if attempts < 3 {
			return "", fmt.Errorf("transient error")
		}
		return "final-sig", nil
	}

	client := NewClient(Config{
		BaseURL:            "http://localhost:8080",
		Signer:             signer,
		SignatureAlgorithm: "RS256",
		WorkerCount:        1,
	})

	req := &AuditLogRequest{
		ActorID:    "retry-actor",
		ShouldSign: true,
	}

	// Push to queue
	client.LogEvent(ctx, req)

	// Wait for attempts to reach 3 (3 retries within the worker)
	// The worker loop does: try 1, fail, try 2, fail, try 3, success.
	// So attempts should be 3.
	success := false
	for i := 0; i < 20; i++ {
		if attempts >= 3 {
			success = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !success {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	// Double check that it didn't do a 4th attempt
	time.Sleep(200 * time.Millisecond)
	if attempts > 3 {
		t.Errorf("expected exactly 3 attempts, got %d", attempts)
	}

	_ = client.Close(ctx)
}
