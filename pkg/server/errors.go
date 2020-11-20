package server

import (
	"errors"
)

var (
	ErrNotFound = errors.New("error: not found")
	ErrInvalid  = errors.New("error: invalid")
)
