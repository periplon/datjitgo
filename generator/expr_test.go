package generator

import (
	"math"
	"testing"

	"github.com/jmcarbo/datjitgo/core/value"
)

func mkRow(pairs ...any) *value.Object {
	obj := value.NewObject()
	for i := 0; i+1 < len(pairs); i += 2 {
		obj.Set(pairs[i].(string), pairs[i+1].(value.Value))
	}
	return obj
}

func mustEval(t *testing.T, src string, row *value.Object) value.Value {
	t.Helper()
	node, err := parseExpr(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	v, err := evalExpr(node, evalEnv{row: row})
	if err != nil {
		t.Fatalf("eval %q: %v", src, err)
	}
	return v
}

func TestExprCases(t *testing.T) {
	row := mkRow(
		"a", value.Int(10),
		"b", value.Int(3),
		"name", value.Str("Alice"),
		"flag", value.Bool(true),
		"price", value.Float(2.5),
		"empty", value.Null(),
	)

	cases := []struct {
		src  string
		want value.Value
	}{
		{"a", value.Int(10)},
		{"a + b", value.Int(13)},
		{"a - b", value.Int(7)},
		{"a * b", value.Int(30)},
		{"a / b", value.Int(3)},
		{"a % b", value.Int(1)},
		{"2 + 3 * 4", value.Int(14)},
		{"(2 + 3) * 4", value.Int(20)},
		{"-a", value.Int(-10)},
		{"a > b", value.Bool(true)},
		{"a < b", value.Bool(false)},
		{"a >= 10", value.Bool(true)},
		{"a == 10", value.Bool(true)},
		{"a != 10", value.Bool(false)},
		{"flag and (a > 5)", value.Bool(true)},
		{"flag or false", value.Bool(true)},
		{"not flag", value.Bool(false)},
		{`"hi " + name`, value.Str("hi Alice")},
		{`lower(name)`, value.Str("alice")},
		{`upper(name)`, value.Str("ALICE")},
		{`concat("x", name, "!")`, value.Str("xAlice!")},
		{`starts_with(name, "Al")`, value.Bool(true)},
		{`ends_with(name, "ce")`, value.Bool(true)},
		{`if(a > 5, "hi", "lo")`, value.Str("hi")},
		{`a in [1, 2, 10]`, value.Bool(true)},
		{`b in [7, 8, 9]`, value.Bool(false)},
	}
	for _, c := range cases {
		got := mustEval(t, c.src, row)
		if !valuesEqual(got, c.want) {
			t.Errorf("%q: got %+v want %+v", c.src, got, c.want)
		}
	}
}

func TestExprRoundFloat(t *testing.T) {
	row := mkRow("x", value.Float(3.14159))
	got := mustEval(t, "round(x, 2)", row)
	if math.Abs(got.F-3.14) > 1e-9 {
		t.Fatalf("round result: %v", got)
	}
}

func TestExprUnresolvedPath(t *testing.T) {
	got := mustEval(t, "nope", mkRow())
	if got.Kind != value.KindNull {
		t.Fatalf("want null, got %v", got)
	}
}

func TestExprDivisionByZeroErrors(t *testing.T) {
	node, err := parseExpr("1/0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := evalExpr(node, evalEnv{}); err == nil {
		t.Fatal("expected division-by-zero error")
	}
}

func TestExprStringNumberPromotion(t *testing.T) {
	row := mkRow("qty", value.Int(5))
	got := mustEval(t, `"count=" + qty`, row)
	if got.Kind != value.KindString || got.S != "count=5" {
		t.Fatalf("string+int failed: %+v", got)
	}
}

func TestExprIfThenRewrite(t *testing.T) {
	row := mkRow("status", value.Str("shipped"), "shipped_at", value.Str("2026-01-01"))
	got, err := evalRule(`if status == "shipped" then shipped_at != null`, "Order", row, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != value.KindBool || !got.B {
		t.Fatalf("rule should hold: %+v", got)
	}
}
