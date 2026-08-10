package jsonpath

import (
	"errors"
	"strconv"
)

var (
	// ErrPathParse is returned when a JSONPath expression cannot be parsed.
	ErrPathParse = errors.New("jsonpath: parse error")
	// ErrFunction is returned for function registration or expression failures.
	ErrFunction = errors.New("jsonpath: function error")
	// ErrUnmarshal is returned when JSON unmarshaling fails in QueryJSON functions.
	ErrUnmarshal = errors.New("jsonpath: unmarshal error")
	// ErrInvalidPath is returned for an invalid compiled or normalized path.
	ErrInvalidPath = errors.New("jsonpath: invalid path")
)

// ParseError describes a JSONPath parse failure in a program-readable form.
type ParseError struct {
	// Offset is the byte offset in the original expression where parsing failed.
	Offset int
	// Reason is a short human-readable reason for the failure.
	Reason string
	// Snippet is a short source excerpt around Offset.
	Snippet string
	// Cause is the underlying lexer, parser, or function validation error.
	Cause error
}

// Error returns a human-readable parse error message.
func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	if e.Snippet != "" {
		return e.Reason + " at position " + strconv.Itoa(e.Offset) + " near " + e.Snippet
	}
	if e.Offset >= 0 {
		return e.Reason + " at position " + strconv.Itoa(e.Offset)
	}
	return e.Reason
}

// Unwrap returns the underlying parse failure cause.
func (e *ParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
