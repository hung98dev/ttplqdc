package auth

import (
	"errors"
	"net/http"
	"strings"
)

// APIError is one client-visible failure shaped for the HTTPS error body
// of auth.md § HTTPS Endpoints / errors.md § Error Shape.
type APIError struct {
	Status         int
	Code           string
	Retryability   string
	RetryAfterMs   int64
	QueuePosition  int32
	SafeMessageKey string
}

func (e *APIError) Error() string { return e.Code }

// apiError builds one API error with the retryability/safe-key derived
// from the code (retryability classes of errors.md § Retryability).
func apiError(status int, code string) *APIError {
	e := &APIError{Status: status, Code: code, SafeMessageKey: "error." + safeKey(code)}
	switch code {
	case "CLIENT_UPDATE_REQUIRED", "CONTENT_INCOMPATIBLE":
		e.Retryability = "UPDATE_REQUIRED"
	case "RATE_LIMITED", "SERVER_OVERLOADED", "SERVER_DRAINING":
		e.Retryability = "BACKOFF"
	case "TEMPORARY_DEPENDENCY_FAILURE":
		e.Retryability = "RETRY_SAME_OPERATION"
	case "AUTH_INVALID", "AUTH_EXPIRED", "RESUME_EXPIRED":
		e.Retryability = "REAUTHENTICATE"
	case "PROTOCOL_VIOLATION", "SESSION_EPOCH_STALE", "SESSION_REPLACED":
		e.Retryability = "RECONNECT"
	default:
		e.Retryability = "NEVER"
	}
	return e
}

func safeKey(code string) string {
	return strings.ToLower(code)
}

var (
	ErrBadRequest = apiError(http.StatusBadRequest, "INVALID_STATE")
)

// AsAPIError normalizes any error to *APIError; internals become
// TEMPORARY_DEPENDENCY_FAILURE (errors.md: never expose internals).
func AsAPIError(err error) *APIError {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae
	}
	return apiError(http.StatusServiceUnavailable, "TEMPORARY_DEPENDENCY_FAILURE")
}

// ---------------------------------------------------------------------------
// Validation errors (auth.md § Password Provider + § HTTPS Endpoints)

var (
	ErrUsernameInvalid = apiError(http.StatusBadRequest, "USERNAME_INVALID")
	ErrEmailInvalid    = apiError(http.StatusBadRequest, "EMAIL_INVALID")
	ErrPasswordInvalid = apiError(http.StatusBadRequest, "PASSWORD_INVALID")

	ErrAuthInvalid  = apiError(http.StatusUnauthorized, "AUTH_INVALID")
	ErrAuthExpired  = apiError(http.StatusUnauthorized, "AUTH_EXPIRED")
	ErrAuthRequired = apiError(http.StatusUnauthorized, "AUTH_REQUIRED")

	ErrBanned               = apiError(http.StatusForbidden, "ACCOUNT_BANNED")
	ErrSuspended            = apiError(http.StatusForbidden, "ACCOUNT_SUSPENDED")
	ErrCredentialChangeLock = apiError(http.StatusForbidden, "CREDENTIAL_CHANGE_LOCKED")

	ErrUsernameTaken         = apiError(http.StatusConflict, "USERNAME_TAKEN")
	ErrEmailTaken            = apiError(http.StatusConflict, "EMAIL_TAKEN")
	ErrProviderAlreadyLinked = apiError(http.StatusConflict, "PROVIDER_ALREADY_LINKED")
	ErrLastLoginMethod       = apiError(http.StatusConflict, "LAST_LOGIN_METHOD")
	ErrInvalidState          = apiError(http.StatusConflict, "INVALID_STATE")

	ErrClientUpdate    = apiError(http.StatusUpgradeRequired, "CLIENT_UPDATE_REQUIRED")
	ErrContentIncompat = apiError(http.StatusUpgradeRequired, "CONTENT_INCOMPATIBLE")

	ErrTemporaryDependency = apiError(http.StatusServiceUnavailable, "TEMPORARY_DEPENDENCY_FAILURE")
)

// ErrRateLimited builds a 429 with retry_after.
func ErrRateLimited(retryAfterMs int64) *APIError {
	e := apiError(http.StatusTooManyRequests, "RATE_LIMITED")
	e.RetryAfterMs = retryAfterMs
	return e
}

// ErrServerOverloaded builds the 503 queue payload.
func ErrServerOverloaded(queuePosition int32, retryAfterMs int64) *APIError {
	e := apiError(http.StatusServiceUnavailable, "SERVER_OVERLOADED")
	e.QueuePosition = queuePosition
	e.RetryAfterMs = retryAfterMs
	return e
}
