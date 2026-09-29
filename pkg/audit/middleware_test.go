package audit

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"log/slog"
	"sync"
	"testing"
)

// mockAuditor is a controllable Auditor for middleware tests.
// It records LogEvent calls and can be configured enabled/disabled.
type mockAuditor struct {
	mu             sync.Mutex
	enabled        bool
	logEventCalls  int
	receivedEvents []*AuditLogRequest
	receivedCtxs   []context.Context
	failOnLogEvent bool // if true, LogEvent fails the test when called
	t              *testing.T
}

func (m *mockAuditor) LogEvent(ctx context.Context, event *AuditLogRequest) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOnLogEvent {
		m.t.Error("LogEvent must not be called")
		return false
	}
	m.logEventCalls++
	m.receivedCtxs = append(m.receivedCtxs, ctx)
	if event != nil {
		cp := *event
		m.receivedEvents = append(m.receivedEvents, &cp)
	} else {
		m.receivedEvents = append(m.receivedEvents, nil)
	}
	return true
}

func (m *mockAuditor) SignEvent(ctx context.Context, event *AuditLogRequest) error {
	return nil
}

func (m *mockAuditor) SignMessageBytes(ctx context.Context, message []byte) (string, error) {
	return "", nil
}

func (m *mockAuditor) LogSignedEvent(ctx context.Context, event *AuditLogRequest) {}

func (m *mockAuditor) VerifyIntegrity(event *AuditLogRequest, publicKey crypto.PublicKey) (bool, error) {
	return false, nil
}

func (m *mockAuditor) IsEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func (m *mockAuditor) Close(ctx context.Context) error { return nil }

func (m *mockAuditor) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.logEventCalls
}

func (m *mockAuditor) events() []*AuditLogRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*AuditLogRequest, len(m.receivedEvents))
	copy(out, m.receivedEvents)
	return out
}

func sampleRequest() *AuditLogRequest {
	trace := "trace-123"
	target := "target-1"
	return &AuditLogRequest{
		TraceID:    &trace,
		Timestamp:  "2023-01-01T00:00:00Z",
		EventType:  "USER_MANAGEMENT",
		Action:     "CREATE",
		Status:     StatusSuccess,
		ActorType:  "USER",
		ActorID:    "actor-1",
		TargetType: "RESOURCE",
		TargetID:   &target,
		Message:    []byte(`{"k":"v"}`),
		Metadata:   map[string]interface{}{"source": "test"},
	}
}

