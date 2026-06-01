// Package errors defines the single error type used across datjitgo.
package errors

import "fmt"

// ErrorKind classifies an Error by the pipeline stage or failure mode it
// represents. Callers match on it via the package sentinels and errors.Is.
type ErrorKind int

// The Kind constants enumerate every error class datjitgo reports.
const (
	KindUnknown ErrorKind = iota
	KindParse
	KindValidation
	KindGeneration
	KindUniquenessExhausted
	KindRuleViolated
	KindIO
	KindFeatureDeferred
	KindCorpusMissing
	KindCyclicDependency
)

func (k ErrorKind) String() string {
	switch k {
	case KindParse:
		return "parse error"
	case KindValidation:
		return "validation error"
	case KindGeneration:
		return "generation error"
	case KindUniquenessExhausted:
		return "uniqueness exhausted"
	case KindRuleViolated:
		return "rule violated"
	case KindIO:
		return "io error"
	case KindFeatureDeferred:
		return "feature deferred"
	case KindCorpusMissing:
		return "corpus missing"
	case KindCyclicDependency:
		return "cyclic dependency"
	default:
		// unknown kinds fall through to the trailing return
	}
	return "error"
}

// Location identifies a position in a schema source for diagnostics.
type Location struct {
	File string
	Line int
	Col  int
}

// Error is the single error type used across datjitgo. Kind classifies it;
// the optional Entity, Field, and Location enrich diagnostics, and Cause
// chains an underlying error for errors.Unwrap/Is/As.
type Error struct {
	Kind     ErrorKind
	Entity   string
	Field    string
	Location *Location
	Message  string
	Cause    error
}

func (e *Error) Error() string {
	loc := ""
	if e.Location != nil {
		loc = fmt.Sprintf(" at %s:%d:%d", e.Location.File, e.Location.Line, e.Location.Col)
	}
	ent := ""
	if e.Entity != "" {
		ent = " [" + e.Entity
		if e.Field != "" {
			ent += "." + e.Field
		}
		ent += "]"
	}
	return fmt.Sprintf("%s%s%s: %s", e.Kind, loc, ent, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

// Is matches sentinels by Kind so callers can do errors.Is(err, errors.ErrParse).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return t.Kind == e.Kind
}

// Sentinels — compare with errors.Is.
var (
	ErrParse               = &Error{Kind: KindParse}
	ErrValidation          = &Error{Kind: KindValidation}
	ErrGeneration          = &Error{Kind: KindGeneration}
	ErrUniquenessExhausted = &Error{Kind: KindUniquenessExhausted}
	ErrRuleViolated        = &Error{Kind: KindRuleViolated}
	ErrIO                  = &Error{Kind: KindIO}
	ErrFeatureDeferred     = &Error{Kind: KindFeatureDeferred}
	ErrCorpusMissing       = &Error{Kind: KindCorpusMissing}
	ErrCyclicDependency    = &Error{Kind: KindCyclicDependency}
)

// Parsef builds a parse error at the given location.
func Parsef(loc *Location, format string, a ...any) *Error {
	return &Error{Kind: KindParse, Location: loc, Message: fmt.Sprintf(format, a...)}
}

// Validationf builds a validation error.
func Validationf(format string, a ...any) *Error {
	return &Error{Kind: KindValidation, Message: fmt.Sprintf(format, a...)}
}

// Generationf builds a generation error.
func Generationf(format string, a ...any) *Error {
	return &Error{Kind: KindGeneration, Message: fmt.Sprintf(format, a...)}
}

// Wrap wraps cause with a new Error of the given kind and message.
func Wrap(kind ErrorKind, cause error, format string, a ...any) *Error {
	return &Error{Kind: kind, Cause: cause, Message: fmt.Sprintf(format, a...)}
}
