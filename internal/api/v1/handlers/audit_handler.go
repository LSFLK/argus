package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/LSFLK/argus/internal/api/v1/services"
	"github.com/LSFLK/argus/internal/api/v1/utils"
	"github.com/google/uuid"
)

// AuditHandler handles HTTP requests for audit logs
type AuditHandler struct {
	service *services.AuditService
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(service *services.AuditService) *AuditHandler {
	return &AuditHandler{service: service}
}

// CreateAuditLog handles POST /api/audit-logs
func (h *AuditHandler) CreateAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateAuditLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validation for signed events
	if req.Signature != "" || req.PublicKeyID != "" || req.SignatureAlgorithm != "" {
		if req.Signature == "" || req.PublicKeyID == "" || req.SignatureAlgorithm == "" {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid signed event: signature, publicKeyId, and signatureAlgorithm must all be provided if any are present", nil)
			return
		}
	}

	// Validation is handled by the service layer (auditLog.Validate())
	auditLog, err := h.service.CreateAuditLog(r.Context(), &req)
	if err != nil {
		// Return 400 Bad Request for validation errors, 500 for other errors
		if services.IsValidationError(err) {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload", err)
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create audit log", err)
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, auditLog)
}

// CreateAuditLogBatch handles POST /api/audit-logs/bulk
func (h *AuditHandler) CreateAuditLogBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateAuditLogBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Basic validation for signatures in batch
	for _, logReq := range req {
		if logReq.Signature != "" || logReq.PublicKeyID != "" || logReq.SignatureAlgorithm != "" {
			if logReq.Signature == "" || logReq.PublicKeyID == "" || logReq.SignatureAlgorithm == "" {
				utils.RespondWithError(w, http.StatusBadRequest, "Invalid signed event in batch: signature, publicKeyId, and signatureAlgorithm must all be provided", nil)
				return
			}
		}
	}

	logs, err := h.service.CreateAuditLogBatch(r.Context(), req)
	if err != nil {
		if services.IsValidationError(err) {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload in batch", err)
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create audit log batch", err)
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, logs)
}

// GetAuditLogs handles GET /api/audit-logs
func (h *AuditHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	traceID := r.URL.Query().Get("traceId")
	eventType := r.URL.Query().Get("eventType")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	includeMessageStr := r.URL.Query().Get("includeMessage")

	limit := 100            // default
	offset := 0             // default
	includeMessage := false // default: omit large messages in list view

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}
	if includeMessageStr == "true" {
		includeMessage = true
	}

	// Validate traceId format if provided
	var traceIDPtr *string
	if traceID != "" {
		// Validate UUID format - return 400 for invalid format instead of 500
		if _, err := uuid.Parse(traceID); err != nil {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid traceId format: expected UUID", err)
			return
		}
		traceIDPtr = &traceID
	}

	var eventTypePtr *string
	if eventType != "" {
		eventTypePtr = &eventType
	}

	logs, total, err := h.service.GetAuditLogs(r.Context(), traceIDPtr, eventTypePtr, limit, offset, includeMessage)
	if err != nil {
		// Check if it's a validation error (e.g. invalid traceId format from service layer)
		if services.IsValidationError(err) {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid query parameters", err)
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve audit logs", err)
		return
	}

	response := models.GetAuditLogsResponse{
		Logs:   make([]models.AuditLogResponse, len(logs)),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	for i, log := range logs {
		response.Logs[i] = models.ToAuditLogResponse(log)
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// GetAuditLogByID handles GET /api/audit-logs/:id
func (h *AuditHandler) GetAuditLogByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// In a real router like chi/gorilla, the ID would be in the URL params.
	// Since we are using a basic handler, we might expect it in the query or path.
	// The current main.go doesn't show a sophisticated router, so we'll look for "id" query param for now,
	// or assume the caller handles routing to this method with the ID available.
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing audit log ID", nil)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid audit log ID format", err)
		return
	}

	log, err := h.service.GetAuditLogByID(r.Context(), id)
	if err != nil {
		if services.IsNotFoundError(err) {
			utils.RespondWithError(w, http.StatusNotFound, "Audit log not found", err)
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve audit log", err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, models.ToAuditLogResponse(*log))
}
