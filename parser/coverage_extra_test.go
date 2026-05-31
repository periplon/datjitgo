package parser

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/periplon/datjitgo/core/model"
)

func TestParseDocumentDefaultsUnknownKeysAndSeedPropagation(t *testing.T) {
	src := `
domain: d
seed: 123
future_extension: ignored
entities:
  E:
    id: uuid
`
	doc, err := New().Parse(strings.NewReader(src), "defaults.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Locale != "en-US" {
		t.Fatalf("locale=%q", doc.Locale)
	}
	if doc.Generation.Seed == nil || *doc.Generation.Seed != 123 {
		t.Fatalf("generation seed: %+v", doc.Generation.Seed)
	}
}

func TestParseEnumVariantObjectsAndErrors(t *testing.T) {
	src := `
domain: d
enums:
  Status:
    - value: active
      label: Active
      weight: 2.5
      description: Can log in
entities:
  E:
    status: Status
`
	doc, err := New().Parse(strings.NewReader(src), "enum.yaml")
	if err != nil {
		t.Fatal(err)
	}
	status, ok := doc.Enums.Get("Status")
	if !ok || len(status.Variants) != 1 {
		t.Fatalf("status enum: %+v ok=%v", status, ok)
	}
	v := status.Variants[0]
	if v.Value != "active" || v.Label != "Active" || v.Description != "Can log in" {
		t.Fatalf("variant metadata: %+v", v)
	}
	if v.Weight == nil || *v.Weight != 2.5 {
		t.Fatalf("variant weight: %+v", v.Weight)
	}

	cases := []string{
		`domain: d
enums:
  Status: [{label: Missing value}]
entities: {E: {id: uuid}}`,
		`domain: d
enums:
  Status:
    - [bad]
entities: {E: {id: uuid}}`,
		`domain: d
enums:
  Status: {active: true}
entities: {E: {id: uuid}}`,
	}
	for _, tc := range cases {
		if _, err := New().Parse(strings.NewReader(tc), "enum-bad.yaml"); err == nil {
			t.Fatalf("expected enum parse error for:\n%s", tc)
		}
	}
}

func TestParseYamlEdgeErrors(t *testing.T) {
	cases := []string{
		`domain: d
seed: nope`,
		`domain: d
volume:
  E: "x..10"
entities: {E: {id: uuid}}`,
		`domain: d
volume:
  E: "1..x"
entities: {E: {id: uuid}}`,
		`domain: d
volume:
  E: [1, 2]
entities: {E: {id: uuid}}`,
		`domain: d
types:
  Address: string
entities: {E: {id: uuid}}`,
		`domain: d
entities:
  E:
    tags: [string]`,
		`domain: d
entities:
  E:
    _meta: '@bad('
    id: uuid`,
		`domain: d
entities:
  E:
    _coherence:
      group: not-a-list
    id: uuid`,
		`domain: d
entities:
  E:
    field:
      type: string
      default_chain: []`,
		`domain: d
entities:
  E:
    field:
      type: string
      default_chain: source`,
		`domain: d
entities:
  E:
    field:
      type: string
      compute: []`,
		`domain: d
entities:
  E:
    field:
      type: string
      compute:
        - scalar`,
		`domain: d
entities:
  E:
    field:
      type: string
      compute:
        - when: x`,
		`domain: d
rules:
  - "E.id > 0 @probability("`,
		`domain: d
rules:
  - "E.id > 0 @probability(nope)"`,
		`domain: d
rules:
  - [bad]`,
		`domain: d
tools:
  E: scalar
entities: {E: {id: uuid}}`,
	}
	for _, src := range cases {
		t.Run(strings.ReplaceAll(strings.Split(strings.TrimSpace(src), "\n")[0], " ", "_"), func(t *testing.T) {
			if _, err := New().Parse(strings.NewReader(src), "bad.yaml"); err == nil {
				t.Fatalf("expected parse error for:\n%s", src)
			}
		})
	}
}

func TestParseRulesMappingSeverities(t *testing.T) {
	src := `
domain: d
entities:
  E:
    id: int
rules:
  - assert: E.id > 0
    severity: warn
    error: warn message
  - assert: E.id > 1
    probability: 0.75
  - assert: E.id > 2
    severity: probability(0.5)
  - "E.id > 3 @strict"
`
	doc, err := New().Parse(strings.NewReader(src), "rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Rules[0].Severity != model.RuleWarn || doc.Rules[0].ErrorMessage != "warn message" {
		t.Fatalf("warn rule: %+v", doc.Rules[0])
	}
	if doc.Rules[1].Severity != model.RuleProbabilistic || doc.Rules[1].Probability != 0.75 {
		t.Fatalf("probability field rule: %+v", doc.Rules[1])
	}
	if doc.Rules[2].Severity != model.RuleProbabilistic || doc.Rules[2].Probability != 0.5 {
		t.Fatalf("probability severity rule: %+v", doc.Rules[2])
	}
	if doc.Rules[3].Severity != model.RuleStrict || strings.Contains(doc.Rules[3].Expr, "@strict") {
		t.Fatalf("strict rule: %+v", doc.Rules[3])
	}
}

func TestNodeToAnyScalarsDocumentsSequencesAndAliases(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(`
base: &base
  i: 42
  f: 1.25
  b: true
  n: null
  s: text
alias: *base
seq: [1, false, null]
badint: !!int nope
`), &root); err != nil {
		t.Fatal(err)
	}
	got, err := nodeToAny(&root)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}
	base, ok := m["base"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", m["base"])
	}
	alias, ok := m["alias"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", m["alias"])
	}
	if base["i"] != int64(42) || base["f"] != 1.25 || base["b"] != true || base["n"] != nil || base["s"] != "text" {
		t.Fatalf("base: %#v", base)
	}
	if alias["s"] != "text" {
		t.Fatalf("alias: %#v", alias)
	}
	seq, ok := m["seq"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", m["seq"])
	}
	if seq[0] != int64(1) || seq[1] != false || seq[2] != nil {
		t.Fatalf("seq: %#v", seq)
	}
	if m["badint"] != "nope" {
		t.Fatalf("badint fallback: %#v", m["badint"])
	}

	emptyDoc := &yaml.Node{Kind: yaml.DocumentNode}
	if v, err := nodeToAny(emptyDoc); err != nil || v != nil {
		t.Fatalf("empty document: %#v err=%v", v, err)
	}
	if v, err := nodeToAny(nil); err != nil || v != nil {
		t.Fatalf("nil node: %#v err=%v", v, err)
	}
}
