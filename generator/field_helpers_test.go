package generator

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

func TestFieldDecoratorHelpers(t *testing.T) {
	rng := NewRand(1)
	decs := []model.Decorator{
		{Name: "len", Args: []model.DecoratorArg{{Kind: model.ArgRange, From: "3", To: "5"}}},
		{Name: "count", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: int64(2)}}},
		{Name: "multiple_of", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: int64(5)}}},
	}
	if lo, hi := countRange(decs, 0, 0); lo != 2 || hi != 2 {
		t.Fatalf("countRange=%d..%d", lo, hi)
	}
	if lo, hi := lenBounds(decs[0].Args[0]); lo != 3 || hi != 5 {
		t.Fatalf("lenBounds=%d..%d", lo, hi)
	}
	if got := applyMultipleOf(value.Int(17), decs); got.I != 15 {
		t.Fatalf("multiple_of int=%v", got)
	}
	if got := applyMultipleOf(value.Float(17.9), decs); got.F != 15 {
		t.Fatalf("multiple_of float=%v", got)
	}
	short := applyLen(value.Str("x"), decs, rng)
	if len(short.S) < 3 || len(short.S) > 5 {
		t.Fatalf("applyLen string=%q", short.S)
	}
	list := applyLen(value.List([]value.Value{value.Int(1), value.Int(2), value.Int(3), value.Int(4), value.Int(5), value.Int(6)}), decs, rng)
	if len(list.L) > 5 {
		t.Fatalf("applyLen list len=%d", len(list.L))
	}
	if firstFloatArg([]model.DecoratorArg{{Kind: model.ArgLiteral, Literal: float64(0.25)}}, 0) != 0.25 {
		t.Fatal("firstFloatArg failed")
	}
}

func TestLiteralAndDisplayHelpers(t *testing.T) {
	cases := []model.DecoratorArg{
		{Kind: model.ArgLiteral, Literal: "x"},
		{Kind: model.ArgLiteral, Literal: int64(2)},
		{Kind: model.ArgLiteral, Literal: float64(3.5)},
		{Kind: model.ArgLiteral, Literal: true},
		{Kind: model.ArgIdent, Ident: "ident", Raw: "ident"},
		{Kind: model.ArgRange, Raw: "1..2"},
	}
	for _, c := range cases {
		if literalAsValue(c).Kind == value.KindNull {
			t.Fatalf("literalAsValue returned null for %+v", c)
		}
		if decoratorLiteralString(c) == "" {
			t.Fatalf("decoratorLiteralString empty for %+v", c)
		}
	}
	obj := value.NewObject()
	obj.Set("a", value.Int(1))
	vals := []value.Value{
		value.Str("x"),
		value.Int(1),
		value.Float(1.5),
		value.Bool(true),
		value.UUID(uuid.Nil),
		value.Time(time.Unix(0, 0)),
		value.Dec(decimal.NewFromInt(9)),
		value.List([]value.Value{value.Int(1)}),
		value.Obj(obj),
		value.Null(),
	}
	for _, v := range vals {
		if valueKey(v) == "" {
			t.Fatalf("empty valueKey for %+v", v)
		}
	}
	if !strings.Contains(valueDisplay(value.Time(time.Unix(0, 0))), "1970") {
		t.Fatal("time display mismatch")
	}
}

func TestApplyRangeClampsDecimal(t *testing.T) {
	rangeArg := func(from, to string, loExcl, hiExcl bool) []model.Decorator {
		return []model.Decorator{{Name: "range", Args: []model.DecoratorArg{{
			Kind: model.ArgRange, From: from, To: to, LoExcl: loExcl, HiExcl: hiExcl,
		}}}}
	}
	dec := func(s string) value.Value {
		d, err := decimal.NewFromString(s)
		if err != nil {
			t.Fatalf("decimal %q: %v", s, err)
		}
		return value.Dec(d)
	}
	cases := []struct {
		name string
		in   value.Value
		decs []model.Decorator
		want string
	}{
		{"above hi clamps", dec("96714809.21"), rangeArg("0", "10000", false, false), "10000"},
		{"below lo clamps", dec("-3.5"), rangeArg("0", "5", false, false), "0"},
		{"inside untouched", dec("3.7"), rangeArg("0", "5", false, false), "3.7"},
		{"hi exclusive steps by scale", dec("7.25"), rangeArg("0", "5", false, true), "4.99"},
		{"lo exclusive steps by scale", dec("-1.0"), rangeArg("0", "5", true, false), "0.1"},
		{"bad bounds untouched", dec("42"), rangeArg("x", "y", false, false), "42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyRange(tc.in, tc.decs)
			if got.Kind != value.KindDecimal || !got.D.Equal(decimal.RequireFromString(tc.want)) {
				t.Fatalf("applyRange = %v, want %s", got.D, tc.want)
			}
		})
	}
}
