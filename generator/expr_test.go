package generator

import (
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/periplon/datjitgo/core/value"
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

func TestExprAggregateAndDateFunctions(t *testing.T) {
	data := map[string][]*value.Object{
		"Order": {
			mkRow("amount", value.Int(10)),
			mkRow("amount", value.Int(20)),
			mkRow("amount", value.Int(30)),
		},
	}
	cases := map[string]value.Value{
		"sum(Order.amount)":                        value.Int(60),
		"count(Order.amount)":                      value.Int(3),
		"avg(Order.amount)":                        value.Float(20),
		"min(Order.amount)":                        value.Float(10),
		"max(Order.amount)":                        value.Float(30),
		`slug("Hello, Datjit Go!")`:                value.Str("hello-datjit-go"),
		`days_between("2026-01-01", "2026-01-11")`: value.Int(10),
		`years_since("not-a-date")`:                value.Int(0),
	}
	for src, want := range cases {
		node, err := parseExpr(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		got, err := evalExpr(node, evalEnv{data: data})
		if err != nil {
			t.Fatalf("eval %q: %v", src, err)
		}
		if !valuesEqual(got, want) {
			t.Fatalf("%q: got %+v want %+v", src, got, want)
		}
	}
}

func TestExprParseAndEvalErrorBranches(t *testing.T) {
	if err := ParseExpr("1 + 2"); err != nil {
		t.Fatal(err)
	}
	if err := ParseExpr("(1 +"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := evalExpr(exprNode{kind: exprFunc, str: "missing"}, evalEnv{}); err == nil {
		t.Fatal("expected unknown function error")
	}
	if _, err := compareValues(value.Bool(true), value.Str("x")); err == nil {
		t.Fatal("expected compare error")
	}
}

func TestExprDirectEvaluatorBranches(t *testing.T) {
	obj := value.NewObject()
	obj.Set("amount", value.Int(7))
	row := mkRow(
		"items", value.List([]value.Value{value.Obj(obj)}),
		"when", value.Time(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
	)
	data := map[string][]*value.Object{"Order": {mkRow("amount", value.Int(4)), mkRow("amount", value.Int(6))}}
	if got := resolvePath("items.amount", evalEnv{row: row}); got.Kind != value.KindList || len(got.L) != 1 {
		t.Fatalf("list path: %+v", got)
	}
	if got := resolvePath("Order.amount", evalEnv{data: data}); got.Kind != value.KindList || len(got.L) != 2 {
		t.Fatalf("entity path: %+v", got)
	}
	if got, err := evalBinary("%", value.Float(5.5), value.Float(2)); err != nil || got.F == 0 {
		t.Fatalf("float modulo: %+v %v", got, err)
	}
	if _, err := evalBinary("%", value.Int(1), value.Int(0)); err == nil {
		t.Fatal("expected modulo by zero error")
	}
	if _, err := evalBinary("?", value.Int(1), value.Int(2)); err == nil {
		t.Fatal("expected unknown operator error")
	}
	if _, err := numericOp(value.Str("x"), value.Int(1), func(a, b int64) int64 { return a }, func(a, b float64) float64 { return a }); err == nil {
		t.Fatal("expected non-numeric error")
	}
	if f, ok := asFloat(value.Dec(decimal.NewFromFloat(1.25))); !ok || f != 1.25 {
		t.Fatalf("decimal float=%v %v", f, ok)
	}
	if !valuesEqual(value.Int(1), value.Float(1)) || valuesEqual(value.Null(), value.Int(0)) {
		t.Fatal("valuesEqual cross-kind mismatch")
	}
	if c, err := compareValues(value.Time(time.Unix(1, 0)), value.Time(time.Unix(2, 0))); err != nil || c >= 0 {
		t.Fatalf("time compare=%d %v", c, err)
	}
	for _, v := range []value.Value{value.Bool(false), value.Null(), value.Int(0), value.Float(0), value.Str("")} {
		if truthy(v) {
			t.Fatalf("%+v should be falsey", v)
		}
	}
	if !truthy(value.List(nil)) {
		t.Fatal("list should be truthy")
	}
	if got, err := evalIf([]exprNode{{kind: exprLit, val: value.Bool(false)}, {kind: exprLit, val: value.Str("yes")}, {kind: exprLit, val: value.Str("no")}}, evalEnv{}); err != nil || got.S != "no" {
		t.Fatalf("evalIf false branch: %+v %v", got, err)
	}
	if _, err := evalIf(nil, evalEnv{}); err == nil {
		t.Fatal("expected if arity error")
	}
	if got := firstListOrEntity([]value.Value{value.Int(3)}, nil, evalEnv{}); len(got) != 1 || got[0].I != 3 {
		t.Fatalf("firstList fallback: %+v", got)
	}
	if got := firstListOrEntity([]value.Value{value.Int(0)}, []exprNode{{kind: exprPath, str: "Order"}}, evalEnv{data: data}); len(got) != 2 {
		t.Fatalf("firstList entity: %+v", got)
	}
}
