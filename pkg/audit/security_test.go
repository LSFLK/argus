package audit

import (
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
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	payload := []byte(`{"event":"test"}`)
	sigBase64, alg, _ := SignPayload(payload, privateKey)

	privateKey2, _ := rsa.GenerateKey(rand.Reader, 2048)
	err := VerifyPayload(payload, sigBase64, alg, &privateKey2.PublicKey)
	if err == nil {
		t.Errorf("expected verification to fail with wrong key")
	}
}

func TestClient_InterfaceImplementation(t *testing.T) {
	// This test ensures Client implements Auditor interface
	var _ Auditor = (*Client)(nil)
}
