package parser

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
	derrs "github.com/jmcarbo/datjitgo/core/errors"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestParseMinimalFixture(t *testing.T) {
	f := openFixture(t, "minimal.yaml")
	doc, err := New().Parse(f, "minimal.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Domain != "test_minimal" {
		t.Fatalf("domain=%q", doc.Domain)
	}
	if doc.Version != "0.1.0" {
		t.Fatalf("version=%q", doc.Version)
	}
	if doc.Seed == nil || *doc.Seed != 42 {
		t.Fatalf("seed=%v", doc.Seed)
	}
	if doc.Entities.Len() != 1 {
		t.Fatalf("entities=%d", doc.Entities.Len())
	}
	u, ok := doc.Entities.Get("User")
	if !ok {
		t.Fatal("missing User")
	}
	if u.Fields.Len() != 5 {
		t.Fatalf("fields=%d", u.Fields.Len())
	}
	idField, _ := u.Fields.Get("id")
	if !model.HasDecorator(idField.Decorators, "primary") {
		t.Fatalf("id missing @primary: %+v", idField.Decorators)
	}
	if v, ok := doc.Volume["User"]; !ok || v.Exact != 10 {
		t.Fatalf("volume: %+v", doc.Volume)
	}
}

func TestParseEntityOrderPreserved(t *testing.T) {
	f := openFixture(t, "project_management.yaml")
	doc, err := New().Parse(f, "project_management.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Organization", "User", "Project", "Task"}
	got := doc.Entities.Keys()
	if len(got) != len(want) {
		t.Fatalf("len=%d", len(got))
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("entity[%d]=%q want %q", i, got[i], w)
		}
	}
}

func TestParseEntityMetaDecorators(t *testing.T) {
	f := openFixture(t, "entity_meta.yaml")
	doc, err := New().Parse(f, "entity_meta.yaml")
	if err != nil {
		t.Fatal(err)
	}
	post, _ := doc.Entities.Get("Post")
	if len(post.Meta) != 2 {
		t.Fatalf("Post.Meta=%+v", post.Meta)
	}
	names := []string{post.Meta[0].Name, post.Meta[1].Name}
	if names[0] != "timestamps" || names[1] != "soft_delete" {
		t.Fatalf("meta names: %+v", names)
	}
}

func TestParseCoherence(t *testing.T) {
	f := openFixture(t, "coherence_groups.yaml")
	doc, err := New().Parse(f, "coherence_groups.yaml")
	if err != nil {
		t.Fatal(err)
	}
	emp, _ := doc.Entities.Get("Employee")
	if emp.Coherence.Len() != 1 {
		t.Fatalf("coherence groups: %+v", emp.Coherence.Keys())
	}
	id, _ := emp.Coherence.Get("identity")
	if len(id) != 4 {
		t.Fatalf("identity members: %+v", id)
	}
}

