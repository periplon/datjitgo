package generator

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/parser"
)

// TestBareRuleScopedToEntitiesWithField guards claim 1: a bare (unqualified)
// rule must apply only to entities that declare the referenced field, never to
// unrelated entities. Previously such a rule was checked against every entity;
// an entity lacking the field resolved it to null, failing the @strict rule and
// exhausting the row-retry budget.
func TestBareRuleScopedToEntitiesWithField(t *testing.T) {
	const schema = `domain: test_rule_scope
version: 0.1.0
seed: 5

volume:
  Player: 10
  Team: 10

entities:
  Player:
    id: uuid @primary
    score: int @range(0..100)

  Team:
    id: uuid @primary
    name: company.name

rules:
  - score >= 0 @strict
`
	doc, err := parser.New().Parse(strings.NewReader(schema), "schema.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		// Before the fix this failed: the bare rule `score >= 0` was enforced
		// against Team (which has no score), null >= 0 was falsey, and the
		// @strict retry budget was exhausted.
		t.Fatalf("generate: bare rule wrongly enforced against entity without the field: %v", err)
	}
	teams, _ := ds.Entities.Get("Team")
	if len(teams) != 10 {
		t.Fatalf("expected 10 Team rows, got %d", len(teams))
	}
	players, _ := ds.Entities.Get("Player")
	if len(players) != 10 {
		t.Fatalf("expected 10 Player rows, got %d", len(players))
	}
}

// TestBareRuleStillEnforcedOnMatchingEntity confirms the scoping does not
// silently drop enforcement: a bare rule impossible to satisfy on the entity
// that declares the field must still fail generation.
func TestBareRuleStillEnforcedOnMatchingEntity(t *testing.T) {
	const schema = `domain: test_rule_scope_enforced
version: 0.1.0
seed: 5

volume:
  Player: 5
  Team: 5

entities:
  Player:
    id: uuid @primary
    score: int @range(0..100)

  Team:
    id: uuid @primary
    name: company.name

rules:
  - score > 1000 @strict
`
	doc, err := parser.New().Parse(strings.NewReader(schema), "schema.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = newEngine().Generate(doc, ports.GenerateOptions{})
	if err == nil {
		t.Fatal("expected rule violation: score in 0..100 can never satisfy score > 1000")
	}
	var de *errors.Error
	if !stderrors.As(err, &de) || de.Kind != errors.KindRuleViolated {
		t.Fatalf("expected KindRuleViolated, got %v", err)
	}
	if de.Entity != "Player" {
		t.Fatalf("rule should fail on Player (declares score), got entity %q", de.Entity)
	}
}

// TestRuleTargetEntitiesScoping unit-tests the scoping resolver directly.
func TestRuleTargetEntitiesScoping(t *testing.T) {
	const schema = `domain: t
version: 0.1.0
entities:
  Player:
    id: uuid @primary
    score: int @range(0..100)
  Team:
    id: uuid @primary
    score: int @range(0..100)
  League:
    id: uuid @primary
    name: company.name
`
	doc, err := parser.New().Parse(strings.NewReader(schema), "schema.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	cases := []struct {
		expr string
		want []string
	}{
		{"Player.score >= 0", []string{"Player"}},            // qualified → named only
		{"score >= 0", []string{"Player", "Team"}},           // bare → both entities with score
		{"name != null", []string{"League"}},                 // bare → only entity with name
		{"id != null", []string{"Player", "Team", "League"}}, // bare field on all
		{"@@@", []string{"Player", "Team", "League"}},        // malformed → all (fail loud)
	}
	for _, c := range cases {
		got := ruleTargetEntities(c.expr, doc)
		if len(got) != len(c.want) {
			t.Fatalf("%q: got %v, want %v", c.expr, keysOf(got), c.want)
		}
		for _, w := range c.want {
			if _, ok := got[w]; !ok {
				t.Fatalf("%q: missing %q (got %v)", c.expr, w, keysOf(got))
			}
		}
	}
}

func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
