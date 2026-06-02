package model

// Entity is one top-level entity definition from the DDL document.
type Entity struct {
	Name      string
	Meta      []Decorator
	Fields    *OrderedMap[string, *Field]
	Coherence *OrderedMap[string, []string]
}

// NewEntity constructs an empty entity with initialised maps.
func NewEntity(name string) *Entity {
	return &Entity{
		Name:      name,
		Fields:    NewOrderedMap[string, *Field](),
		Coherence: NewOrderedMap[string, []string](),
	}
}

// Field is a single field inside an entity (or reusable type).
type Field struct {
	Name         string
	Type         TypeExpr
	Decorators   []Decorator
	Label        string
	Description  string
	DefaultChain *DefaultChainSpec
	Compute      []ComputeBranch

	// Discriminator, on a polymorphic-reference field (a union of two or more
	// entity references), names the synthetic companion field that records
	// which target entity each generated row's reference points to. Empty for
	// ordinary fields. Set by polymorphic-reference normalization.
	Discriminator string
	// DiscriminatorFor, on a synthetic discriminator field, names the source
	// polymorphic-reference field it describes. Empty for ordinary fields.
	// The generator skips independent generation of such fields; their value
	// is produced as a side effect of generating the source field.
	DiscriminatorFor string
}

// DefaultChainSpec captures the @default_chain / `default_chain:` expanded
// form: walk Sources in order, take first non-null. `When` gates the whole
// chain; `Fallback` is used if every source is null.
type DefaultChainSpec struct {
	Sources  []string
	When     string
	Fallback string
}

// ComputeBranch is one branch of a `compute:` list. Empty When means else.
type ComputeBranch struct {
	When  string
	Value string
}
