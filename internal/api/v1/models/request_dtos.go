package models

// CreateAuditLogRequest represents the request payload for creating a generalized audit log
// This matches the final SQL schema with unified actor/target approach
type CreateAuditLogRequest struct {
	// Trace & Correlation
	TraceID *string `json:"traceId,omitempty"` // UUID string, nullable for standalone events

	// Temporal
	Timestamp string `json:"timestamp" validate:"required"` // ISO 8601 format, required

	// Event Classification
	EventType string `json:"eventType,omitempty"`        // MANAGEMENT_EVENT, USER_MANAGEMENT
	Action    string `json:"action" validate:"required"` // CREATE, READ, UPDATE, DELETE
	Status    string `json:"status" validate:"required"` // SUCCESS, FAILURE

	// Actor Information (unified approach)
	ActorType string `json:"actorType" validate:"required"` // SERVICE, ADMIN, MEMBER, SYSTEM
	ActorID   string `json:"actorId" validate:"required"`   // email, uuid, or service-name (required)

	// Target Information (unified approach)
	TargetType string  `json:"targetType" validate:"required"` // SERVICE, RESOURCE
	TargetID   *string `json:"targetId,omitempty"`             // resource_id or service_name

	// Metadata
	Message  []byte                 `json:"message,omitempty"`  // Raw message or payload for signing
	Metadata map[string]interface{} `json:"metadata,omitempty"` // Consolidated metadata

	// Security & Non-Repudiation
	Signature          string `json:"signature,omitempty"`
	SignatureAlgorithm string `json:"signatureAlgorithm,omitempty"`
	PublicKeyID        string `json:"publicKeyId,omitempty"`
}

// CreateAuditLogBatchRequest represents a batch of audit log creation requests
type CreateAuditLogBatchRequest []CreateAuditLogRequest
