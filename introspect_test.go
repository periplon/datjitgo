package datjit_test

import (
	"fmt"
	"strings"
	"testing"

	datjit "github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/model"
)

// parseDoc parses src into a *model.Document via a default Service, failing the
// test on any error.
func parseDoc(t *testing.T, src string) *model.Document {
	t.Helper()
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(src), "test.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

const introspectSchema = `domain: shop
version: 1.0.0
generation:
  locale: en-US
volume:
  User: 10
  Order: 20..40
enums:
  Status:
    - new
    - paid
    - shipped
entities:
  User:
    id: uuid @primary
    email: email @unique
    age: int @range(18..65)
  Order:
    id: uuid @primary
    owner: ->User
    status: Status
rules:
  - User.age >= 18
`

func TestSchemaSummary(t *testing.T) {
	doc := parseDoc(t, introspectSchema)
	sum := datjit.NewDefault().SchemaSummary(doc)

	if sum.Domain != "shop" || sum.Version != "1.0.0" || sum.Locale != "en-US" {
		t.Fatalf("header mismatch: %+v", sum)
	}

	// Entities in document order.
	if len(sum.Entities) != 2 || sum.Entities[0].Name != "User" || sum.Entities[1].Name != "Order" {
		t.Fatalf("entities/order mismatch: %+v", sum.Entities)
	}
	// Field rendering incl. decorators.
	email := sum.Entities[0].Fields[1]
	if email.Name != "email" || email.Type != "email" || len(email.Decorators) != 1 || email.Decorators[0] != "@unique" {
		t.Fatalf("email field mismatch: %+v", email)
	}
	owner := sum.Entities[1].Fields[1]
	if owner.Type != "->User" {
		t.Fatalf("owner type = %q, want ->User", owner.Type)
	}

	// Enums sorted by name.
	if len(sum.Enums) != 1 || sum.Enums[0].Name != "Status" {
		t.Fatalf("enums mismatch: %+v", sum.Enums)
	}
	if got := sum.Enums[0].Variants; len(got) != 3 || got[0] != "new" {
		t.Fatalf("enum variants mismatch: %+v", got)
	}

	// Volumes sorted by entity name.
	if len(sum.Volumes) != 2 || sum.Volumes[0].Entity != "Order" || sum.Volumes[0].Spec != "20..40" {
		t.Fatalf("volumes mismatch: %+v", sum.Volumes)
	}
	if sum.Volumes[1].Entity != "User" || sum.Volumes[1].Spec != "10" {
		t.Fatalf("user volume mismatch: %+v", sum.Volumes[1])
	}

	// Rules carry a severity tag.
	if len(sum.Rules) != 1 || !strings.HasSuffix(sum.Rules[0], "@strict") {
		t.Fatalf("rules mismatch: %+v", sum.Rules)
	}
}

func TestSchemaSummaryIncludesPolymorphicDiscriminator(t *testing.T) {
	doc := parseDoc(t, `domain: poly
entities:
  User:
    id: uuid @primary
  Org:
    id: uuid @primary
  Comment:
    id: uuid @primary
    author: "->User | ->Org"
`)
	sum := datjit.NewDefault().SchemaSummary(doc)
	var comment model.SchemaEntitySummary
	for _, e := range sum.Entities {
		if e.Name == "Comment" {
			comment = e
		}
	}
	names := map[string]string{}
	for _, f := range comment.Fields {
		names[f.Name] = f.Type
	}
	if names["author"] != "->User | ->Org" {
		t.Fatalf("author type = %q", names["author"])
	}
	if _, ok := names["author_type"]; !ok {
		t.Fatalf("expected synthetic author_type discriminator field, got %+v", names)
	}
}

func TestDependencyGraph(t *testing.T) {
	doc := parseDoc(t, `domain: g
entities:
  User:
    id: uuid @primary
    tags: <->Tag
    manager: ->self?
  Tag:
    id: uuid @primary
  Order:
    id: uuid @primary
    owner: ->User
    payer: "->User | ->Org"
  Org:
    id: uuid @primary
`)
	g := datjit.NewDefault().DependencyGraph(doc)

	if len(g.Nodes) != 4 || g.Nodes[0] != "User" {
		t.Fatalf("nodes mismatch: %+v", g.Nodes)
	}

	type key struct{ from, to, field, kind string }
	got := map[key]bool{}
	for _, e := range g.Edges {
		got[key{e.From, e.To, e.Field, e.Kind}] = true
	}
	wants := []key{
		{"User", "Tag", "tags", "many-to-many"},
		{"User", "User", "manager", "self"},
		{"Order", "User", "owner", "reference"},
		{"Order", "User", "payer", "polymorphic"},
		{"Order", "Org", "payer", "polymorphic"},
	}
	for _, w := range wants {
		if !got[w] {
			t.Fatalf("missing edge %+v in %+v", w, g.Edges)
		}
	}
	if len(g.Cycles) != 0 {
		t.Fatalf("expected no cycles, got %+v", g.Cycles)
	}
}

func TestDependencyGraphCycle(t *testing.T) {
	doc := parseDoc(t, `domain: c
entities:
  A:
    id: uuid @primary
    b: ->B
  B:
    id: uuid @primary
    a: ->A
`)
	g := datjit.NewDefault().DependencyGraph(doc)
	if len(g.Cycles) != 1 {
		t.Fatalf("expected one cycle, got %+v", g.Cycles)
	}
	cyc := g.Cycles[0]
	if len(cyc) < 3 || cyc[0] != cyc[len(cyc)-1] {
		t.Fatalf("malformed cycle path %+v", cyc)
	}
}

func TestSchemaSummaryDeterministic(t *testing.T) {
	doc := parseDoc(t, introspectSchema)
	svc := datjit.NewDefault()
	a := fmt.Sprintf("%+v", svc.SchemaSummary(doc))
	b := fmt.Sprintf("%+v", svc.SchemaSummary(doc))
	if a != b {
		t.Fatalf("summary not deterministic:\n%s\n%s", a, b)
	}
}

func TestDiffSchemaSummaries(t *testing.T) {
	base := datjit.NewDefault().SchemaSummary(parseDoc(t, introspectSchema))

	cases := []struct {
		name     string
		mutate   func(*model.SchemaSummary)
		wantKind string
		breaking bool
	}{
		{
			"entity-removed",
			func(s *model.SchemaSummary) { s.Entities = s.Entities[:1] },
			"entity-removed", true,
		},
		{
			"field-type-changed",
			func(s *model.SchemaSummary) { s.Entities[0].Fields[2].Type = "string" },
			"field-type-changed", true,
		},
		{
			"field-added",
			func(s *model.SchemaSummary) {
				s.Entities[0].Fields = append(s.Entities[0].Fields, model.FieldSummary{Name: "nick", Type: "string"})
			},
			"field-added", false,
		},
		{
			"decorators-changed",
			func(s *model.SchemaSummary) { s.Entities[0].Fields[1].Decorators = nil },
			"field-decorators-changed", false,
		},
		{
			"enum-variant-removed",
			func(s *model.SchemaSummary) { s.Enums[0].Variants = s.Enums[0].Variants[:2] },
			"enum-variants-changed", true,
		},
		{
			"enum-variant-added",
			func(s *model.SchemaSummary) {
				s.Enums[0].Variants = append(s.Enums[0].Variants, "returned")
			},
			"enum-variants-changed", false,
		},
		{
			"volume-changed",
			func(s *model.SchemaSummary) { s.Volumes[1].Spec = "99" },
			"volume-changed", false,
		},
		{
			"domain-changed",
			func(s *model.SchemaSummary) { s.Domain = "other" },
			"domain-changed", true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			modified := cloneSummary(base)
			c.mutate(modified)
			diff := datjit.DiffSchemaSummaries(base, modified)
			var found *model.SchemaChange
			for i := range diff.Changes {
				if diff.Changes[i].Kind == c.wantKind {
					found = &diff.Changes[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("expected a %q change, got %+v", c.wantKind, diff.Changes)
			}
			if found.Breaking != c.breaking {
				t.Fatalf("%s breaking = %v, want %v", c.wantKind, found.Breaking, c.breaking)
			}
			if diff.Breaking() != c.breaking {
				t.Fatalf("diff.Breaking() = %v, want %v", diff.Breaking(), c.breaking)
			}
		})
	}
}

func TestDiffSchemaSummariesEmpty(t *testing.T) {
	base := datjit.NewDefault().SchemaSummary(parseDoc(t, introspectSchema))
	diff := datjit.DiffSchemaSummaries(base, cloneSummary(base))
	if !diff.Empty() {
		t.Fatalf("expected empty diff, got %+v", diff.Changes)
	}
	if datjit.DiffSchemaSummaries(nil, nil) == nil {
		t.Fatal("nil/nil diff should be non-nil")
	}
}

// cloneSummary returns a deep-enough copy of a SchemaSummary for mutation in
// table tests without affecting the original.
func cloneSummary(s *model.SchemaSummary) *model.SchemaSummary {
	out := *s
	out.Entities = make([]model.SchemaEntitySummary, len(s.Entities))
	for i, e := range s.Entities {
		ne := e
		ne.Fields = append([]model.FieldSummary(nil), e.Fields...)
		for j := range ne.Fields {
			ne.Fields[j].Decorators = append([]string(nil), e.Fields[j].Decorators...)
		}
		out.Entities[i] = ne
	}
	out.Enums = make([]model.EnumSummary, len(s.Enums))
	for i, e := range s.Enums {
		ne := e
		ne.Variants = append([]string(nil), e.Variants...)
		out.Enums[i] = ne
	}
	out.Rules = append([]string(nil), s.Rules...)
	out.Volumes = append([]model.VolumeSummary(nil), s.Volumes...)
	return &out
}

// ExampleDiffSchemaSummaries compares two schema signatures and prints the
// breaking changes. Removing a field is breaking; adding one is not.
func ExampleDiffSchemaSummaries() {
	svc := datjit.NewDefault()
	parse := func(src string) *model.SchemaSummary {
		doc, err := svc.Parse(strings.NewReader(src), "schema.yaml")
		if err != nil {
			panic(err)
		}
		return svc.SchemaSummary(doc)
	}

	oldSum := parse(`domain: shop
entities:
  User:
    id: uuid @primary
    email: email
`)
	newSum := parse(`domain: shop
entities:
  User:
    id: uuid @primary
    phone: string
`)

	diff := datjit.DiffSchemaSummaries(oldSum, newSum)
	fmt.Println("breaking:", diff.Breaking())
	for _, c := range diff.Changes {
		fmt.Printf("%s %s.%s\n", c.Kind, c.Entity, c.Field)
	}
	// Output:
	// breaking: true
	// field-removed User.email
	// field-added User.phone
}
