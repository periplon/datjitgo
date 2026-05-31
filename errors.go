package datjit

import (
	stderrors "errors"

	derrs "github.com/periplon/datjitgo/core/errors"
)

// IsParseError reports whether err is a datjit parse error.
func IsParseError(err error) bool {
	return stderrors.Is(err, derrs.ErrParse)
}

// IsValidationError reports whether err is a datjit validation error.
func IsValidationError(err error) bool {
	return stderrors.Is(err, derrs.ErrValidation)
}

// IsGenerationError reports whether err is a datjit generation error.
func IsGenerationError(err error) bool {
	return stderrors.Is(err, derrs.ErrGeneration)
}

// IsCorpusError reports whether err is a datjit corpus lookup/update error.
func IsCorpusError(err error) bool {
	return stderrors.Is(err, derrs.ErrCorpusMissing)
}
