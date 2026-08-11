package errcode

import (
	"errors"
	"fmt"
)

// Prefix is what every rendered code starts with. Support tooling greps for it,
// so it is a constant rather than a literal repeated in two places.
const Prefix = "TF"

// WithCode returns msg with a "(TF-NNNN)" suffix.
//
// Only for failures. A success message carrying a code would read as though
// something had gone wrong.
func WithCode(code int, msg string) string {
	return fmt.Sprintf("%s (%s-%d)", msg, Prefix, code)
}

// Errorf is WithCode for the call sites that build an error rather than a
// string.
//
// server/preferences.go hands err.Error() straight to the reader, so those
// messages need coding too, and spelling that out as
// errors.New(WithCode(code, fmt.Sprintf(...))) at every one of them buries the
// message inside two layers of call.
func Errorf(code int, format string, args ...any) error {
	return errors.New(WithCode(code, fmt.Sprintf(format, args...)))
}
