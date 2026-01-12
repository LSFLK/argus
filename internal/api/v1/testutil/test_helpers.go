package testutil

import (
	v1models "github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/LSFLK/argus/internal/config"
)

// SetupTestEnums configures enum validation for tests
func SetupTestEnums() *config.AuditEnums {
	enums := &config.AuditEnums{
		EventTypes:   []string{"MANAGEMENT_EVENT", "USER_MANAGEMENT", "DATA_FETCH"},
		EventActions: []string{"CREATE", "READ", "UPDATE", "DELETE"},
		ActorTypes:   []string{"SERVICE", "ADMIN", "MEMBER", "SYSTEM"},
		TargetTypes:  []string{"SERVICE", "RESOURCE"},
	}
	enums.InitializeMaps()
	v1models.SetEnumConfig(enums)
	return enums
}

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}
