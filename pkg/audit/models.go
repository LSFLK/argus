package audit

// AuditLogRequest represents the request payload for creating an audit log
type AuditLogRequest struct {
	// Trace & Correlation
	TraceID *string `json:"traceId,omitempty"` // UUID string, nullable for standalone events

	// Temporal
	Timestamp string `json:"timestamp"` // ISO 8601 format, required

	// Event Classification
	EventType string `json:"eventType,omitempty"` // MANAGEMENT_EVENT, USER_MANAGEMENT
	Action    string `json:"action"`              // CREATE, READ, UPDATE, DELETE (Required by server)
	Status    string `json:"status"`              // SUCCESS, FAILURE

	// Actor Information
	ActorType string `json:"actorType"` // SERVICE, ADMIN, MEMBER, SYSTEM
	ActorID   string `json:"actorId"`   // email, uuid, or service-name

	// Target Information
	TargetType string  `json:"targetType"`         // SERVICE, RESOURCE
	TargetID   *string `json:"targetId,omitempty"` // resource_id or service_name

	// Payload & Metadata
	Message  []byte                 `json:"message"`            // Specific blob for NSW/NPQS
	Metadata map[string]interface{} `json:"metadata,omitempty"` // Consolidated metadata

	// Security & Non-Repudiation
	ShouldSign         bool   `json:"-"` // Internal flag to trigger signing
	Signature          string `json:"signature,omitempty"`
	SignatureAlgorithm string `json:"signatureAlgorithm,omitempty"`
	PublicKeyID        string `json:"publicKeyId,omitempty"`
}

// Audit log status constants
const (
	StatusSuccess = "SUCCESS"
	StatusFailure = "FAILURE"
)
