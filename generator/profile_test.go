package generator

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

func dec(name string, args ...model.DecoratorArg) model.Decorator {
	return model.Decorator{Name: name, Args: args}
}

func rangeArg(from, to string) model.DecoratorArg {
	return model.DecoratorArg{Kind: model.ArgRange, From: from, To: to}
}

func identArg(s string) model.DecoratorArg {
	return model.DecoratorArg{Kind: model.ArgIdent, Ident: s}
}

func strField(name string, decs ...model.Decorator) *model.Field {
	return &model.Field{Name: name, Type: model.Primitive{Kind: model.PrimString}, Decorators: decs}
}

func TestResolveProfile(t *testing.T) {
	for in, want := range map[string]string{
		"":          ProfileRealistic,
		"realistic": ProfileRealistic,
		"edge":      ProfileEdge,
		"hostile":   ProfileHostile,
	} {
		got, err := resolveProfile(ports.GenerateOptions{Profile: in})
		if err != nil {
			t.Fatalf("resolveProfile(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("resolveProfile(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"EDGE", "fuzz", " hostile"} {
		if _, err := resolveProfile(ports.GenerateOptions{Profile: bad}); err == nil {
			t.Fatalf("resolveProfile(%q) unexpectedly succeeded", bad)
		}
	}
}

func TestProfileEligibilityMatrix(t *testing.T) {
	cases := []struct {
		name string
		f    *model.Field
		want bool
	}{
		{"plain string", strField("note"), true},
		{"semantic", &model.Field{Name: "n", Type: model.Semantic{Namespace: "person", Tag: "full"}}, true},
		{"primary", strField("id", dec("primary")), false},
		{"auto", strField("ts", dec("auto")), false},
		{"unique", strField("key", dec("unique")), false},
		{"pattern", strField("sku", dec("pattern")), false},
		{"derived", strField("d", dec("derived")), false},
		{"compute decorator", strField("c", dec("compute")), false},
		{"compute branches", &model.Field{Name: "c", Type: model.Primitive{Kind: model.PrimString}, Compute: []model.ComputeBranch{{Value: "1"}}}, false},
		{"default_chain", &model.Field{Name: "dc", Type: model.Primitive{Kind: model.PrimString}, DefaultChain: &model.DefaultChainSpec{Sources: []string{"a"}}}, false},
		{"reference", &model.Field{Name: "owner", Type: model.Reference{Target: "User"}}, false},
		{"list of references", &model.Field{Name: "tags", Type: model.List{Element: model.Reference{Target: "Tag"}}}, false},
		{"polymorphic source", &model.Field{Name: "p", Type: model.Union{Variants: []model.TypeExpr{model.Reference{Target: "A"}, model.Reference{Target: "B"}}}, Discriminator: "p_type"}, false},
		{"discriminator", &model.Field{Name: "p_type", Type: model.Primitive{Kind: model.PrimString}, DiscriminatorFor: "p"}, false},
		{"profile opt-out ident", strField("m", dec("profile", identArg("realistic"))), false},
		{"profile opt-out literal", strField("m", dec("profile", model.DecoratorArg{Kind: model.ArgLiteral, Literal: "realistic"})), false},
		{"bare profile decorator", strField("m", dec("profile")), false},
		{"profile other arg", strField("m", dec("profile", identArg("edge"))), true},
		{"nil field", nil, false},
	}
	ent := model.NewEntity("E")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := profileEligible(ent, tc.f); got != tc.want {
				t.Fatalf("profileEligible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProfileEligibilityCoherenceMember(t *testing.T) {
	ent := model.NewEntity("E")
	ent.Coherence.Set("identity", []string{"first_name", "email"})
	if profileEligible(ent, strField("first_name")) {
		t.Fatal("coherence member should not be eligible")
	}
	if !profileEligible(ent, strField("note")) {
		t.Fatal("non-member should stay eligible")
	}
}

func TestProfileTableStrings(t *testing.T) {
	edge := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimString}, nil)
	if len(edge) != len(edgeStrings) {
		t.Fatalf("edge table has %d entries, want %d", len(edge), len(edgeStrings))
	}
	hostile := profileTable(ProfileHostile, model.Primitive{Kind: model.PrimString}, nil)
	if len(hostile) != len(hostileStrings) {
		t.Fatalf("hostile table has %d entries, want %d", len(hostile), len(hostileStrings))
	}
	if len(hostile) <= len(edge) {
		t.Fatal("hostile table must be a strict superset of edge")
	}
	var sawEmpty, saw255, saw4k bool
	for _, v := range hostile {
		if v.Kind != value.KindString {
			t.Fatalf("string table holds non-string %v", v.Kind)
		}
		if strings.ContainsRune(v.S, 0) {
			t.Fatalf("hostile table entry %q contains a NUL byte", v.S)
		}
		switch {
		case v.S == "":
			sawEmpty = true
		case v.S == strings.Repeat("a", 255):
			saw255 = true
		case v.S == strings.Repeat("A", 4096):
			saw4k = true
		}
	}
	if !sawEmpty || !saw255 || !saw4k {
		t.Fatalf("missing curated entries: empty=%v 255=%v 4k=%v", sawEmpty, saw255, saw4k)
	}
	// Semantic types share the string-like table.
	sem := profileTable(ProfileEdge, model.Semantic{Namespace: "person", Tag: "full"}, nil)
	if !reflect.DeepEqual(sem, edge) {
		t.Fatal("semantic table should equal the string table")
	}
}

func TestProfileTableIntNoRange(t *testing.T) {
	table := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimInt}, nil)
	want := []int64{0, 1, -1, math.MaxInt64, math.MinInt64}
	if len(table) != len(want) {
		t.Fatalf("int table has %d entries, want %d", len(table), len(want))
	}
	for i, w := range want {
		if table[i].Kind != value.KindInt || table[i].I != w {
			t.Fatalf("entry %d = %v, want %d", i, table[i], w)
		}
	}
}

func TestProfileTableIntRangeAware(t *testing.T) {
	decs := []model.Decorator{dec("range", rangeArg("18", "99"))}
	table := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimInt}, decs)
	if len(table) != 2 {
		t.Fatalf("ranged int table has %d entries, want 2 (bounds only): %v", len(table), table)
	}
	if table[0].I != 18 || table[1].I != 99 {
		t.Fatalf("ranged int table = %v, want [18 99]", table)
	}
	// The applyRange clamp must be a no-op for every entry.
	for _, v := range table {
		if got := applyRange(v, decs); !reflect.DeepEqual(got, v) {
			t.Fatalf("applyRange clamped table entry %v to %v", v, got)
		}
	}
	// Small constants survive when in range.
	decs = []model.Decorator{dec("range", rangeArg("-5", "5"))}
	table = profileTable(ProfileEdge, model.Primitive{Kind: model.PrimInt}, decs)
	wantVals := []int64{0, 1, -1, -5, 5}
	if len(table) != len(wantVals) {
		t.Fatalf("ranged int table = %v, want %v", table, wantVals)
	}
	for i, w := range wantVals {
		if table[i].I != w {
			t.Fatalf("entry %d = %d, want %d", i, table[i].I, w)
		}
	}
}

