package parser

import (
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
)

func TestSplitTypeAndDecorators_Simple(t *testing.T) {
	typ, decs, err := splitTypeAndDecorators("uuid @primary @unique")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != "uuid" {
		t.Fatalf("type=%q, want uuid", typ)
	}
	if len(decs) != 2 {
		t.Fatalf("expected 2 decorators, got %d: %+v", len(decs), decs)
	}
	if decs[0].Name != "primary" || decs[1].Name != "unique" {
		t.Fatalf("decorator names wrong: %+v", decs)
	}
}

func TestSplitTypeAndDecorators_NoDecorators(t *testing.T) {
	typ, decs, err := splitTypeAndDecorators("string")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "string" || len(decs) != 0 {
		t.Fatalf("typ=%q decs=%+v", typ, decs)
	}
}

func TestSplitTypeAndDecorators_NestedCommas(t *testing.T) {
	typ, decs, err := splitTypeAndDecorators("int @range(18..65) @dist(normal, mu=35, sigma=12)")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "int" {
		t.Fatalf("type=%q", typ)
	}
	if len(decs) != 2 {
		t.Fatalf("expected 2 decorators, got %d: %+v", len(decs), decs)
	}
	if decs[1].Name != "dist" || len(decs[1].Args) != 3 {
		t.Fatalf("dist args wrong: %+v", decs[1])
	}
	if decs[1].Args[0].Kind != model.ArgIdent || decs[1].Args[0].Ident != "normal" {
		t.Fatalf("arg0 should be ident 'normal': %+v", decs[1].Args[0])
	}
	if decs[1].Args[1].Kind != model.ArgKV || decs[1].Args[1].Key != "mu" || decs[1].Args[1].Value != "35" {
		t.Fatalf("arg1 should be mu=35: %+v", decs[1].Args[1])
	}
}

func TestSplitTypeAndDecorators_QuotedPattern(t *testing.T) {
	typ, decs, err := splitTypeAndDecorators(`string @pattern("SKU-{AA}-{0000}")`)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "string" {
		t.Fatalf("type=%q", typ)
	}
	if len(decs) != 1 {
		t.Fatalf("decs=%+v", decs)
	}
	if decs[0].Name != "pattern" || len(decs[0].Args) != 1 {
		t.Fatalf("pattern decorator wrong: %+v", decs[0])
	}
	arg := decs[0].Args[0]
	if arg.Kind != model.ArgLiteral {
		t.Fatalf("arg kind=%v not literal", arg.Kind)
	}
	lit, ok := arg.Literal.(string)
	if !ok || lit != "SKU-{AA}-{0000}" {
		t.Fatalf("pattern literal=%v want SKU-{AA}-{0000}", arg.Literal)
	}
}

func TestSplitTypeAndDecorators_RangeExclusive(t *testing.T) {
	cases := []struct {
		src              string
		from, to         string
		loExcl, hiExcl   bool
	}{
		{"int @range(1..10)", "1", "10", false, false},
		{"int @range(1<..10)", "1", "10", true, false},
		{"int @range(1..<10)", "1", "10", false, true},
		{"int @range(1<..<10)", "1", "10", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			_, decs, err := splitTypeAndDecorators(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			if len(decs) != 1 || len(decs[0].Args) != 1 {
				t.Fatalf("expected one range arg: %+v", decs)
			}
			arg := decs[0].Args[0]
			if arg.Kind != model.ArgRange {
				t.Fatalf("expected range arg, got %v", arg.Kind)
			}
			if arg.From != tc.from || arg.To != tc.to || arg.LoExcl != tc.loExcl || arg.HiExcl != tc.hiExcl {
				t.Fatalf("range parse wrong: %+v", arg)
			}
		})
	}
}

func TestSplitTypeAndDecorators_KVGreekLetters(t *testing.T) {
	_, decs, err := splitTypeAndDecorators("float @dist(normal, μ=0.5, σ=1.2)")
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 1 || len(decs[0].Args) != 3 {
		t.Fatalf("expected 3 args: %+v", decs)
	}
	a1 := decs[0].Args[1]
	if a1.Kind != model.ArgKV || a1.Key != "μ" || a1.Value != "0.5" {
		t.Fatalf("μ kv wrong: %+v", a1)
	}
	a2 := decs[0].Args[2]
	if a2.Kind != model.ArgKV || a2.Key != "σ" || a2.Value != "1.2" {
		t.Fatalf("σ kv wrong: %+v", a2)
	}
}

