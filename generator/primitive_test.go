package generator

import (
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
)

func TestPrimitiveKinds(t *testing.T) {
	cases := []struct {
		p    model.Primitive
		kind value.Kind
	}{
		{model.Primitive{Kind: model.PrimString}, value.KindString},
		{model.Primitive{Kind: model.PrimInt}, value.KindInt},
		{model.Primitive{Kind: model.PrimFloat}, value.KindFloat},
		{model.Primitive{Kind: model.PrimBool}, value.KindBool},
		{model.Primitive{Kind: model.PrimUUID}, value.KindUUID},
		{model.Primitive{Kind: model.PrimDate}, value.KindTime},
		{model.Primitive{Kind: model.PrimDatetime}, value.KindTime},
		{model.Primitive{Kind: model.PrimTime}, value.KindTime},
		{model.Primitive{Kind: model.PrimDuration}, value.KindTime},
		{model.Primitive{Kind: model.PrimBytes}, value.KindString},
		{model.Primitive{Kind: model.PrimDecimal, Params: []int{10, 2}}, value.KindDecimal},
		{model.Primitive{Kind: model.PrimNull}, value.KindNull},
	}
	rng := NewRand(42)
	for _, c := range cases {
		v := generatePrimitive(c.p, rng)
		if v.Kind != c.kind {
			t.Errorf("%s: got Kind %v want %v", c.p.Kind, v.Kind, c.kind)
		}
	}
}

func TestPrimitiveStringLength(t *testing.T) {
	rng := NewRand(1)
	for i := 0; i < 32; i++ {
		v := generatePrimitive(model.Primitive{Kind: model.PrimString}, rng)
		if v.Kind != value.KindString {
			t.Fatalf("not a string: %v", v)
		}
		if len(v.S) < 8 || len(v.S) > 16 {
			t.Fatalf("string length out of 8..16: %d (%q)", len(v.S), v.S)
		}
	}
}

func TestPrimitiveDeterminismSameSeed(t *testing.T) {
	a := NewRand(7)
	b := NewRand(7)
	for i := 0; i < 16; i++ {
		va := generatePrimitive(model.Primitive{Kind: model.PrimString}, a)
		vb := generatePrimitive(model.Primitive{Kind: model.PrimString}, b)
		if va.S != vb.S {
			t.Fatalf("iter %d: %q != %q", i, va.S, vb.S)
		}
	}
}

func TestPrimitiveUUIDv4(t *testing.T) {
	rng := NewRand(13)
	v := generatePrimitive(model.Primitive{Kind: model.PrimUUID}, rng)
	if v.Kind != value.KindUUID {
		t.Fatalf("not uuid: %v", v)
	}
	if v.U.Version() != 4 {
		t.Fatalf("expected v4, got %d", v.U.Version())
	}
	// Variant should be RFC4122 (10xxxxxx in byte 8).
	if v.U[8]&0xC0 != 0x80 {
		t.Fatalf("wrong variant byte: %02x", v.U[8])
	}
}

func TestPrimitiveDecimalScale(t *testing.T) {
	rng := NewRand(21)
	v := generatePrimitive(model.Primitive{Kind: model.PrimDecimal, Params: []int{8, 3}}, rng)
	if v.Kind != value.KindDecimal {
		t.Fatalf("not decimal: %v", v)
	}
	if v.D.Exponent() < -3 {
		t.Fatalf("decimal exceeds scale=3: %s (exp=%d)", v.D.String(), v.D.Exponent())
	}
}