func TestProfileTableFloat(t *testing.T) {
	table := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimFloat}, nil)
	if len(table) != 4 {
		t.Fatalf("float table has %d entries, want 4", len(table))
	}
	if table[0].F != 0 || !math.Signbit(table[1].F) || table[1].F != 0 {
		t.Fatalf("expected 0 then -0.0, got %v %v", table[0].F, table[1].F)
	}
	if table[2].F != math.MaxFloat64 || table[3].F != math.SmallestNonzeroFloat64 {
		t.Fatalf("expected extremes, got %v %v", table[2].F, table[3].F)
	}

	decs := []model.Decorator{dec("range", rangeArg("0.01", "999.99"))}
	table = profileTable(ProfileEdge, model.Primitive{Kind: model.PrimFloat}, decs)
	if len(table) != 2 || table[0].F != 0.01 || table[1].F != 999.99 {
		t.Fatalf("ranged float table = %v, want [0.01 999.99]", table)
	}
	for _, v := range table {
		if got := applyRange(v, decs); !reflect.DeepEqual(got, v) {
			t.Fatalf("applyRange clamped table entry %v to %v", v, got)
		}
	}
}

func TestProfileTableDecimal(t *testing.T) {
	decs := []model.Decorator{dec("range", rangeArg("0", "10000"))}
	table := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimDecimal, Params: []int{10, 2}}, decs)
	if len(table) == 0 {
		t.Fatal("decimal table is empty")
	}
	for _, v := range table {
		if v.Kind != value.KindDecimal {
			t.Fatalf("decimal table holds %v", v.Kind)
		}
		f, _ := v.D.Float64()
		if f < 0 || f > 10000 {
			t.Fatalf("decimal entry %s outside declared range", v.D)
		}
	}
}