func TestAuditMiddleware_LogAuditEvent(t *testing.T) {
	t.Run("skips when client is nil", func(t *testing.T) {
		m := &AuditMiddleware{client: nil}
		// Must not panic.
		m.LogAuditEvent(context.Background(), sampleRequest())
	})

	t.Run("skips when IsEnabled returns false", func(t *testing.T) {
		client := &mockAuditor{t: t, enabled: false, failOnLogEvent: true}
		m := &AuditMiddleware{client: client}
		m.LogAuditEvent(context.Background(), sampleRequest())
		if got := client.callCount(); got != 0 {
			t.Fatalf("LogEvent call count = %d, want 0", got)
		}
		if len(client.events()) != 0 {
			t.Fatalf("receivedEvents len = %d, want 0", len(client.events()))
		}
	})

	t.Run("calls LogEvent when enabled", func(t *testing.T) {
		client := &mockAuditor{enabled: true}
		m := &AuditMiddleware{client: client}
		req := sampleRequest()
		ctx := context.WithValue(context.Background(), struct{}{}, "marker")

		m.LogAuditEvent(ctx, req)

		if got := client.callCount(); got != 1 {
			t.Fatalf("LogEvent call count = %d, want 1", got)
		}
		events := client.events()
		if len(events) != 1 {
			t.Fatalf("receivedEvents len = %d, want 1", len(events))
		}
		got := events[0]
		if got == nil {
			t.Fatal("received event is nil")
		}
		if got.Action != req.Action || got.ActorID != req.ActorID || got.ActorType != req.ActorType {
			t.Errorf("event mismatch: got action=%q actor=%q/%q, want action=%q actor=%q/%q",
				got.Action, got.ActorType, got.ActorID, req.Action, req.ActorType, req.ActorID)
		}
		if got.Status != req.Status || got.EventType != req.EventType {
			t.Errorf("event classification mismatch: got type=%q status=%q, want type=%q status=%q",
				got.EventType, got.Status, req.EventType, req.Status)
		}
		if string(got.Message) != string(req.Message) {
			t.Errorf("message = %q, want %q", got.Message, req.Message)
		}
		client.mu.Lock()
		receivedCtx := client.receivedCtxs[0]
		client.mu.Unlock()
		if receivedCtx != ctx {
			t.Error("LogEvent was not passed the same context")
		}
	})

	t.Run("forwards nil request when enabled", func(t *testing.T) {
		client := &mockAuditor{enabled: true}
		m := &AuditMiddleware{client: client}
		m.LogAuditEvent(context.Background(), nil)
		if got := client.callCount(); got != 1 {
			t.Fatalf("LogEvent call count = %d, want 1", got)
		}
		events := client.events()
		if len(events) != 1 || events[0] != nil {
			t.Fatalf("expected a single nil event, got %#v", events)
		}
	})

	t.Run("does not call LogEvent after toggling to disabled", func(t *testing.T) {
		client := &mockAuditor{enabled: true}
		m := &AuditMiddleware{client: client}

		m.LogAuditEvent(context.Background(), sampleRequest())
		if got := client.callCount(); got != 1 {
			t.Fatalf("after enabled call: count = %d, want 1", got)
		}

		client.mu.Lock()
		client.enabled = false
		client.failOnLogEvent = true
		client.t = t
		client.mu.Unlock()

		m.LogAuditEvent(context.Background(), sampleRequest())
		if got := client.callCount(); got != 1 {
			t.Fatalf("after disable: count = %d, want 1 (no additional calls)", got)
		}
	})

	t.Run("multiple events when enabled", func(t *testing.T) {
		client := &mockAuditor{enabled: true}
		m := &AuditMiddleware{client: client}

		for i := 0; i < 3; i++ {
			req := sampleRequest()
			req.ActorID = fmt.Sprintf("actor-%d", i+1)
			m.LogAuditEvent(context.Background(), req)
		}
		events := client.events()
		if got := client.callCount(); got != 3 {
			t.Fatalf("LogEvent call count = %d, want 3", got)
		}
		if len(events) != 3 {
			t.Fatalf("receivedEvents len = %d, want 3", len(events))
		}
		for i, ev := range events {
			want := fmt.Sprintf("actor-%d", i+1)
			if ev.ActorID != want {
				t.Errorf("event[%d].ActorID = %q, want %q", i, ev.ActorID, want)
			}
		}
	})
}

func TestNewAuditMiddleware(t *testing.T) {
	ResetGlobalAuditMiddleware()
	t.Cleanup(ResetGlobalAuditMiddleware)

	t.Run("returns middleware with client", func(t *testing.T) {
		ResetGlobalAuditMiddleware()
		client := &mockAuditor{enabled: true}
		m := NewAuditMiddleware(client)
		if m == nil {
			t.Fatal("NewAuditMiddleware returned nil")
		}
		if m.Client() != client {
			t.Error("Client() did not return the provided auditor")
		}
	})

	t.Run("sets global instance on first call only", func(t *testing.T) {
		ResetGlobalAuditMiddleware()
		first := &mockAuditor{enabled: true}
		second := &mockAuditor{enabled: false, failOnLogEvent: true, t: t}

		m1 := NewAuditMiddleware(first)
		m2 := NewAuditMiddleware(second)

		if m1.Client() != first {
			t.Error("first middleware should hold first client")
		}
		if m2.Client() != second {
			t.Error("second middleware instance should hold second client")
		}

		global := GetGlobalAuditMiddleware()
		if global == nil {
			t.Fatal("global middleware is nil")
		}
		if global.Client() != first {
			t.Error("global middleware should remain the first-initialized instance")
		}

		// Global path should still invoke the first (enabled) client.
		LogAuditEvent(context.Background(), sampleRequest())
		if got := first.callCount(); got != 1 {
			t.Fatalf("first client LogEvent count = %d, want 1 via global", got)
		}
		if got := second.callCount(); got != 0 {
			t.Fatalf("second client LogEvent count = %d, want 0", got)
		}
	})

	t.Run("nil client middleware skips logging", func(t *testing.T) {
		ResetGlobalAuditMiddleware()
		m := NewAuditMiddleware(nil)
		m.LogAuditEvent(context.Background(), sampleRequest())
		LogAuditEvent(context.Background(), sampleRequest()) // via global; also nil client
	})
}