func TestSplitTypeAndDecorators_KVMuIdent(t *testing.T) {
	// kv where the value is a bare identifier (e.g. mu=X)
	_, decs, err := splitTypeAndDecorators("float @dist(normal, mu=X)")
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 1 || len(decs[0].Args) != 2 {
		t.Fatalf("unexpected: %+v", decs)
	}
	a := decs[0].Args[1]
	if a.Kind != model.ArgKV || a.Key != "mu" || a.Value != "X" {
		t.Fatalf("mu=X wrong: %+v", a)
	}
}

func TestSplitTypeAndDecorators_BareIdentAndNumbers(t *testing.T) {
	_, decs, err := splitTypeAndDecorators("int @dist(70, 25, 5)")
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 1 || len(decs[0].Args) != 3 {
		t.Fatalf("unexpected: %+v", decs)
	}
	for i, a := range decs[0].Args {
		if a.Kind != model.ArgLiteral {
			t.Fatalf("arg[%d] not literal: %+v", i, a)
		}
		if _, ok := a.Literal.(int64); !ok {
			t.Fatalf("arg[%d] literal not int64: %T %v", i, a.Literal, a.Literal)
		}
	}
}

func TestSplitTypeAndDecorators_SignedNumbers(t *testing.T) {
	_, decs, err := splitTypeAndDecorators("float @range(-5..10) @dist(normal, mu=-1.5)")
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 2 {
		t.Fatalf("unexpected: %+v", decs)
	}
	r := decs[0].Args[0]
	if r.Kind != model.ArgRange || r.From != "-5" || r.To != "10" {
		t.Fatalf("signed range: %+v", r)
	}
	kv := decs[1].Args[1]
	if kv.Kind != model.ArgKV || kv.Key != "mu" || kv.Value != "-1.5" {
		t.Fatalf("signed kv: %+v", kv)
	}
}

func TestSplitTypeAndDecorators_QuotedDefault(t *testing.T) {
	_, decs, err := splitTypeAndDecorators(`enum(active, pending, disabled) @default("pending")`)
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 1 || len(decs[0].Args) != 1 {
		t.Fatalf("unexpected: %+v", decs)
	}
	a := decs[0].Args[0]
	if a.Kind != model.ArgLiteral {
		t.Fatalf("expected literal arg: %+v", a)
	}
	s, ok := a.Literal.(string)
	if !ok || s != "pending" {
		t.Fatalf("default literal wrong: %v", a.Literal)
	}
}

func TestSplitTypeAndDecorators_NullRateFloat(t *testing.T) {
	_, decs, err := splitTypeAndDecorators("string @null_rate(0.3)")
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 1 || len(decs[0].Args) != 1 {
		t.Fatalf("unexpected: %+v", decs)
	}
	a := decs[0].Args[0]
	if a.Kind != model.ArgLiteral {
		t.Fatalf("expected literal: %+v", a)
	}
	if f, ok := a.Literal.(float64); !ok || f != 0.3 {
		t.Fatalf("float literal wrong: %T %v", a.Literal, a.Literal)
	}
}

func TestSplitTypeAndDecorators_EnumInlineStaysInType(t *testing.T) {
	typ, decs, err := splitTypeAndDecorators("enum(free, pro, enterprise) @dist(70, 25, 5)")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "enum(free, pro, enterprise)" {
		t.Fatalf("type=%q", typ)
	}
	if len(decs) != 1 || decs[0].Name != "dist" {
		t.Fatalf("decs=%+v", decs)
	}
}

func TestSplitTypeAndDecorators_LlmWithComma(t *testing.T) {
	typ, decs, err := splitTypeAndDecorators(`string @llm("Write a short tagline, max 8 words", temperature: 1.2)`)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "string" {
		t.Fatalf("type=%q", typ)
	}
	if len(decs) != 1 || decs[0].Name != "llm" {
		t.Fatalf("unexpected: %+v", decs)
	}
	// First arg is quoted string, second arg is key:value (colon-form) — we keep raw
	if len(decs[0].Args) < 1 {
		t.Fatalf("expected at least one arg: %+v", decs[0])
	}
	if decs[0].Args[0].Kind != model.ArgLiteral {
		t.Fatalf("first arg should be literal: %+v", decs[0].Args[0])
	}
	s := decs[0].Args[0].Literal.(string)
	if s != "Write a short tagline, max 8 words" {
		t.Fatalf("prompt=%q", s)
	}
}
