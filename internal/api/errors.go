package api

import (
	"encoding/json"
	"net/http"
	"strings"

	core "go.lumeweb.com/portal/core"
)

const (
	Namespace = "quota"

	// Error keys for quota operations
	ErrKeyPlanNotFound        core.ErrorType = "PLAN_NOT_FOUND"
	ErrKeyGrantNotFound       core.ErrorType = "GRANT_NOT_FOUND"
	ErrKeyInvalidPlanID       core.ErrorType = "INVALID_PLAN_ID"
	ErrKeyInvalidGrantID      core.ErrorType = "INVALID_GRANT_ID"
	ErrKeyPlanCreateFailed    core.ErrorType = "PLAN_CREATE_FAILED"
	ErrKeyPlanUpdateFailed    core.ErrorType = "PLAN_UPDATE_FAILED"
	ErrKeyPlanDeleteFailed    core.ErrorType = "PLAN_DELETE_FAILED"
	ErrKeyGrantCreateFailed   core.ErrorType = "GRANT_CREATE_FAILED"
	ErrKeyUpdateFailed        core.ErrorType = "GRANT_UPDATE_FAILED"
	ErrKeyGrantFetchFailed    core.ErrorType = "GRANT_FETCH_FAILED"
	ErrKeyGrantDeleteFailed   core.ErrorType = "GRANT_DELETE_FAILED"
	ErrKeyInvalidRequest      core.ErrorType = "INVALID_REQUEST"
	ErrKeyInvalidRequestParameters core.ErrorType = "INVALID_REQUEST_PARAMETERS"
	ErrKeyUnauthorized        core.ErrorType = "UNAUTHORIZED"
	ErrKeyForbidden           core.ErrorType = "FORBIDDEN"
	ErrKeyRetentionDaysInvalid core.ErrorType = "RETENTION_DAYS_MUST_BE_POSITIVE"
	ErrKeyReconcilationFailed core.ErrorType = "RECONCILIATION_FAILED"
	ErrKeyCleanupFailed       core.ErrorType = "CLEANUP_FAILED"
	ErrKeyConfigUpdateFailed  core.ErrorType = "CONFIG_UPDATE_FAILED"
	ErrKeyConfigFetchFailed   core.ErrorType = "CONFIG_FETCH_FAILED"
)

// ErrorDetails represents the structured error response format
type ErrorDetails struct {
	Reason  string `json:"reason"`
	Details string `json:"details,omitempty"`
}

// ErrorWrapper wraps ErrorDetails for custom JSON marshaling
type ErrorWrapper struct {
	Error ErrorDetails `json:"error"`
}

// QuotaError represents a quota-specific error that can be marshaled to JSON
type QuotaError struct {
	coreErr *core.Error
}

// MarshalJSON implements json.Marshaler interface
func (e *QuotaError) MarshalJSON() ([]byte, error) {
	if e == nil || e.coreErr == nil {
		return json.Marshal(ErrorWrapper{Error: ErrorDetails{Reason: "Unknown"}})
	}
	reason := string(e.coreErr.Key)

	if strings.HasPrefix(reason, "ErrKey") {
		reason = reason[6:]
	}

	if strings.HasPrefix(reason, "Err") {
		reason = reason[3:]
	}

	details := ErrorDetails{
		Reason:  reason,
		Details: e.coreErr.Message,
	}

	wrapper := ErrorWrapper{Error: details}
	return json.Marshal(wrapper)
}

func (e *QuotaError) Error() string {
	return e.coreErr.Error()
}

func (e *QuotaError) HttpStatus() int {
	return e.coreErr.HttpStatus()
}

func (e *QuotaError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.coreErr
}

