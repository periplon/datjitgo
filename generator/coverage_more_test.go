package generator

import (
	"strings"
	"testing"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

func TestExprParserAdditionalErrorAndEscapeBranches(t *testing.T) {
	cases := []string{
		"",
		"user.",
		"fn(1 2)",
		"fn(1",
		"x in 1",
		"@",
		"\"unterminated",
		"1 +",
		"1 2",
	}
	for _, src := range cases {
		if err := ParseExpr(src); err == nil {
			t.Fatalf("%q: expected parse error", src)
		}
	}

	got := mustEval(t, `"a\nb\t\\\"\'\q"`, nil)
	if got.S != "a\nb\t\\\"'q" {
		t.Fatalf("unescape mismatch: %q", got.S)
	}
}

func TestExprFunctionDefaultsAndDateBranches(t *testing.T) {
	data := map[string][]*value.Object{
		"Thing": {
			mkRow(t, "first", value.Int(4), "second", value.Int(99)),
			mkRow(t, "first", value.Float(2.5)),
		},
	}
	cases := map[string]value.Value{
		"sum(Thing)":              value.Float(6.5),
		"count()":                 value.Int(0),
		"avg()":                   value.Float(0),
		"min()":                   value.Null(),
		"max()":                   value.Null(),
		"years_since()":           value.Int(0),
		"days_between()":          value.Int(0),
		`days_between("bad", "")`: value.Int(0),
		"round()":                 value.Float(0),
		"lower()":                 value.Str(""),
		"upper()":                 value.Str(""),
		"slug()":                  value.Str(""),
		"starts_with()":           value.Bool(false),
		"ends_with()":             value.Bool(false),
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

	if got := yearsSince("2000-01-01"); got <= 0 {
		t.Fatalf("yearsSince valid date = %d", got)
	}
	if got := slugify("---"); got != "" {
		t.Fatalf("slugify punctuation = %q", got)
	}
}

func TestDerivedDefaultAndComputeEdgeBranches(t *testing.T) {
	eng := newEngine()
	st := &generationState{generated: map[string][]*value.Object{}}

	entity := model.NewEntity("User")
	entity.Fields.Set("base", &model.Field{Name: "base", Type: model.Primitive{Kind: model.PrimString}})
	entity.Fields.Set("derived_missing_arg", &model.Field{Name: "derived_missing_arg", Decorators: []model.Decorator{{Name: "derived"}}})
	row := mkRow(t, "base", value.Str("Ada"))
	if err := eng.applyDerived(entity, row, st); err != nil {
		t.Fatalf("missing derived arg should be ignored: %v", err)
	}

	entity.Fields.Set("derived_bad_parse", &model.Field{Name: "derived_bad_parse", Decorators: []model.Decorator{{Name: "derived", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: "1 +"}}}}})
	if err := eng.applyDerived(entity, row, st); err == nil {
		t.Fatal("expected derived parse error")
	}

	entity = model.NewEntity("Defaulted")
	entity.Fields.Set("picked", &model.Field{Name: "picked", DefaultChain: &model.DefaultChainSpec{
		Sources:  []string{"missing", "base"},
		When:     "enabled",
		Fallback: `"fallback"`,
	}})
	row = mkRow(t, "base", value.Str("source"), "enabled", value.Bool(true))
	if err := eng.applyDefaultChain(entity, row, st); err != nil {
		t.Fatalf("default chain source: %v", err)
	}
	if got, _ := row.Get("picked"); got.S != "source" {
		t.Fatalf("default source pick = %+v", got)
	}

	row = mkRow(t, "enabled", value.Bool(false))
	if err := eng.applyDefaultChain(entity, row, st); err != nil {
		t.Fatalf("false when should skip: %v", err)
	}
	if row.Has("picked") {
		t.Fatal("false when should not set default field")
	}

	entity.Fields.Set("bad_when", &model.Field{Name: "bad_when", DefaultChain: &model.DefaultChainSpec{When: "1 +", Fallback: `"x"`}})
	if err := eng.applyDefaultChain(entity, mkRow(t), st); err == nil {
		t.Fatal("expected default_chain when parse error")
	}

	entity = model.NewEntity("Computed")
	entity.Fields.Set("label", &model.Field{Name: "label", Compute: []model.ComputeBranch{
		{When: "score > 10", Value: `"high"`},
		{When: "score > 0", Value: `"low"`},
		{Value: `"none"`},
	}})
	row = mkRow(t, "score", value.Int(5))
	if err := eng.applyCompute(entity, row, st); err != nil {
		t.Fatalf("compute branch: %v", err)
	}
	if got, _ := row.Get("label"); got.S != "low" {
		t.Fatalf("compute branch = %+v", got)
	}

	entity.Fields.Set("bad_value", &model.Field{Name: "bad_value", Compute: []model.ComputeBranch{{Value: "1 +"}}})
	if err := eng.applyCompute(entity, mkRow(t), st); err == nil {
		t.Fatal("expected compute value parse error")
	}
}

