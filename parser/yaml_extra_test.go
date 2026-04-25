package parser

import (
	"strings"
	"testing"
)

func TestParseGenerationToolsAndMappingRules(t *testing.T) {
	src := `
domain: rich
tools:
  User:
    enabled: true
    limit: 3
    ratio: 1.5
    labels: [a, b]
    nested:
      none: null
generation:
  seed: 99
  locale: ca-ES
  locales:
    ca-ES: 7
    en-US: 3
  null_strategy: sparse
  id_format: prefixed
  date_format: "2006-01-02"
  currency_format: "EUR"
  llm:
    provider: openai
    endpoint: http://localhost
    model: test
    api_key: secret
    temperature: 0.7
    timeout_secs: 30
    max_tokens: 128
entities:
  User:
    id: int
rules:
  - when: User.id > 0
    assert: User.id < 10
    error: too large
    severity: probability(0.25)
  - cross_row:
      entity: User
`
	doc, err := New().Parse(strings.NewReader(src), "rich.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Generation.Seed == nil || *doc.Generation.Seed != 99 {
		t.Fatalf("seed: %+v", doc.Generation.Seed)
	}
	if doc.Generation.LLM == nil || doc.Generation.LLM.Provider != "openai" || doc.Generation.LLM.MaxTokens == nil {
		t.Fatalf("llm: %+v", doc.Generation.LLM)
	}
	if doc.Tools["User"].Raw["enabled"] != true {
		t.Fatalf("tools: %+v", doc.Tools)
	}
	if len(doc.Rules) != 2 || doc.Rules[0].Probability != 0.25 || doc.Rules[1].CrossRow == "" {
		t.Fatalf("rules: %+v", doc.Rules)
	}
}

func TestParseInvalidShapes(t *testing.T) {
	cases := []string{
		`[]`,
		`domain: bad
volume: []`,
		`domain: bad
enums: []`,
		`domain: bad
types: []`,
		`domain: bad
entities: []`,
		`domain: bad
entities:
  User: []`,
		`domain: bad
entities:
  User:
    _coherence: []`,
		`domain: bad
entities:
  User:
    field:
      label: Missing type`,
		`domain: bad
rules: {}`,
		`domain: bad
tools: []`,
		`domain: bad
generation: []`,
		`domain: bad
generation:
  llm: []`,
	}
	for _, src := range cases {
		t.Run(strings.ReplaceAll(strings.Split(src, "\n")[0], " ", "_"), func(t *testing.T) {
			if _, err := New().Parse(strings.NewReader(src), "bad.yaml"); err == nil {
				t.Fatalf("expected parse error for:\n%s", src)
			}
		})
	}
}
