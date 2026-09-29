package audit

import (
	"context"
	"crypto"
	"testing"
)

// disabledAuditor is an Auditor whose IsEnabled returns false.
// LogEvent fails the test if called, asserting the middleware skips it.
type disabledAuditor struct {
	t *testing.T
}

func (d *disabledAuditor) LogEvent(ctx context.Context, event *AuditLogRequest) bool {
	d.t.Error("LogEvent must not be called when IsEnabled returns false")
	return false
}

func (d *disabledAuditor) SignEvent(ctx context.Context, event *AuditLogRequest) error {
	return nil
}

func (d *disabledAuditor) SignMessageBytes(ctx context.Context, message []byte) (string, error) {
	return "", nil
}

func (d *disabledAuditor) LogSignedEvent(ctx context.Context, event *AuditLogRequest) {}

func (d *disabledAuditor) VerifyIntegrity(event *AuditLogRequest, publicKey crypto.PublicKey) (bool, error) {
	return false, nil
}

func (d *disabledAuditor) IsEnabled() bool { return false }

func (d *disabledAuditor) Close(ctx context.Context) error { return nil }

func TestLogAuditEvent_SkipsWhenDisabled(t *testing.T) {
	m := &AuditMiddleware{client: &disabledAuditor{t: t}}
	m.LogAuditEvent(context.Background(), &AuditLogRequest{
		Action:    "TEST",
		ActorID:   "actor-1",
		ActorType: "USER",
	})
}