func TestGenerateByTypeCompositeAndReferenceEdges(t *testing.T) {
	eng := newEngine()
	rng := NewRand(22)
	st := &generationState{
		enumDefs: map[string]model.EnumDef{},
		generated: map[string][]*value.Object{
			"Target": {mkRow(t, "id", value.Int(1)), mkRow(t, "id", value.Int(2)), value.NewObject()},
		},
		seqs: newSeqCounters(),
	}
	weight := 9.0
	st.enumDefs["Tier"] = model.EnumDef{Variants: []model.EnumVariant{{Value: "free"}, {Value: "paid", Weight: &weight}}}

	entity := model.NewEntity("Owner")
	field := &model.Field{Name: "complex"}
	field.Type = model.Tuple{Elements: []model.TypeExpr{
		model.NamedType{Name: "Tier"},
		model.NamedType{Name: "Alias"},
		model.List{Element: model.Primitive{Kind: model.PrimInt}},
		model.Map{Key: model.Primitive{Kind: model.PrimBool}, Value: model.Primitive{Kind: model.PrimString}},
		model.Nullable{Inner: model.Primitive{Kind: model.PrimString}},
		model.Union{Variants: []model.TypeExpr{model.Primitive{Kind: model.PrimInt}}},
		model.Union{},
	}}
	got, err := eng.generateByType(entity, field, field.Type, value.NewObject(), st, rng)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != value.KindList || len(got.L) != 7 {
		t.Fatalf("tuple result = %+v", got)
	}

	manyField := &model.Field{Name: "refs", Decorators: []model.Decorator{{Name: "count", Args: []model.DecoratorArg{{Kind: model.ArgRange, From: "5", To: "9"}}}}}
	refs := eng.generateReference(entity, manyField, model.Reference{Target: "Target", Many: true}, st, rng)
	if refs.Kind != value.KindList || len(refs.L) != 3 {
		t.Fatalf("many refs should clamp to rows: %+v", refs)
	}
	sawNull := false
	for _, ref := range refs.L {
		if ref.Kind == value.KindNull {
			sawNull = true
		}
	}
	if !sawNull {
		t.Fatalf("empty target row should contribute a null first field: %+v", refs)
	}

	emptyRefs := eng.generateReference(entity, manyField, model.Reference{Target: "Missing", ManyToMany: true}, st, rng)
	if emptyRefs.Kind != value.KindList || len(emptyRefs.L) != 0 {
		t.Fatalf("missing many-to-many target = %+v", emptyRefs)
	}
	if single := eng.generateReference(entity, &model.Field{Name: "self"}, model.Reference{Target: "self"}, &generationState{generated: map[string][]*value.Object{}}, rng); single.Kind != value.KindNull {
		t.Fatalf("missing self ref = %+v", single)
	}
	if first := firstField(value.NewObject()); first.Kind != value.KindNull {
		t.Fatalf("empty firstField = %+v", first)
	}
}

func TestDistributionParsingAndSamplingEdges(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []model.DecoratorArg
		kind distKind
	}{
		{"nil", nil, distUniform},
		{"lognormal", []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "lognormal"}}, distLogNormal},
		{"exponential", []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "exponential"}, {Kind: model.ArgKV, Key: "lambda", Value: "bad"}}, distExponential},
		{"geometric", []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "geometric"}, {Kind: model.ArgKV, Key: "p", Value: "1.2"}}, distGeometric},
		{"zipf", []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "zipf"}, {Kind: model.ArgKV, Key: "N", Value: "1"}}, distZipf},
		{"bimodal", []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "bimodal"}, {Kind: model.ArgKV, Key: "peaks", Value: "10"}, {Kind: model.ArgLiteral, Literal: int64(30)}}, distBimodal},
		{"bad-bimodal", []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "bimodal"}}, distNormal},
		{"weighted", []model.DecoratorArg{{Kind: model.ArgIdent, Ident: "weighted"}, {Kind: model.ArgKV, Key: "a", Value: "2"}, {Kind: model.ArgKV, Key: "b", Value: "bad"}}, distWeighted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var d *model.Decorator
			if tc.name != "nil" {
				d = &model.Decorator{Name: "dist", Args: tc.args}
			}
			if got := parseDistDecorator(d); got.Kind != tc.kind {
				t.Fatalf("kind = %v want %v, spec=%+v", got.Kind, tc.kind, got)
			}
		})
	}

	rng := NewRand(33)
	for _, spec := range []distSpec{
		{Kind: distExponential, Lambda: 0},
		{Kind: distGeometric, P: 0},
		{Kind: distZipf, S: 0, ZipfN: 1},
		{Kind: distCategorical},
		{Kind: distWeighted},
	} {
		v := sampleFloat(rng, spec, 10, 20, true)
		if v < 0 {
			t.Fatalf("sampleFloat returned negative for %+v: %v", spec, v)
		}
	}
	if got := sampleEnumIndex(rng, nil); got != 0 {
		t.Fatalf("empty enum index = %d", got)
	}
	if got := sampleEnumIndex(rng, []float64{0, -1, 0}); got < 0 || got > 2 {
		t.Fatalf("non-positive enum index = %d", got)
	}
}