func TestProfileTableTime(t *testing.T) {
	table := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimDate}, nil)
	want := []time.Time{
		time.Unix(0, 0).UTC(),
		time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	if len(table) != len(want) {
		t.Fatalf("time table has %d entries, want %d", len(table), len(want))
	}
	for i, w := range want {
		if !table[i].T.Equal(w) {
			t.Fatalf("entry %d = %v, want %v", i, table[i].T, w)
		}
	}

	decs := []model.Decorator{dec("range", rangeArg("2020-01-01", "2025-12-31"))}
	table = profileTable(ProfileEdge, model.Primitive{Kind: model.PrimDatetime}, decs)
	if len(table) != 2 {
		t.Fatalf("ranged time table = %v, want bounds only", table)
	}
	for _, v := range table {
		if got := applyRange(v, decs); !got.T.Equal(v.T) {
			t.Fatalf("applyRange clamped time entry %v to %v", v.T, got.T)
		}
	}
}

func TestProfileTableUUIDBoolEnum(t *testing.T) {
	table := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimUUID}, nil)
	if len(table) != 1 || table[0].Kind != value.KindUUID || table[0].U != uuid.Nil {
		t.Fatalf("uuid table = %v, want single all-zeros entry", table)
	}
	if got := profileTable(ProfileEdge, model.Primitive{Kind: model.PrimBool}, nil); len(got) != 0 {
		t.Fatalf("bool table should be empty, got %v", got)
	}
	if got := profileTable(ProfileEdge, model.EnumInline{Values: []string{"a", "b"}}, nil); len(got) != 0 {
		t.Fatalf("enum table should be empty, got %v", got)
	}
}

func TestProfileTableCompound(t *testing.T) {
	list := profileTable(ProfileEdge, model.List{Element: model.Primitive{Kind: model.PrimString}}, nil)
	if len(list) != len(edgeStrings)+1 {
		t.Fatalf("list table has %d entries, want %d", len(list), len(edgeStrings)+1)
	}
	if list[0].Kind != value.KindList || len(list[0].L) != 0 {
		t.Fatalf("first list entry should be the empty list, got %v", list[0])
	}
	for _, v := range list[1:] {
		if v.Kind != value.KindList || len(v.L) != 1 {
			t.Fatalf("list entry should be single-element, got %v", v)
		}
	}

	nullable := profileTable(ProfileEdge, model.Nullable{Inner: model.Primitive{Kind: model.PrimString}}, nil)
	if len(nullable) != len(edgeStrings)+1 || !nullable[0].IsNull() {
		t.Fatalf("nullable table should be null + inner entries, got %v", nullable)
	}

	if got := profileTable(ProfileEdge, model.Map{Key: model.Primitive{Kind: model.PrimString}, Value: model.Primitive{Kind: model.PrimInt}}, nil); len(got) != 0 {
		t.Fatalf("map table should be empty, got %v", got)
	}
}

func TestApplyProfileSubstitutionDrawDiscipline(t *testing.T) {
	ent := model.NewEntity("E")
	f := strField("note")
	base := value.Str("base")

	// Realistic (and ""): zero draws, value untouched.
	for _, p := range []string{"", ProfileRealistic} {
		rng := &countingRNG{inner: NewRand(1)}
		if got := applyProfileSubstitution(ent, f, base, p, rng); !reflect.DeepEqual(got, base) {
			t.Fatalf("profile %q substituted: %v", p, got)
		}
		if rng.draws != 0 {
			t.Fatalf("profile %q consumed %d draws, want 0", p, rng.draws)
		}
	}

	// Edge: exactly two draws for an eligible field, regardless of outcome.
	for seed := int64(0); seed < 8; seed++ {
		rng := &countingRNG{inner: NewRand(seed)}
		applyProfileSubstitution(ent, f, base, ProfileEdge, rng)
		if rng.draws != 2 {
			t.Fatalf("seed %d: eligible field consumed %d draws, want 2", seed, rng.draws)
		}
	}

	// Ineligible field: zero draws even under hostile.
	rng := &countingRNG{inner: NewRand(1)}
	applyProfileSubstitution(ent, strField("id", dec("primary")), base, ProfileHostile, rng)
	if rng.draws != 0 {
		t.Fatalf("ineligible field consumed %d draws, want 0", rng.draws)
	}

	// Tableless type (bool): zero draws.
	rng = &countingRNG{inner: NewRand(1)}
	applyProfileSubstitution(ent, &model.Field{Name: "b", Type: model.Primitive{Kind: model.PrimBool}}, value.Bool(true), ProfileHostile, rng)
	if rng.draws != 0 {
		t.Fatalf("tableless field consumed %d draws, want 0", rng.draws)
	}
}

