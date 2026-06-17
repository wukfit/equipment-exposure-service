package domain

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEquipmentNotFound = errors.New("equipment not found")
	ErrExposureNotFound  = errors.New("exposure not found")
	ErrInvalidDuration   = errors.New("invalid duration")
	ErrInvalidMagnitude  = errors.New("invalid magnitude")
	ErrInvalidName       = errors.New("invalid name")

	// ErrDataConsistency marks a referential-integrity failure: a stored
	// aggregate references another that can no longer be resolved. It is a
	// server-side fault (→ 500), not a client "not found".
	ErrDataConsistency = errors.New("data consistency error")
)
