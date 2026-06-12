package generator

import (
	stderrors "errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
	"github.com/periplon/datjitgo/parser"
)

// kvArg builds a key=value decorator argument for config-parse tests.
func kvArg(key, val string) model.DecoratorArg {
	return model.DecoratorArg{Kind: model.ArgKV, Raw: key + "=" + val, Key: key, Value: val}
}

// strArg builds a quoted-string decorator argument.
func strArg(s string) model.DecoratorArg {
	return model.DecoratorArg{Kind: model.ArgLiteral, Raw: `"` + s + `"`, Literal: s}
}

func TestParseSeriesConfig(t *testing.T) {
	d := &model.Decorator{Name: "series", Args: []model.DecoratorArg{
		kvArg("start", "2026-01-01T00:00:00Z"),
		kvArg("interval", "1m"),
		kvArg("jitter", "10s"),
	}}
	cfg, err := parseSeriesConfig(d)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !cfg.Start.Equal(want) {
		t.Errorf("start = %v, want %v", cfg.Start, want)
	}
	if cfg.Interval != time.Minute {
		t.Errorf("interval = %v, want 1m", cfg.Interval)
	}
	if cfg.Jitter != 10*time.Second {
		t.Errorf("jitter = %v, want 10s", cfg.Jitter)
	}
}

func TestParseSeriesConfigDayForms(t *testing.T) {
	d := &model.Decorator{Name: "series", Args: []model.DecoratorArg{
		kvArg("start", "2026-01-01"),
		kvArg("interval", "7d"),
	}}
	cfg, err := parseSeriesConfig(d)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !cfg.Start.Equal(want) {
		t.Errorf("start = %v, want %v", cfg.Start, want)
	}
	if cfg.Interval != 7*24*time.Hour {
		t.Errorf("interval = %v, want 168h", cfg.Interval)
	}
	if cfg.Jitter != 0 {
		t.Errorf("jitter default = %v, want 0", cfg.Jitter)
	}
}