func TestCoherenceDeriveAndDecoratorSourceEdges(t *testing.T) {
	eng := New(nil)
	rng := NewRand(44)
	if got := decoratorIdentSources([]model.DecoratorArg{
		{Kind: model.ArgIdent, Ident: "name"},
		{Kind: model.ArgLiteral, Literal: "company"},
		{Kind: model.ArgLiteral, Literal: int64(3)},
	}); len(got) != 2 || got[0] != "name" || got[1] != "company" {
		t.Fatalf("decoratorIdentSources = %v", got)
	}

	for _, tc := range []struct {
		target string
		source string
		want   string
	}{
		{"email", "Solo", "solo@example.com"},
		{"username", "", "user"},
		{"timezone", "New York", "America/New_York"},
		{"phone", "Ada Lovelace", "+1-555-"},
		{"notes", "Ada Lovelace", "Ada Lovelace"},
	} {
		got, err := eng.deriveFromSource(tc.target, tc.source, rng)
		if err != nil {
			t.Fatalf("%s: %v", tc.target, err)
		}
		if !strings.Contains(got.S, tc.want) {
			t.Fatalf("%s from %q = %q, want contains %q", tc.target, tc.source, got.S, tc.want)
		}
	}

	entity := model.NewEntity("User")
	entity.Fields.Set("name", &model.Field{Name: "name", Type: model.Primitive{Kind: model.PrimString}})
	entity.Fields.Set("email", &model.Field{Name: "email", Decorators: []model.Decorator{{Name: "from"}}})
	row := mkRow(t, "name", value.Str("Ada"))
	if err := eng.applyFromDerivations(entity, row, rng, map[string]struct{}{"email": {}}); err != nil {
		t.Fatal(err)
	}
	if row.Has("email") {
		t.Fatal("coherent field should be skipped by @from")
	}
	if err := eng.applyFromDerivations(entity, row, rng, nil); err != nil {
		t.Fatal(err)
	}
	if row.Has("email") {
		t.Fatal("@from without sources should be ignored")
	}
}

func TestSemanticFallbackAndErrorBranches(t *testing.T) {
	eng := New(nil)
	rng := NewRand(55)
	errorTags := []model.Semantic{
		{Namespace: "person", Tag: "full"},
		{Namespace: "person", Tag: "first"},
		{Namespace: "person", Tag: "last"},
	}
	for _, tag := range errorTags {
		if _, err := eng.generateSemantic(tag, rng); err == nil {
			t.Fatalf("%+v: expected missing corpus error", tag)
		}
	}

	for _, tag := range []model.Semantic{
		{Namespace: "email"},
		{Namespace: "phone"},
		{Namespace: "url", Tag: "avatar"},
		{Namespace: "address", Tag: "country"},
		{Namespace: "currency", Tag: "eur"},
		{Namespace: "text", Tag: "word"},
		{Namespace: "text", Tag: "sentence"},
		{Namespace: "text", Tag: "paragraph"},
		{Namespace: "product", Tag: "title"},
		{Namespace: "product", Tag: "description"},
		{Namespace: "company", Tag: "name"},
		{Namespace: "company", Tag: "industry"},
		{Namespace: "company", Tag: "catch_phrase"},
		{Namespace: "job", Tag: "title"},
		{Namespace: "job", Tag: "department"},
		{Namespace: "color", Tag: "name"},
		{Namespace: "file", Tag: "mime"},
	} {
		got, err := eng.generateSemantic(tag, rng)
		if err != nil {
			t.Fatalf("%+v: %v", tag, err)
		}
		if strings.TrimSpace(valueDisplay(got)) == "" {
			t.Fatalf("%+v: empty fallback", tag)
		}
	}

	if _, err := eng.sampleCorpusString(rng, "missing"); err == nil {
		t.Fatal("expected missing corpus error")
	}
}
