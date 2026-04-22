package generator

import (
	"fmt"
	"strings"

	"github.com/jmcarbo/datjitgo/core/ports"
)

// patternWords powers the {word}/{WORD} placeholders. Intentionally the same
// short NATO-inspired list used by the Rust reference implementation for
// parity of behaviour.
var patternWords = []string{
	"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
	"india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa",
	"quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey",
	"xray", "yankee", "zulu",
}

// seqCounters maps a caller-provided key (typically "<entity>.<field>") to a
// monotonically-increasing counter used by the `{seq}` placeholder.
type seqCounters struct {
	m map[string]uint64
}

func newSeqCounters() *seqCounters { return &seqCounters{m: map[string]uint64{}} }

func (s *seqCounters) next(key string) uint64 {
	if s.m == nil {
		s.m = map[string]uint64{}
	}
	s.m[key]++
	return s.m[key]
}

// expandPattern rewrites template replacing each `{…}` placeholder per
// spec §3.7. Unrecognised placeholders are preserved verbatim, and an
// unbalanced `{` with no closing `}` is emitted as a literal.
//
// The counterKey is used by the `{seq}` placeholder; separate fields should
// pass distinct keys so their sequences don't collide.
func expandPattern(template string, rng ports.Randomizer, counters *seqCounters, counterKey string) string {
	var b strings.Builder
	b.Grow(len(template) + 16)

	i := 0
	for i < len(template) {
		if template[i] != '{' {
			b.WriteByte(template[i])
			i++
			continue
		}
		// Find matching '}'
		close := strings.IndexByte(template[i+1:], '}')
		if close < 0 {
			b.WriteString(template[i:])
			break
		}
		placeholder := template[i+1 : i+1+close]
		expandPlaceholder(placeholder, rng, counters, counterKey, &b)
		i += close + 2
	}
	return b.String()
}

func expandPlaceholder(ph string, rng ports.Randomizer, counters *seqCounters, counterKey string, out *strings.Builder) {
	switch ph {
	case "uuid":
		u := randomUUIDv4(rng)
		out.WriteString(u.String())
		return
	case "seq":
		out.WriteString(fmt.Sprintf("%d", counters.next(counterKey)))
		return
	case "word":
		w := patternWords[rng.IntN(int64(len(patternWords)))]
		out.WriteString(w)
		return
	case "WORD":
		w := patternWords[rng.IntN(int64(len(patternWords)))]
		out.WriteString(strings.ToUpper(w))
		return
	}

	if ph == "" {
		out.WriteString("{}")
		return
	}

	switch {
	case allRune(ph, 'A'):
		for i := 0; i < len(ph); i++ {
			out.WriteByte(byte('A' + rng.IntN(26)))
		}
	case allRune(ph, 'a'):
		for i := 0; i < len(ph); i++ {
			out.WriteByte(byte('a' + rng.IntN(26)))
		}
	case allRune(ph, '0'):
		out.WriteString(fmt.Sprintf("%0*d", len(ph), rng.IntN(pow10(int64(len(ph))))))
	case allRune(ph, '#'):
		for i := 0; i < len(ph); i++ {
			out.WriteString(fmt.Sprintf("%X", rng.IntN(16)))
		}
	default:
		out.WriteByte('{')
		out.WriteString(ph)
		out.WriteByte('}')
	}
}

func allRune(s string, r byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != r {
			return false
		}
	}
	return true
}

func pow10(n int64) int64 {
	r := int64(1)
	for i := int64(0); i < n; i++ {
		r *= 10
	}
	return r
}
