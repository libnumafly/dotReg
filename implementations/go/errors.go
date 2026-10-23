package dotreg

import "fmt"

// Code identifies a sample implementation error without including input values.
type Code string

const (
	ErrSyntax         Code = "syntax"
	ErrUnsupported    Code = "unsupported"
	ErrInput          Code = "input"
	ErrDuplicate      Code = "duplicate"
	ErrString         Code = "string"
	ErrTypeMismatch   Code = "type_mismatch"
	ErrTypeConversion Code = "type_conversion"
	ErrMissing        Code = "missing"
)

// Error deliberately omits raw values and lower-level reader/converter errors.
// These error codes are sample API choices, not additions to the format grammar.
type Error struct {
	Line int
	Code Code
}

func (e *Error) Error() string {
	message := "configuration error"
	switch e.Code {
	case ErrSyntax:
		message = "invalid configuration syntax"
	case ErrUnsupported:
		message = "syntax outside the sample's supported subset"
	case ErrInput:
		message = "invalid input or input read failed"
	case ErrDuplicate:
		message = "duplicate value key"
	case ErrString:
		message = "invalid quoted string"
	case ErrTypeMismatch:
		message = "requested type does not match"
	case ErrTypeConversion:
		message = "declared type conversion failed"
	case ErrMissing:
		message = "key not found"
	}
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, message)
	}
	return message
}

func problem(line int, code Code) error { return &Error{Line: line, Code: code} }
