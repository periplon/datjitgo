package generator

import (
	"strings"
	"unicode"

	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// dirtyTimeLayouts are the alternative renderings used by the format_mix
// corruption kind. The corrupted value intentionally degrades from time to
// string — mixed timestamp formats are the mess being simulated.
var dirtyTimeLayouts = []string{"01/02/2006", "2006-01-02 15:04:05", "Jan 2, 2006"}

// applyDirtyOp dispatches a single corruption operator. Every operator is a
// pure function of (value, substream) and consumes a fixed number of RNG
// draws regardless of the value's runtime content, so the decision stream
// depends only on (row index, static config) — no-op corruptions still
// consume their draws.
func applyDirtyOp(k dirtyKind, v value.Value, rng ports.Randomizer) value.Value {
	switch k {
	case dirtyTypo:
		return dirtyTypoOp(v, rng)
	case dirtyCase:
		return dirtyCaseOp(v, rng)
	case dirtyWhitespace:
		return dirtyWhitespaceOp(v, rng)
	case dirtyNull:
		return value.Null()
	case dirtyFormatMix:
		return dirtyFormatMixOp(v, rng)
	default:
		// dirtyDuplicate is row-level and handled by the engine post-pass.
		return v
	}
}

// dirtyTypoOp applies one of three single-character typos to a string value:
// swap two adjacent characters, drop a character, or double a character.
// Always two draws (operation + position); non-string or too-short values
// pass through unchanged.
func dirtyTypoOp(v value.Value, rng ports.Randomizer) value.Value {
	op := scaleDirtyIndex(rng.Float(), 3)
	pos := rng.Float()
	if v.Kind != value.KindString {
		return v
	}
	r := []rune(v.S)
	if len(r) < 2 {
		return v
	}
	switch op {
	case 0: // swap two adjacent characters
		i := scaleDirtyIndex(pos, len(r)-1)
		r[i], r[i+1] = r[i+1], r[i]
		v.S = string(r)
	case 1: // drop a character
		i := scaleDirtyIndex(pos, len(r))
		out := make([]rune, 0, len(r)-1)
		out = append(out, r[:i]...)
		out = append(out, r[i+1:]...)
		v.S = string(out)
	default: // double a character
		i := scaleDirtyIndex(pos, len(r))
		out := make([]rune, 0, len(r)+1)
		out = append(out, r[:i+1]...)
		out = append(out, r[i:]...)
		v.S = string(out)
	}
	return v
}

// dirtyCaseOp mangles letter case: UPPER, lower, or first-letter case swap.
// Always one draw; values without letters pass through unchanged.
func dirtyCaseOp(v value.Value, rng ports.Randomizer) value.Value {
	op := scaleDirtyIndex(rng.Float(), 3)
	if v.Kind != value.KindString || !strings.ContainsFunc(v.S, unicode.IsLetter) {
		return v
	}
	switch op {
	case 0:
		v.S = strings.ToUpper(v.S)
	case 1:
		v.S = strings.ToLower(v.S)
	default: // swap the case of the first letter
		r := []rune(v.S)
		for i, c := range r {
			if !unicode.IsLetter(c) {
				continue
			}
			if unicode.IsUpper(c) {
				r[i] = unicode.ToLower(c)
			} else {
				r[i] = unicode.ToUpper(c)
			}
			break
		}
		v.S = string(r)
	}
	return v
}

// dirtyWhitespaceOp injects stray whitespace: a leading space, a trailing
// space, or an internal double space at a seeded word boundary (falling back
// to a trailing space for single-word strings). Always two draws (operation
// + boundary); non-string values pass through unchanged.
func dirtyWhitespaceOp(v value.Value, rng ports.Randomizer) value.Value {
	op := scaleDirtyIndex(rng.Float(), 3)
	boundary := rng.Float()
	if v.Kind != value.KindString {
		return v
	}
	switch op {
	case 0:
		v.S = " " + v.S
	case 1:
		v.S += " "
	default:
		idxs := dirtySpaceOffsets(v.S)
		if len(idxs) == 0 {
			// Single word: no internal boundary — fall back to trailing.
			v.S += " "
			return v
		}
		i := idxs[scaleDirtyIndex(boundary, len(idxs))]
		v.S = v.S[:i] + " " + v.S[i:]
	}
	return v
}

// dirtyFormatMixOp re-renders a time value as a string in one of the
// dirtyTimeLayouts. Always one draw; non-time values pass through unchanged.
func dirtyFormatMixOp(v value.Value, rng ports.Randomizer) value.Value {
	pick := scaleDirtyIndex(rng.Float(), len(dirtyTimeLayouts))
	if v.Kind != value.KindTime {
		return v
	}
	return value.Str(v.T.Format(dirtyTimeLayouts[pick]))
}

// scaleDirtyIndex maps a uniform float in [0, 1) onto an index in [0, n).
// Operators use it instead of IntN so that the number of draws they consume
// never depends on a value-derived modulus.
func scaleDirtyIndex(f float64, n int) int {
	if n <= 0 {
		return 0
	}
	i := int(f * float64(n))
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// dirtySpaceOffsets returns the byte offsets of every ASCII space in s.
// Used to pick an internal word boundary for whitespace corruption.
func dirtySpaceOffsets(s string) []int {
	var out []int
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			out = append(out, i)
		}
	}
	return out
}
