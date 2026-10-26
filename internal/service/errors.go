package service

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid      = errors.New("invalid request")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("access denied")
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
)

func invalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalid, message) }