func init() {
	core.MustRegisterNamespace(Namespace)
	core.MustRegisterDefaultErrorMessages(Namespace, map[core.ErrorType]core.ErrorDefinition{
		ErrKeyPlanNotFound:        {Key: ErrKeyPlanNotFound, Message: "Quota plan not found"},
		ErrKeyGrantNotFound:       {Key: ErrKeyGrantNotFound, Message: "Allowance grant not found"},
		ErrKeyInvalidPlanID:       {Key: ErrKeyInvalidPlanID, Message: "Invalid plan ID format"},
		ErrKeyInvalidGrantID:      {Key: ErrKeyInvalidGrantID, Message: "Invalid grant ID format"},
		ErrKeyPlanCreateFailed:    {Key: ErrKeyPlanCreateFailed, Message: "Failed to create quota plan"},
		ErrKeyPlanUpdateFailed:    {Key: ErrKeyPlanUpdateFailed, Message: "Failed to update quota plan"},
		ErrKeyPlanDeleteFailed:    {Key: ErrKeyPlanDeleteFailed, Message: "Failed to delete quota plan"},
		ErrKeyGrantCreateFailed:   {Key: ErrKeyGrantCreateFailed, Message: "Failed to create grant"},
		ErrKeyUpdateFailed:        {Key: ErrKeyUpdateFailed, Message: "Failed to update grant"},
		ErrKeyGrantFetchFailed:    {Key: ErrKeyGrantFetchFailed, Message: "Failed to fetch grants"},
		ErrKeyGrantDeleteFailed:   {Key: ErrKeyGrantDeleteFailed, Message: "Failed to delete grant"},
		ErrKeyInvalidRequest:      {Key: ErrKeyInvalidRequest, Message: "Invalid request parameter"},
		ErrKeyInvalidRequestParameters: {Key: ErrKeyInvalidRequestParameters, Message: "Invalid request parameter"},
		ErrKeyUnauthorized:        {Key: ErrKeyUnauthorized, Message: "Unauthorized access"},
		ErrKeyForbidden:           {Key: ErrKeyForbidden, Message: "Access forbidden"},
		ErrKeyRetentionDaysInvalid: {Key: ErrKeyRetentionDaysInvalid, Message: "Retention days must be positive"},
		ErrKeyReconcilationFailed: {Key: ErrKeyReconcilationFailed, Message: "Failed to reconcile quota usage"},
		ErrKeyCleanupFailed:       {Key: ErrKeyCleanupFailed, Message: "Failed to cleanup old records"},
		ErrKeyConfigUpdateFailed:  {Key: ErrKeyConfigUpdateFailed, Message: "Failed to update system config"},
		ErrKeyConfigFetchFailed:   {Key: ErrKeyConfigFetchFailed, Message: "Failed to fetch system config"},
	})

	core.MustRegisterErrorCodes(Namespace, map[core.ErrorType]int{
		ErrKeyPlanNotFound:        http.StatusNotFound,
		ErrKeyGrantNotFound:       http.StatusNotFound,
		ErrKeyInvalidPlanID:       http.StatusBadRequest,
		ErrKeyInvalidGrantID:      http.StatusBadRequest,
		ErrKeyPlanCreateFailed:    http.StatusInternalServerError,
		ErrKeyPlanUpdateFailed:    http.StatusInternalServerError,
		ErrKeyPlanDeleteFailed:    http.StatusInternalServerError,
		ErrKeyGrantCreateFailed:   http.StatusInternalServerError,
		ErrKeyUpdateFailed:        http.StatusInternalServerError,
		ErrKeyGrantFetchFailed:    http.StatusInternalServerError,
		ErrKeyGrantDeleteFailed:   http.StatusInternalServerError,
		ErrKeyInvalidRequest:      http.StatusBadRequest,
		ErrKeyInvalidRequestParameters: http.StatusBadRequest,
		ErrKeyUnauthorized:        http.StatusUnauthorized,
		ErrKeyForbidden:           http.StatusForbidden,
		ErrKeyRetentionDaysInvalid: http.StatusBadRequest,
		ErrKeyReconcilationFailed: http.StatusInternalServerError,
		ErrKeyCleanupFailed:       http.StatusInternalServerError,
		ErrKeyConfigUpdateFailed:  http.StatusInternalServerError,
		ErrKeyConfigFetchFailed:   http.StatusInternalServerError,
	})
}

func NewError(key core.ErrorType, err error, args ...any) *QuotaError {
	return &QuotaError{core.NewError(Namespace, key, err, args...)}
}