// countingRNG wraps a Randomizer and counts Float/IntN draws.
type countingRNG struct {
	inner ports.Randomizer
	draws int
}

func (c *countingRNG) Substream(scope string) ports.Randomizer {
	return &countingRNG{inner: c.inner.Substream(scope)}
}
func (c *countingRNG) Float() float64 { c.draws++; return c.inner.Float() }
func (c *countingRNG) IntN(n int64) int64 {
	c.draws++
	return c.inner.IntN(n)
}
func (c *countingRNG) NormFloat() float64 { c.draws++; return c.inner.NormFloat() }
func (c *countingRNG) ExpFloat() float64  { c.draws++; return c.inner.ExpFloat() }
func (c *countingRNG) Shuffle(n int, swap func(i, j int)) {
	c.inner.Shuffle(n, swap)
}

func TestEngineRealisticProfileMatchesNoProfile(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/profiles.yaml")
	base, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"", ProfileRealistic} {
		got, err := newEngine().Generate(doc, ports.GenerateOptions{Profile: p})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(base, got) {
			t.Fatalf("profile %q output differs from no-profile output", p)
		}
	}
}

func TestEngineProfileDeterministicAndDistinct(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/profiles.yaml")
	gen := func(p string) *value.Dataset {
		t.Helper()
		ds, err := newEngine().Generate(doc, ports.GenerateOptions{Profile: p})
		if err != nil {
			t.Fatal(err)
		}
		return ds
	}
	for _, p := range []string{ProfileEdge, ProfileHostile} {
		if !reflect.DeepEqual(gen(p), gen(p)) {
			t.Fatalf("profile %q is not deterministic", p)
		}
	}
	if reflect.DeepEqual(gen(ProfileRealistic), gen(ProfileEdge)) {
		t.Fatal("edge output should differ from realistic for this fixture")
	}

	if _, err := newEngine().Generate(doc, ports.GenerateOptions{Profile: "bogus"}); err == nil {
		t.Fatal("unknown profile should be rejected")
	}
}

func TestEngineProfileRespectsOptOutAndPinnedFields(t *testing.T) {
	doc := loadFixture(t, "../testdata/fixtures/profiles.yaml")
	base, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hostile, err := newEngine().Generate(doc, ports.GenerateOptions{Profile: ProfileHostile})
	if err != nil {
		t.Fatal(err)
	}
	baseRows, _ := base.Entities.Get("Account")
	hostileRows, _ := hostile.Entities.Get("Account")
	if len(baseRows) == 0 || len(baseRows) != len(hostileRows) {
		t.Fatalf("row counts differ: %d vs %d", len(baseRows), len(hostileRows))
	}

	// Fields generated before the first eligible field share an untouched RNG
	// stream prefix, so they must be byte-identical under any profile:
	// coherence members (owner, contact_email) and the leading @primary id.
	for _, fname := range []string{"id", "owner", "contact_email"} {
		for i := range baseRows {
			b, _ := baseRows[i].Get(fname)
			h, _ := hostileRows[i].Get(fname)
			if !reflect.DeepEqual(b, h) {
				t.Fatalf("ineligible field %q row %d changed under hostile profile: %v vs %v", fname, i, b, h)
			}
		}
	}

	// Later ineligible fields see a shifted stream (eligible fields before
	// them consume two extra draws each) so their values may differ — but
	// they must never receive boundary-table substitutions.
	hostileSet := map[string]struct{}{}
	for _, s := range hostileStrings {
		hostileSet[s] = struct{}{}
	}
	for i, row := range hostileRows {
		for _, fname := range []string{"motto", "api_key"} {
			v, _ := row.Get(fname)
			if v.Kind != value.KindString {
				t.Fatalf("%s row %d: unexpected kind %v", fname, i, v.Kind)
			}
			if _, hit := hostileSet[v.S]; hit {
				t.Fatalf("%s row %d received a boundary substitution: %q", fname, i, v.S)
			}
		}
		status, _ := row.Get("status")
		if status.Kind != value.KindString || (status.S != "active" && status.S != "pending" && status.S != "banned") {
			t.Fatalf("status row %d left enum domain: %v", i, status)
		}
		code, _ := row.Get("referral_code")
		if code.Kind != value.KindString || !strings.HasPrefix(code.S, "RF-") {
			t.Fatalf("referral_code row %d lost its pattern shape: %v", i, code)
		}
	}
}