func TestParseSeriesConfigErrors(t *testing.T) {
	cases := map[string][]model.DecoratorArg{
		"missing start":    {kvArg("interval", "1m")},
		"missing interval": {kvArg("start", "2026-01-01")},
		"bad start":        {kvArg("start", "yesterday"), kvArg("interval", "1m")},
		"bad interval":     {kvArg("start", "2026-01-01"), kvArg("interval", "fortnight")},
		"negative jitter":  {kvArg("start", "2026-01-01"), kvArg("interval", "1m"), kvArg("jitter", "-5s")},
	}
	for name, args := range cases {
		if _, err := parseSeriesConfig(&model.Decorator{Name: "series", Args: args}); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}

func TestParseWalkConfig(t *testing.T) {
	d := &model.Decorator{Name: "walk", Args: []model.DecoratorArg{
		kvArg("start", "100"),
		kvArg("drift", "0.1"),
		kvArg("volatility", "2.5"),
		kvArg("min", "0"),
		kvArg("max", "10000"),
	}}
	cfg, err := parseWalkConfig(d)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Start != 100 || cfg.Drift != 0.1 || cfg.Volatility != 2.5 {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Min == nil || *cfg.Min != 0 || cfg.Max == nil || *cfg.Max != 10000 {
		t.Errorf("clamps = %v..%v", cfg.Min, cfg.Max)
	}
}

func TestParseWalkConfigDefaults(t *testing.T) {
	cfg, err := parseWalkConfig(&model.Decorator{Name: "walk", Args: []model.DecoratorArg{kvArg("start", "-3")}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Start != -3 || cfg.Drift != 0 || cfg.Volatility != 1 || cfg.Min != nil || cfg.Max != nil {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestParseWalkConfigErrors(t *testing.T) {
	cases := map[string][]model.DecoratorArg{
		"missing start": {kvArg("drift", "1")},
		"bad start":     {kvArg("start", "high")},
		"bad drift":     {kvArg("start", "1"), kvArg("drift", "up")},
		"min above max": {kvArg("start", "1"), kvArg("min", "10"), kvArg("max", "5")},
	}
	for name, args := range cases {
		if _, err := parseWalkConfig(&model.Decorator{Name: "walk", Args: args}); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}

func TestParseChainConfig(t *testing.T) {
	d := &model.Decorator{Name: "chain", Args: []model.DecoratorArg{
		strArg("pending>shipped:3, pending>cancelled:1, shipped>delivered:1.0"),
		kvArg("start", "pending"),
	}}
	cfg, err := parseChainConfig(d)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Start != "pending" {
		t.Errorf("start = %q", cfg.Start)
	}
	edges := cfg.Transitions["pending"]
	if len(edges) != 2 || edges[0].To != "shipped" || edges[1].To != "cancelled" {
		t.Fatalf("pending edges = %+v", edges)
	}
	// 3:1 normalizes to 0.75 / 0.25.
	if edges[0].Prob != 0.75 || edges[1].Prob != 0.25 {
		t.Errorf("normalized probs = %v, %v", edges[0].Prob, edges[1].Prob)
	}
	// delivered and cancelled are absorbing: no outgoing transitions.
	if _, ok := cfg.Transitions["delivered"]; ok {
		t.Error("delivered should be absorbing")
	}
	if _, ok := cfg.Transitions["cancelled"]; ok {
		t.Error("cancelled should be absorbing")
	}
	want := []string{"pending", "shipped", "cancelled", "delivered"}
	if !reflect.DeepEqual(cfg.States, want) {
		t.Errorf("states = %v, want %v", cfg.States, want)
	}
}

func TestParseChainConfigErrors(t *testing.T) {
	cases := map[string][]model.DecoratorArg{
		"no table":          {kvArg("start", "pending")},
		"unquoted table":    {{Kind: model.ArgIdent, Raw: "pending", Ident: "pending"}},
		"empty table":       {strArg("  ")},
		"malformed entry":   {strArg("pending-shipped:1")},
		"missing prob":      {strArg("pending>shipped")},
		"prob not a number": {strArg("pending>shipped:often")},
		"zero prob":         {strArg("pending>shipped:0")},
		"negative prob":     {strArg("pending>shipped:-1")},
	}
	for name, args := range cases {
		if _, err := parseChainConfig(&model.Decorator{Name: "chain", Args: args}); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}

// statefulSchema is a schema exercising all three decorators, used by the
// engine-level property tests below.
const statefulSchema = `
domain: stateful_test
seed: 7
volume:
  Metric: 50
enums:
  Status: [pending, shipped, delivered, cancelled]
entities:
  Metric:
    id: uuid @primary
    ts: datetime @series(start=2026-01-01T00:00:00Z, interval=1m, jitter=10s)
    day: date @series(start=2026-03-01, interval=1d)
    value: float @walk(start=100, drift=0.5, volatility=20, min=60, max=140)
    count: int @walk(start=10, drift=1, volatility=4, min=0, max=30)
    status: Status @chain("pending>shipped:0.8, pending>cancelled:0.2, shipped>delivered:1.0", start=pending)
`

func generateStateful(t *testing.T) []*value.Object {
	t.Helper()
	doc, err := parser.New().Parse(strings.NewReader(statefulSchema), "stateful.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := ds.Entities.Get("Metric")
	if !ok || len(rows) != 50 {
		t.Fatalf("Metric rows = %d", len(rows))
	}
	return rows
}

func TestSeriesMonotoneAndJitterBounded(t *testing.T) {
	rows := generateStateful(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var prev time.Time
	for i, row := range rows {
		v, _ := row.Get("ts")
		if v.Kind != value.KindTime {
			t.Fatalf("row %d: ts kind = %v", i, v.Kind)
		}
		// Jitter bound: |ts - (start + i·interval)| <= 10s.
		nominal := start.Add(time.Duration(i) * time.Minute)
		off := v.T.Sub(nominal)
		if off < -10*time.Second || off > 10*time.Second {
			t.Errorf("row %d: jitter offset %v out of [-10s, 10s]", i, off)
		}
		// Monotone non-decreasing because jitter (10s) < interval/2 (30s).
		if i > 0 && v.T.Before(prev) {
			t.Errorf("row %d: ts %v before previous %v", i, v.T, prev)
		}
		prev = v.T
	}
}

func TestSeriesDayIntervalExact(t *testing.T) {
	rows := generateStateful(t)
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	for i, row := range rows {
		v, _ := row.Get("day")
		want := start.AddDate(0, 0, i)
		if !v.T.Equal(want) {
			t.Errorf("row %d: day = %v, want %v", i, v.T, want)
		}
	}
}

func TestWalkClampedAndRounded(t *testing.T) {
	rows := generateStateful(t)
	v0, _ := rows[0].Get("value")
	if v0.F != 100 {
		t.Errorf("row 0: value = %v, want exactly 100 (start, no draw)", v0.F)
	}
	c0, _ := rows[0].Get("count")
	if c0.I != 10 {
		t.Errorf("row 0: count = %v, want exactly 10 (start, no draw)", c0.I)
	}
	hitLo, hitHi := false, false
	for i, row := range rows {
		v, _ := row.Get("value")
		if v.Kind != value.KindFloat {
			t.Fatalf("row %d: value kind = %v", i, v.Kind)
		}
		if v.F < 60 || v.F > 140 {
			t.Errorf("row %d: value %v outside [60, 140]", i, v.F)
		}
		if v.F == 60 {
			hitLo = true
		}
		if v.F == 140 {
			hitHi = true
		}
		if r := roundTo(v.F, 2); r != v.F {
			t.Errorf("row %d: value %v not rounded to 2 places", i, v.F)
		}
		c, _ := row.Get("count")
		if c.Kind != value.KindInt {
			t.Fatalf("row %d: count kind = %v", i, c.Kind)
		}
		if c.I < 0 || c.I > 30 {
			t.Errorf("row %d: count %v outside [0, 30]", i, c.I)
		}
	}
	// With volatility 20 inside a width-80 corridor over 50 rows the clamps
	// are exercised; if this ever flakes the volatility is too low.
	if !hitLo && !hitHi {
		t.Error("walk never touched its clamps; clamp path untested")
	}
}

func TestChainEdgesAndAbsorbingStates(t *testing.T) {
	rows := generateStateful(t)
	allowed := map[string]map[string]bool{
		"pending": {"pending": false, "shipped": true, "cancelled": true},
		"shipped": {"delivered": true},
		// Absorbing states may only self-loop.
		"delivered": {"delivered": true},
		"cancelled": {"cancelled": true},
	}
	prev := ""
	for i, row := range rows {
		v, _ := row.Get("status")
		if i == 0 {
			if v.S != "pending" {
				t.Fatalf("row 0: status = %q, want start state pending", v.S)
			}
		} else if !allowed[prev][v.S] {
			t.Errorf("row %d: illegal transition %s>%s", i, prev, v.S)
		}
		prev = v.S
	}
	if prev != "delivered" && prev != "cancelled" {
		t.Errorf("final state %q not absorbing after 50 rows", prev)
	}
}

func TestChainDefaultStartIsFirstVariant(t *testing.T) {
	schema := `
domain: chain_default
seed: 1
volume:
  Job: 5
entities:
  Job:
    state: enum(queued, running, done) @chain("queued>running:1, running>done:1")
`
	doc, err := parser.New().Parse(strings.NewReader(schema), "chain.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := ds.Entities.Get("Job")
	v, _ := rows[0].Get("state")
	if v.S != "queued" {
		t.Errorf("row 0 state = %q, want first declared variant queued", v.S)
	}
	// Deterministic ladder: queued → running → done, then absorbed.
	want := []string{"queued", "running", "done", "done", "done"}
	for i, row := range rows {
		v, _ := row.Get("state")
		if v.S != want[i] {
			t.Errorf("row %d state = %q, want %q", i, v.S, want[i])
		}
	}
}

func TestStatefulDeterminism(t *testing.T) {
	gen := func() *value.Dataset {
		doc := loadFixture(t, "../testdata/fixtures/time_series.yaml")
		ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return ds
	}
	if !reflect.DeepEqual(gen(), gen()) {
		t.Error("two generations from the same schema+seed differ")
	}
}

func TestValidateStateful(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		entity  string
		field   string
		message string
	}{
		{
			name: "chain bad state name",
			schema: `
entities:
  Order:
    status: enum(pending, shipped) @chain("pending>refunded:1")
`,
			entity: "Order", field: "status", message: "not a variant",
		},
		{
			name: "chain bad start state",
			schema: `
entities:
  Order:
    status: enum(pending, shipped) @chain("pending>shipped:1", start=lost)
`,
			entity: "Order", field: "status", message: "not a variant",
		},
		{
			name: "chain on non-enum field",
			schema: `
entities:
  Order:
    status: string @chain("a>b:1")
`,
			entity: "Order", field: "status", message: "requires an enum",
		},
		{
			name: "series on non-time field",
			schema: `
entities:
  Metric:
    ts: int @series(start=2026-01-01, interval=1d)
`,
			entity: "Metric", field: "ts", message: "requires a date or datetime",
		},
		{
			name: "walk on non-numeric field",
			schema: `
entities:
  Metric:
    value: string @walk(start=1)
`,
			entity: "Metric", field: "value", message: "requires an int, float or decimal",
		},
		{
			name: "stateful on coherence member",
			schema: `
entities:
  Office:
    _coherence:
      location: [city, opened]
    city: string
    opened: date @series(start=2026-01-01, interval=1d)
`,
			entity: "Office", field: "opened", message: "coherence-group member",
		},
		{
			name: "stateful on reference field",
			schema: `
entities:
  User:
    id: uuid @primary
  Order:
    user_id: ->User @chain("a>b:1")
`,
			entity: "Order", field: "user_id", message: "reference field",
		},
		{
			name: "stateful on compound type",
			schema: `
entities:
  Metric:
    values: "[int] @walk(start=0)"
`,
			entity: "Metric", field: "values", message: "compound type",
		},
		{
			name: "multiple stateful decorators",
			schema: `
entities:
  Metric:
    v: int @walk(start=0) @chain("a>b:1")
`,
			entity: "Metric", field: "v", message: "multiple stateful decorators",
		},
		{
			name: "config error surfaces in validation",
			schema: `
entities:
  Metric:
    ts: datetime @series(interval=1m)
`,
			entity: "Metric", field: "ts", message: "missing required start",
		},
		{
			name: "stateful on reusable type field",
			schema: `
types:
  Money:
    amount: float @walk(start=0)
entities:
  Order:
    total: Money
`,
			entity: "Money", field: "amount", message: "reusable type",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := parser.New().Parse(strings.NewReader("domain: v\n"+tc.schema), "v.yaml")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = ValidateStateful(doc)
			if err == nil {
				t.Fatal("want validation error, got nil")
			}
			var de *errors.Error
			if !stderrors.As(err, &de) {
				t.Fatalf("error type = %T", err)
			}
			if de.Kind != errors.KindValidation {
				t.Errorf("kind = %v, want KindValidation", de.Kind)
			}
			if de.Entity != tc.entity || de.Field != tc.field {
				t.Errorf("location = %s.%s, want %s.%s", de.Entity, de.Field, tc.entity, tc.field)
			}
			if !strings.Contains(de.Message, tc.message) {
				t.Errorf("message %q does not contain %q", de.Message, tc.message)
			}
		})
	}
}

func TestValidateStatefulAcceptsCleanSchema(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/time_series.yaml")
	if err := ValidateStateful(doc); err != nil {
		t.Fatalf("ValidateStateful: %v", err)
	}
}
