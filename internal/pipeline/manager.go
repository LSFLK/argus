package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/LSFLK/argus/internal/pipeline/sinks"
)

const (
	DefaultAsyncQueueSize = 1000
	DefaultWorkerCount    = 5
)

// Config defines the configuration for the Pipeline Manager.
type Config struct {
	AsyncQueueSize int
	WorkerCount    int
}

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
func NewManager(cfg *Config, sinks ...sinks.Sink) *Manager {
	if cfg == nil {
		cfg = &Config{
			AsyncQueueSize: DefaultAsyncQueueSize,
			WorkerCount:    DefaultWorkerCount,
		}
	}
	if cfg.AsyncQueueSize <= 0 {
		cfg.AsyncQueueSize = DefaultAsyncQueueSize
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = DefaultWorkerCount
	}

	m := &Manager{
		sinks:      sinks,
		asyncQueue: make(chan asyncTask, cfg.AsyncQueueSize),
		quit:       make(chan struct{}),
	}

	// Start a pool of background workers for fire-and-forget sinks
	for i := 0; i < cfg.WorkerCount; i++ {
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
	// Implement an internal watchdog to prevent goroutine accumulation
	// if the parent context doesn't have a deadline and a sink hangs.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

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
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

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
	// Detach context to ensure background workers don't fail when HTTP request completes
	detachedCtx := context.WithoutCancel(ctx)
	select {
	case m.asyncQueue <- asyncTask{ctx: detachedCtx, log: log}:
	default:
		slog.Warn("Pipeline async queue full, dropping log", "id", log.ID)
	}
}

// DispatchBatchAsync submits a batch of logs for background fan-out.
func (m *Manager) DispatchBatchAsync(ctx context.Context, logs []models.AuditLog) {
	detachedCtx := context.WithoutCancel(ctx)
	select {
	case m.asyncQueue <- asyncTask{ctx: detachedCtx, logs: logs}:
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
