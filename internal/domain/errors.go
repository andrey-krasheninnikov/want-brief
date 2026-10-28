package domain

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalid        = errors.New("invalid")
	ErrConflict       = errors.New("conflict")
	ErrForbidden      = errors.New("forbidden")
	ErrSourceReadOnly = errors.New("synced item cannot be deleted")
	ErrNoToken        = errors.New("source token is empty")
	ErrUnsupported    = errors.New("unsupported source kind")
)
