package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	v1database "github.com/LSFLK/argus/internal/api/v1/database"
	v1handlers "github.com/LSFLK/argus/internal/api/v1/handlers"
	v1models "github.com/LSFLK/argus/internal/api/v1/models"
	v1services "github.com/LSFLK/argus/internal/api/v1/services"
	"github.com/LSFLK/argus/internal/config"
	"github.com/LSFLK/argus/internal/database"
	"github.com/LSFLK/argus/internal/middleware"
	"github.com/LSFLK/argus/internal/pipeline"
	"github.com/LSFLK/argus/internal/pipeline/sinks"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Build information - set during build
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Parse command line flags
	var (
		env  = flag.String("env", config.GetEnvOrDefault("ENVIRONMENT", "production"), "Environment (development, production)")
		port = flag.String("port", config.GetEnvOrDefault("PORT", "3001"), "Port to listen on")
	)
	flag.Parse()

	// Server configuration
	serverPort := *port

	// Load enum configuration from YAML file
	configPath := config.GetEnvOrDefault("AUDIT_ENUMS_CONFIG", "configs/enums.yaml")
	enums, err := config.LoadEnums(configPath)
	if err != nil {
		slog.Warn("Failed to load enum configuration, using defaults", "error", err, "path", configPath)
		enums = config.GetDefaultEnums()
	}
	slog.Info("Loaded enum configuration", "path", configPath,
		"eventTypes", len(enums.EventTypes),
		"eventActions", len(enums.EventActions),
		"actorTypes", len(enums.ActorTypes),
		"targetTypes", len(enums.TargetTypes))

	// Initialize enum configuration in models package
	// Pass the AuditEnums instance to leverage O(1) validation methods
	v1models.SetEnumConfig(enums)

	// Initialize database connection
	dbConfig := database.NewDatabaseConfig()
	if dbConfig.Type == database.DatabaseTypeSQLite {
		slog.Info("Connecting to database",
			"type", "SQLite",
			"database_path", dbConfig.DatabasePath)
	} else {
		slog.Info("Connecting to database",
			"type", "PostgreSQL",
			"host", dbConfig.Host,
			"database", dbConfig.Database)
	}

	// Initialize GORM connection
	gormDB, err := database.ConnectGormDB(dbConfig)
	if err != nil {
		slog.Error("Failed to connect to database via GORM", "error", err)
		os.Exit(1)
	}

	// Setup routes
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Simple health check - just return healthy if service is running
		// Database connectivity is checked during startup, not in health check
		w.WriteHeader(http.StatusOK)
		response := map[string]string{
			"service": "argus",
			"status":  "healthy",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode health response", "error", err)
		}
	})

	// Version endpoint
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]string{
			"version":   Version,
			"buildTime": BuildTime,
			"gitCommit": GitCommit,
			"service":   "argus",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode version response", "error", err)
		}
	})

	// Initialize security: Public Key Registry
	keyRegistry := v1services.NewPublicKeyRegistry()
	// Optionally load keys from config/environment here if needed

	// Initialize Sinks (Writers)
	postgresSink := sinks.NewPostgresSink(gormDB)
	consoleSink := sinks.NewConsoleSink()

	activeSinks := []sinks.Sink{postgresSink, consoleSink}

	// Initialize S3 Compliance Sink if configured
	s3Bucket := config.GetEnvOrDefault("S3_COMPLIANCE_BUCKET", "")
	if s3Bucket != "" {
		s3Region := config.GetEnvOrDefault("S3_REGION", "us-east-1")
		s3Prefix := config.GetEnvOrDefault("S3_PREFIX", "audit-logs")
		s3Endpoint := config.GetEnvOrDefault("S3_ENDPOINT", "")
		s3UsePathStyle := config.GetEnvOrDefault("S3_USE_PATH_STYLE", "") == "true"
		s3ObjectLockMode := config.GetEnvOrDefault("S3_OBJECT_LOCK_MODE", "COMPLIANCE")

		retentionDays := 2555 // 7 years default
		if daysStr := os.Getenv("S3_RETENTION_DAYS"); daysStr != "" {
			if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 {
				retentionDays = parsedDays
			}
		}

		s3Cfg := sinks.S3SinkConfig{
			Bucket:         s3Bucket,
			Region:         s3Region,
			Prefix:         s3Prefix,
			Endpoint:       s3Endpoint,
			UsePathStyle:   s3UsePathStyle,
			ObjectLockMode: s3ObjectLockMode,
			RetentionDays:  retentionDays,
		}

		slog.Info("Initializing S3 Compliance Sink with Object Lock",
			"bucket", s3Cfg.Bucket,
			"region", s3Cfg.Region,
			"prefix", s3Cfg.Prefix,
			"endpoint", s3Cfg.Endpoint,
			"objectLockMode", s3Cfg.ObjectLockMode,
			"retentionDays", s3Cfg.RetentionDays)

		s3Sink, err := sinks.NewS3Sink(context.Background(), s3Cfg, nil)
		if err != nil {
			slog.Error("Failed to initialize S3 Compliance Sink", "error", err)
			os.Exit(1)
		}
		activeSinks = append(activeSinks, s3Sink)
	} else {
		slog.Info("S3 Compliance Sink not configured (set S3_COMPLIANCE_BUCKET to enable)")
	}

	// Initialize Readers (Query)
	gormReader := v1database.NewGormReader(gormDB)

	// Initialize Sink Manager (Router)
	// This enables Argus to fan out logs to multiple destinations concurrently.
	pipelineManager := pipeline.NewManager(&pipeline.Config{
		AsyncQueueSize: 1000,
		WorkerCount:    5,
	}, activeSinks...)

	// Initialize v1 API
	// The service layer now depends on the Manager for writes and GormReader for reads.
	v1AuditService := v1services.NewAuditService(pipelineManager, gormReader, keyRegistry)
	v1AuditHandler := v1handlers.NewAuditHandler(v1AuditService)

	// API endpoint for generalized audit logs (V1)
	mux.HandleFunc("/api/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			v1AuditHandler.CreateAuditLog(w, r)
		case http.MethodGet:
			v1AuditHandler.GetAuditLogs(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/audit-logs/bulk", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			v1AuditHandler.CreateAuditLogBatch(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/audit-summary", v1AuditHandler.GetAuditSummary)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Start server
	slog.Info("Argus starting",
		"environment", *env,
		"port", serverPort,
		"version", Version,
		"buildTime", BuildTime,
		"gitCommit", GitCommit)
	slog.Info("Database configuration",
		"database_path", dbConfig.DatabasePath)

	// Setup Middleware Chain
	// Order (outer to inner): Metrics -> CORS -> Auth -> mux
	handler := middleware.MetricsMiddleware(mux)
	handler = middleware.NewCORSMiddleware()(handler)
	handler = middleware.AuthMiddleware(handler)

	server := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("Starting HTTP server", "address", server.Addr)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down Argus...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	// Close the pipeline manager to flush any pending logs in sinks
	if errs := pipelineManager.Close(); len(errs) > 0 {
		for _, err := range errs {
			slog.Error("Failed to close sink during shutdown", "error", err)
		}
	}

	slog.Info("Argus exited")
}
