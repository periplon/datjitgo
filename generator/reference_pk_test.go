package generator

import (
	"strings"
	"testing"

	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
	"github.com/periplon/datjitgo/parser"
)

// TestReferenceResolvesPrimaryKeyUnderCoherence guards the coherence/FK
// shadowing bug: when an FK-target entity also declares a coherence group,
// coherence members are written into the row before the @primary field, so the
// positional "first field" is a coherence value, not the key. Foreign keys must
// still resolve to the target's @primary (id), independent of insertion order.
func TestReferenceResolvesPrimaryKeyUnderCoherence(t *testing.T) {
	const schema = `domain: test_fk_pk
version: 0.1.0
seed: 7

volume:
  Customer: 5
  Order: 20

entities:
  Customer:
    id: uuid @primary
    _coherence:
      location: [city, state]
    city: string
    state: string

  Order:
    id: uuid @primary
    customer: ->Customer
`

	doc, err := parser.New().Parse(strings.NewReader(schema), "schema.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ds, err := newEngine().Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	customers, _ := ds.Entities.Get("Customer")
	if len(customers) == 0 {
		t.Fatal("no Customer rows")
	}

	// Sanity: coherence really does push id off the first position — otherwise
	// the test would pass even with the bug present.
	if got := customers[0].Keys()[0]; got == "id" {
		t.Fatalf("expected coherence to shadow id at position 0, but first key is %q", got)
	}

	validIDs := map[string]struct{}{}
	cityValues := map[string]struct{}{}
	for _, c := range customers {
		id, ok := c.Get("id")
		if !ok || id.Kind != value.KindUUID {
			t.Fatalf("customer missing uuid id: %v", c.Keys())
		}
		validIDs[valueKey(id)] = struct{}{}
		if city, ok := c.Get("city"); ok {
			cityValues[valueKey(city)] = struct{}{}
		}
	}

	orders, _ := ds.Entities.Get("Order")
	if len(orders) == 0 {
		t.Fatal("no Order rows")
	}
	for i, o := range orders {
		fk, ok := o.Get("customer")
		if !ok {
			t.Fatalf("order %d missing customer FK", i)
		}
		// The FK must be a primary-key uuid, never a coherence string value.
		if fk.Kind != value.KindUUID {
			t.Fatalf("order %d FK resolved to non-PK value kind=%v (%v) — coherence shadowed the primary key", i, fk.Kind, fk)
		}
		if _, ok := validIDs[valueKey(fk)]; !ok {
			t.Fatalf("order %d FK %v is not any Customer.id", i, fk)
		}
		if _, isCity := cityValues[valueKey(fk)]; isCity {
			t.Fatalf("order %d FK resolved to a coherence (city) value, not the primary key", i)
		}
	}
}