func TestParseEnumsList(t *testing.T) {
	f := openFixture(t, "enums_and_distributions.yaml")
	doc, err := New().Parse(f, "enums_and_distributions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pri, ok := doc.Enums.Get("Priority")
	if !ok {
		t.Fatal("missing Priority enum")
	}
	if len(pri.Variants) != 4 {
		t.Fatalf("variants: %+v", pri.Variants)
	}
	if pri.Variants[0].Value != "critical" {
		t.Fatalf("first variant: %+v", pri.Variants[0])
	}
}

func TestParseRulesWithModifiers(t *testing.T) {
	f := openFixture(t, "rules.yaml")
	doc, err := New().Parse(f, "rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Rules) != 5 {
		t.Fatalf("rules=%d", len(doc.Rules))
	}
	// @probability(0.8) on 4th rule
	if doc.Rules[3].Severity != model.RuleProbabilistic || doc.Rules[3].Probability != 0.8 {
		t.Fatalf("probabilistic rule: %+v", doc.Rules[3])
	}
	// @warn on 5th rule
	if doc.Rules[4].Severity != model.RuleWarn {
		t.Fatalf("warn rule: %+v", doc.Rules[4])
	}
	// Make sure modifier was stripped from the expression
	for i, r := range doc.Rules {
		if strings.Contains(r.Expr, "@") {
			t.Fatalf("rule[%d] expr still has @: %q", i, r.Expr)
		}
	}
}

func TestParseTypesSection(t *testing.T) {
	f := openFixture(t, "named_types.yaml")
	doc, err := New().Parse(f, "named_types.yaml")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := doc.Types.Get("Address")
	if !ok {
		t.Fatal("missing Address type")
	}
	if addr.Fields.Len() != 5 {
		t.Fatalf("Address fields: %d", addr.Fields.Len())
	}
}

func TestParseVolumeRangeAndInferred(t *testing.T) {
	src := `
domain: d
volume:
  A: 100
  B: "1000..2000"
  C: ~
entities:
  A:
    id: uuid @primary
`
	doc, err := New().Parse(strings.NewReader(src), "inline")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Volume["A"].Exact != 100 {
		t.Fatalf("A: %+v", doc.Volume["A"])
	}
	if doc.Volume["B"].Min != 1000 || doc.Volume["B"].Max != 2000 {
		t.Fatalf("B: %+v", doc.Volume["B"])
	}
	if !doc.Volume["C"].Inferred {
		t.Fatalf("C: %+v", doc.Volume["C"])
	}
}

func TestParseExpandedFieldForm(t *testing.T) {
	src := `
domain: d
entities:
  E:
    id: uuid @primary
    notes:
      type: string
      label: "Notes"
      description: "free form"
`
	doc, err := New().Parse(strings.NewReader(src), "inline")
	if err != nil {
		t.Fatal(err)
	}
	e, _ := doc.Entities.Get("E")
	notes, _ := e.Fields.Get("notes")
	if notes.Label != "Notes" || notes.Description != "free form" {
		t.Fatalf("expanded form: %+v", notes)
	}
	if _, ok := notes.Type.(model.Primitive); !ok {
		t.Fatalf("type: %T", notes.Type)
	}
}

func TestParseDefaultChainAndCompute(t *testing.T) {
	src := `
domain: d
entities:
  E:
    id: uuid @primary
    score:
      type: float
      default_chain:
        - a.b
        - c.d
      when: "x == 1"
      fallback: "0.0"
    tier:
      type: string
      compute:
        - when: "score > 100"
          value: "'high'"
        - else: "'low'"
`
	doc, err := New().Parse(strings.NewReader(src), "inline")
	if err != nil {
		t.Fatal(err)
	}
	e, _ := doc.Entities.Get("E")
	score, _ := e.Fields.Get("score")
	if score.DefaultChain == nil {
		t.Fatal("DefaultChain missing")
	}
	if len(score.DefaultChain.Sources) != 2 {
		t.Fatalf("sources: %+v", score.DefaultChain.Sources)
	}
	if score.DefaultChain.When != "x == 1" || score.DefaultChain.Fallback != "0.0" {
		t.Fatalf("when/fallback: %+v", score.DefaultChain)
	}
	tier, _ := e.Fields.Get("tier")
	if len(tier.Compute) != 2 {
		t.Fatalf("compute branches: %+v", tier.Compute)
	}
	if tier.Compute[0].When != "score > 100" {
		t.Fatalf("branch 0: %+v", tier.Compute[0])
	}
	if tier.Compute[1].When != "" {
		t.Fatalf("else branch should have empty When: %+v", tier.Compute[1])
	}
}

func TestParseGenerationAndTools(t *testing.T) {
	src := `
domain: d
generation:
  seed: 7
  locale: "es-ES"
  null_strategy: "sparse"
  id_format: "uuid"
  llm:
    provider: openai
    endpoint: "http://localhost:1234/v1"
    model: "gpt-4"
    temperature: 0.5
    timeout_secs: 30
entities:
  E:
    id: uuid @primary
tools:
  E:
    list:
      page_size: 25
`
	doc, err := New().Parse(strings.NewReader(src), "inline")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Generation.Seed == nil || *doc.Generation.Seed != 7 {
		t.Fatalf("gen seed: %+v", doc.Generation)
	}
	if doc.Generation.Locale != "es-ES" {
		t.Fatalf("gen locale: %+v", doc.Generation)
	}
	if doc.Generation.NullStrategy != "sparse" || doc.Generation.IDFormat != "uuid" {
		t.Fatalf("gen: %+v", doc.Generation)
	}
	if doc.Generation.LLM == nil || doc.Generation.LLM.Provider != "openai" {
		t.Fatalf("llm: %+v", doc.Generation.LLM)
	}
	if doc.Generation.LLM.Temperature == nil || *doc.Generation.LLM.Temperature != 0.5 {
		t.Fatalf("llm temp: %+v", doc.Generation.LLM)
	}
	if _, ok := doc.Tools["E"]; !ok {
		t.Fatalf("tools: %+v", doc.Tools)
	}
}

func TestParseMissingDomainError(t *testing.T) {
	src := `
entities:
  E:
    id: uuid @primary
`
	_, err := New().Parse(strings.NewReader(src), "inline")
	if err == nil {
		t.Fatal("expected error for missing domain")
	}
	var e *derrs.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *errors.Error, got %T: %v", err, err)
	}
	if e.Kind != derrs.KindParse {
		t.Fatalf("kind=%v", e.Kind)
	}
	if e.Location == nil || e.Location.File != "inline" {
		t.Fatalf("location: %+v", e.Location)
	}
}

func TestParseInvalidYamlError(t *testing.T) {
	src := "domain: d\n  entities: [bad"
	_, err := New().Parse(strings.NewReader(src), "bad.yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	var e *derrs.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *errors.Error: %T %v", err, err)
	}
	if e.Kind != derrs.KindParse {
		t.Fatalf("kind=%v", e.Kind)
	}
}

func TestParseAllFixtures(t *testing.T) {
	pattern := filepath.Join("..", "testdata", "fixtures", "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("no fixtures under %s", pattern)
	}
	for _, m := range matches {
		base := filepath.Base(m)
		if strings.HasPrefix(base, "llm_") {
			continue
		}
		t.Run(base, func(t *testing.T) {
			f, err := os.Open(m)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			doc, err := New().Parse(f, base)
			if err != nil {
				t.Fatalf("parse %s: %v", base, err)
			}
			if doc.Domain == "" {
				t.Fatalf("%s: empty domain", base)
			}
			if doc.Entities.Len() == 0 {
				t.Fatalf("%s: no entities", base)
			}
		})
	}
}
