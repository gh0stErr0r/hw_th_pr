package core

import "errors"

var (
	ErrTooShort           = errors.New("container too short")
	ErrBadMagic           = errors.New("bad container magic")
	ErrUnsupportedVersion = errors.New("unsupported container version")
	ErrBadParams          = errors.New("invalid Argon2id parameters")
	ErrAuthFailed         = errors.New("authentication failed")
	ErrInputTooLarge      = errors.New("input too large")
)
