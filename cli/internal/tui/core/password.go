package core

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"unicode/utf8"
)

func ValidateNew(pw []byte) error {
	switch n := utf8.RuneCount(pw); {
	case n < 8:
		return errors.New("Use at least 8 characters")
	case n > 128:
		return errors.New("Use at most 128 characters")
	case len(bytes.TrimSpace(pw)) != len(pw):
		return errors.New("Remove spaces at the start or end")
	}
	return nil
}

func Same(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }
