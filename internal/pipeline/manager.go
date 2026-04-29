package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/LSFLK/argus/internal/pipeline/sinks"
)

// Manager coordinates the fan-out of audit logs to multiple registered sinks.
// It supports both synchronous and asynchronous dispatching.
type Manager struct {
	sinks []sinks.Sink

	// Async worker pool
	asyncQueue chan asyncTask
	quit       chan struct{}
}

type asyncTask struct {
	ctx  context.Context
	log  *models.AuditLog
	logs []models.AuditLog
}

// NewManager creates a new pipeline manager and starts background workers for async dispatch.
func NewManager(sinks ...sinks.Sink) *Manager {
	m := &Manager{
		sinks:      sinks,
		asyncQueue: make(chan asyncTask, 1000),
		quit:       make(chan struct{}),
	}

	// Start a pool of background workers for fire-and-forget sinks
	for i := 0; i < 5; i++ {
		go m.worker()
	}

	return m
}

func (m *Manager) worker() {
	for {
		select {
		case task := <-m.asyncQueue:
			if task.log != nil {
				_ = m.Dispatch(task.ctx, task.log)
			} else if task.logs != nil {
				_ = m.DispatchBatch(task.ctx, task.logs)
			}
		case <-m.quit:
			return
		}
	}
}

// Dispatch fans out an audit log to all registered sinks concurrently.
// It returns a slice of errors encountered during the dispatch process.
// If one sink fails, it does not prevent others from attempting to write.
func (m *Manager) Dispatch(ctx context.Context, log *models.AuditLog) []error {
	var (
		wg    sync.WaitGroup
		errs  []error
		errMu sync.Mutex
	)

	wg.Add(len(m.sinks))
	for _, sink := range m.sinks {
		go func(s sinks.Sink) {
			defer wg.Done()
			if err := s.Write(ctx, log); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("sink %s failed: %w", s.Name(), err))
				errMu.Unlock()
			}
		}(sink)
	}

	// Use a channel to wait for all sinks to finish or for the context to time out.
	// This prevents goroutine leaks if a sink hangs indefinitely.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return errs
	case <-ctx.Done():
		return append(errs, ctx.Err())
	}
}

// DispatchBatch fans out a batch of audit logs to all registered sinks concurrently.
func (m *Manager) DispatchBatch(ctx context.Context, logs []models.AuditLog) []error {
	var (
		wg    sync.WaitGroup
		errs  []error
		errMu sync.Mutex
	)

	wg.Add(len(m.sinks))
	for _, sink := range m.sinks {
		go func(s sinks.Sink) {
			defer wg.Done()
			if err := s.WriteBatch(ctx, logs); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("sink %s failed: %w", s.Name(), err))
				errMu.Unlock()
			}
		}(sink)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return errs
	case <-ctx.Done():
		return append(errs, ctx.Err())
	}
}

// DispatchAsync submits a log for background fan-out.
func (m *Manager) DispatchAsync(ctx context.Context, log *models.AuditLog) {
	select {
	case m.asyncQueue <- asyncTask{ctx: ctx, log: log}:
	default:
		slog.Warn("Pipeline async queue full, dropping log", "id", log.ID)
	}
}

// DispatchBatchAsync submits a batch of logs for background fan-out.
func (m *Manager) DispatchBatchAsync(ctx context.Context, logs []models.AuditLog) {
	select {
	case m.asyncQueue <- asyncTask{ctx: ctx, logs: logs}:
	default:
		slog.Warn("Pipeline async queue full, dropping batch", "count", len(logs))
	}
}

// Sinks returns the list of registered sinks.
func (m *Manager) Sinks() []sinks.Sink {
	return m.sinks
}

// Close gracefully shuts down all registered sinks and the worker pool.
func (m *Manager) Close() []error {
	close(m.quit)

	var errs []error
	for _, sink := range m.sinks {
		if err := sink.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
