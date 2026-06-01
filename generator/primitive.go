package generator

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// alphanum is the character class for the default string primitive.
const alphanum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// now returns the reference "now" used by date/datetime/time/duration
// primitive generation. Split into its own function so tests can swap it in
// future (the spec-mandated range is [now-10y, now+1y], so any shift in the
// reference moment trickles through).
var now = func() time.Time {
	return time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
}

// generatePrimitive produces a value for the given primitive type using rng.
// Phase-1 defaults mirror the spec:
//   - string → 8..16 alphanumeric characters
//   - int    → uniform in [0, 1e6) (signed ±1e6 when bit-width is unspecified)
//   - float  → uniform in [0, 1)
//   - bool   → 50/50
//   - uuid   → v4 drawn from rng (not crypto/rand)
//   - date/datetime/time/duration → uniform in [now-10y, now+1y]
//   - bytes  → 16 random bytes
//   - decimal(p,s) → random value in [0, 10^(p-s)) truncated to scale s
func generatePrimitive(p model.Primitive, rng ports.Randomizer) value.Value {
	switch p.Kind {
	case model.PrimString:
		return value.Str(randomAlphanum(rng, 8, 16))
	case model.PrimInt:
		// With no bit-width param, stay well within int64. Symmetric range.
		return value.Int(rng.IntN(2_000_001) - 1_000_000)
	case model.PrimFloat:
		return value.Float(rng.Float())
	case model.PrimBool:
		return value.Bool(rng.IntN(2) == 0)
	case model.PrimUUID:
		return value.UUID(randomUUIDv4(rng))
	case model.PrimDate:
		return value.Time(randomTimeInDefaultRange(rng).Truncate(24 * time.Hour))
	case model.PrimDatetime:
		return value.Time(randomTimeInDefaultRange(rng))
	case model.PrimTime:
		// Time-of-day: random h/m/s on a zero date.
		h := rng.IntN(24)
		m := rng.IntN(60)
		s := rng.IntN(60)
		return value.Time(time.Date(0, 1, 1, int(h), int(m), int(s), 0, time.UTC))
	case model.PrimDuration:
		// Random duration up to 72h.
		secs := rng.IntN(72 * 3600)
		return value.Time(time.Time{}.Add(time.Duration(secs) * time.Second))
	case model.PrimBytes:
		// Writers layer is responsible for base64-encoding bytes; we store
		// them as a raw string (16 random bytes → 16 runes from 0..255).
		buf := make([]byte, 16)
		for i := range buf {
			buf[i] = byte(rng.IntN(256))
		}
		return value.Str(string(buf))
	case model.PrimDecimal:
		prec, scale := 10, 2
		if len(p.Params) >= 1 && p.Params[0] > 0 {
			prec = p.Params[0]
		}
		if len(p.Params) >= 2 && p.Params[1] >= 0 {
			scale = p.Params[1]
		}
		if scale > prec {
			scale = prec
		}
		maxVal := math.Pow(10, float64(prec-scale))
		v := rng.Float() * maxVal
		return value.Dec(decimal.NewFromFloat(v).Round(int32(scale)))
	case model.PrimNull:
		return value.Null()
	case model.PrimAny:
		// Match the Rust generator — pick one of int/string/bool.
		switch rng.IntN(3) {
		case 0:
			return value.Int(rng.IntN(1000))
		case 1:
			return value.Str(fmt.Sprintf("val_%d", rng.IntN(1000)))
		default:
			return value.Bool(rng.IntN(2) == 0)
		}
	}
	return value.Null()
}

// randomAlphanum returns a random string of length in [lo, hi].
func randomAlphanum(rng ports.Randomizer, lo, hi int) string {
	if hi < lo {
		hi = lo
	}
	span := int64(hi - lo + 1)
	n := lo + int(rng.IntN(span))
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = alphanum[rng.IntN(int64(len(alphanum)))]
	}
	return string(buf)
}

// randomTimeInDefaultRange returns a uniform random time in [now-10y, now+1y].
func randomTimeInDefaultRange(rng ports.Randomizer) time.Time {
	base := now()
	lo := base.AddDate(-10, 0, 0)
	hi := base.AddDate(1, 0, 0)
	span := hi.Sub(lo)
	if span <= 0 {
		return base
	}
	off := time.Duration(rng.IntN(int64(span)))
	return lo.Add(off)
}

// randomUUIDv4 draws 16 bytes from rng and sets the v4/RFC-4122 bits. Using
// the RNG (rather than crypto/rand) preserves determinism across runs.
func randomUUIDv4(rng ports.Randomizer) uuid.UUID {
	var u uuid.UUID
	a := uint64Of(rng)
	b := uint64Of(rng)
	for i := 0; i < 8; i++ {
		u[i] = byte(a >> (8 * i))
		u[i+8] = byte(b >> (8 * i))
	}
	// Version 4
	u[6] = (u[6] & 0x0F) | 0x40
	// RFC 4122 variant
	u[8] = (u[8] & 0x3F) | 0x80
	return u
}
