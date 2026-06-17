package domain

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEquipmentNotFound = errors.New("equipment not found")
	ErrExposureNotFound  = errors.New("exposure not found")
	ErrInvalidDuration   = errors.New("invalid duration")
	ErrInvalidMagnitude  = errors.New("invalid magnitude")
	ErrInvalidName       = errors.New("invalid name")
)