func TestInitializeGlobalAudit(t *testing.T) {
	ResetGlobalAuditMiddleware()
	t.Cleanup(ResetGlobalAuditMiddleware)

	t.Run("disabled client skips via global LogAuditEvent", func(t *testing.T) {
		ResetGlobalAuditMiddleware()
		client := &mockAuditor{enabled: false, failOnLogEvent: true, t: t}
		InitializeGlobalAudit(client)

		LogAuditEvent(context.Background(), sampleRequest())

		if got := client.callCount(); got != 0 {
			t.Fatalf("LogEvent call count = %d, want 0", got)
		}
		global := GetGlobalAuditMiddleware()
		if global == nil || global.Client() != client {
			t.Fatal("global middleware not initialized with disabled client")
		}
	})

	t.Run("enabled client receives events via global LogAuditEvent", func(t *testing.T) {
		ResetGlobalAuditMiddleware()
		client := &mockAuditor{enabled: true}
		InitializeGlobalAudit(client)

		req := sampleRequest()
		LogAuditEvent(context.Background(), req)

		if got := client.callCount(); got != 1 {
			t.Fatalf("LogEvent call count = %d, want 1", got)
		}
		events := client.events()
		if len(events) != 1 || events[0].ActorID != req.ActorID {
			t.Fatalf("unexpected events: %#v", events)
		}
	})

	t.Run("subsequent InitializeGlobalAudit is ignored", func(t *testing.T) {
		ResetGlobalAuditMiddleware()
		first := &mockAuditor{enabled: true}
		second := &mockAuditor{enabled: true}
		InitializeGlobalAudit(first)
		InitializeGlobalAudit(second)

		LogAuditEvent(context.Background(), sampleRequest())
		if first.callCount() != 1 {
			t.Fatalf("first client count = %d, want 1", first.callCount())
		}
		if second.callCount() != 0 {
			t.Fatalf("second client count = %d, want 0 (initialize ignored)", second.callCount())
		}
	})
}

func TestLogAuditEvent_GlobalUninitialized(t *testing.T) {
	ResetGlobalAuditMiddleware()
	t.Cleanup(ResetGlobalAuditMiddleware)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	// Must not panic; should warn that global middleware is missing.
	LogAuditEvent(context.Background(), sampleRequest())

	if !bytes.Contains(buf.Bytes(), []byte("Global AuditMiddleware is not initialized")) {
		t.Fatalf("expected warn log about uninitialized middleware, got: %s", buf.String())
	}
}

func TestGetGlobalAuditMiddleware(t *testing.T) {
	ResetGlobalAuditMiddleware()
	t.Cleanup(ResetGlobalAuditMiddleware)

	if GetGlobalAuditMiddleware() != nil {
		t.Fatal("expected nil global before init")
	}

	client := &mockAuditor{enabled: true}
	InitializeGlobalAudit(client)
	got := GetGlobalAuditMiddleware()
	if got == nil || got.Client() != client {
		t.Fatal("GetGlobalAuditMiddleware did not return initialized instance")
	}
}

func TestResetGlobalAuditMiddleware(t *testing.T) {
	ResetGlobalAuditMiddleware()
	t.Cleanup(ResetGlobalAuditMiddleware)

	InitializeGlobalAudit(&mockAuditor{enabled: true})
	if GetGlobalAuditMiddleware() == nil {
		t.Fatal("expected global after init")
	}

	ResetGlobalAuditMiddleware()
	if GetGlobalAuditMiddleware() != nil {
		t.Fatal("expected nil global after reset")
	}

	// After reset, a new InitializeGlobalAudit should take effect.
	client := &mockAuditor{enabled: true}
	InitializeGlobalAudit(client)
	LogAuditEvent(context.Background(), sampleRequest())
	if client.callCount() != 1 {
		t.Fatalf("after reset+reinit: count = %d, want 1", client.callCount())
	}
}
