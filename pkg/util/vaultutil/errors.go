package vaultutil

import "errors"

var (
	ErrNoSuchEngineMount = errors.New("vaultutil: engine mount does not exist")
)
