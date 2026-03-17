package audit

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
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
		RequestMetadata:    json.RawMessage(`{"key":"value"}`),
		Signature:          "should-be-stripped",
		SignatureAlgorithm: "RS256",
		PublicKeyID:        "key-1",
	}

	b, err := CanonicalizeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("failed to decode canonical json: %v", err)
	}

	if _, ok := parsed["signature"]; ok {
		t.Errorf("expected signature to be stripped")
	}
	if _, ok := parsed["signatureAlgorithm"]; ok {
		t.Errorf("expected signatureAlgorithm to be stripped")
	}
	if _, ok := parsed["publicKeyId"]; ok {
		t.Errorf("expected publicKeyId to be stripped")
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
	err = client.SignEvent(ctx, req, "test-key-1")
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
