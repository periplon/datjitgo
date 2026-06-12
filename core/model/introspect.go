package model

import "strings"

// SchemaSummary is a stable, ordered, machine-readable schema signature. It is
// a read-only projection of a Document suitable for committing as a CI drift
// fixture and diffing across schema versions. All slices are deterministically
// ordered (see field comments) so two summaries of the same Document compare
// and serialize identically.
type SchemaSummary struct {
	Domain   string                `json:"domain"`
	Version  string                `json:"version"`
	Locale   string                `json:"locale"`
	Entities []SchemaEntitySummary `json:"entities"` // document order
	Enums    []EnumSummary         `json:"enums"`    // sorted by name
	Rules    []string              `json:"rules"`    // canonical rule strings, document order
	Volumes  []VolumeSummary       `json:"volumes"`  // sorted by entity name
}

// SchemaEntitySummary is the per-entity projection used by SchemaSummary.
// Fields are listed in declaration order.
//
// It is named SchemaEntitySummary (not EntitySummary) because the stable
// public model.EntitySummary already exists for the inspect surface; this
// feature is purely additive and must not break it.
type SchemaEntitySummary struct {
	Name   string         `json:"name"`
	Fields []FieldSummary `json:"fields"` // declaration order
}

// FieldSummary captures a field's canonical type and decorator strings. The
// strings are deterministic and round-trip stable: the same Document always
// yields the same strings.
type FieldSummary struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`       // canonical type string, e.g. "->User | ->Org", "int"
	Decorators []string `json:"decorators"` // canonical decorator strings, e.g. "@unique", "@range(18..65)"
}

// EnumSummary is the per-enum projection: a name and its variant values in
// declaration order.
type EnumSummary struct {
	Name     string   `json:"name"`
	Variants []string `json:"variants"`
}

// VolumeSummary records the effective volume spec for an entity as a string,
// e.g. "10" or "10..20".
type VolumeSummary struct {
	Entity string `json:"entity"`
	Spec   string `json:"spec"`
}

// SchemaDiff is the comparison of two SchemaSummary values, classifying every
// change as breaking or compatible.
type SchemaDiff struct {
	Changes []SchemaChange `json:"changes"`
}

// Breaking reports whether the diff contains any breaking change.
func (d *SchemaDiff) Breaking() bool {
	if d == nil {
		return false
	}
	for _, c := range d.Changes {
		if c.Breaking {
			return true
		}
	}
	return false
}

// Empty reports whether the diff contains no changes.
func (d *SchemaDiff) Empty() bool {
	return d == nil || len(d.Changes) == 0
}

// SchemaChange is one entry in a SchemaDiff. Kind names the category of change;
// Entity and Field locate it where applicable; Old and New carry the before and
// after values (either may be empty for additions/removals).
//
// Breaking is true for changes that alter the consumer-visible shape:
// entity-removed, field-removed, field-type-changed, enum-removed,
// enum variant removal, and domain-changed. Additions, volume changes,
// decorator changes, and variant additions are compatible — note that decorator
// changes can alter generated values but not the consumer-visible shape, so
// they are classified compatible.
type SchemaChange struct {
	Kind     string `json:"kind"`
	Entity   string `json:"entity,omitempty"`
	Field    string `json:"field,omitempty"`
	Old      string `json:"old,omitempty"`
	New      string `json:"new,omitempty"`
	Breaking bool   `json:"breaking"`
}

// DependencyGraph describes the entity-reference structure of a Document:
// nodes (entity names in document order), directed edges (one per referencing
// field, with polymorphic unions expanded to one edge per target), and any
// reference cycles. Self-references are excluded from edges and cycles.
type DependencyGraph struct {
	Nodes  []string   `json:"nodes"`  // entity names, document order
	Edges  []DepEdge  `json:"edges"`  // one edge per referencing field/target
	Cycles [][]string `json:"cycles"` // each cycle as an entity path, e.g. ["A","B","A"]
}

// DepEdge is a single reference edge in a DependencyGraph. Kind is one of
// "reference", "many-to-many", "polymorphic", or "self".
type DepEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Field string `json:"field"` // referencing field name
	Kind  string `json:"kind"`  // "reference" | "many-to-many" | "polymorphic" | "self"
}

// RenderType returns the canonical DDL string for a TypeExpr. The rendering is
// deterministic and round-trip stable: parsing the result yields an equivalent
// TypeExpr. It is a pure struct-to-string projection and needs no parser
// knowledge, so it lives in the dependency-free model package.
func RenderType(t TypeExpr) string {
	switch v := t.(type) {
	case Primitive:
		return renderPrimitive(v)
	case Semantic:
		return renderSemantic(v)
	case EnumInline:
		return "enum(" + strings.Join(v.Values, ", ") + ")"
	case NamedType:
		return v.Name
	case Reference:
		return renderReference(v)
	case List:
		return "[" + RenderType(v.Element) + "]"
	case Map:
		return "{" + RenderType(v.Key) + ": " + RenderType(v.Value) + "}"
	case Tuple:
		parts := make([]string, len(v.Elements))
		for i, e := range v.Elements {
			parts[i] = RenderType(e)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case Nullable:
		return RenderType(v.Inner) + "?"
	case Union:
		parts := make([]string, len(v.Variants))
		for i, e := range v.Variants {
			parts[i] = RenderType(e)
		}
		return strings.Join(parts, " | ")
	case nil:
		return ""
	}
	return "?"
}

// renderPrimitive renders a primitive, appending any (params) suffix.
func renderPrimitive(p Primitive) string {
	name := p.Kind.String()
	if len(p.Params) == 0 {
		return name
	}
	parts := make([]string, len(p.Params))
	for i, n := range p.Params {
		parts[i] = itoa(n)
	}
	return name + "(" + strings.Join(parts, ", ") + ")"
}

// renderSemantic renders a semantic type, joining namespace and tag with a dot
// and appending any (params) suffix.
func renderSemantic(s Semantic) string {
	name := s.Namespace
	if s.Tag != "" {
		name += "." + s.Tag
	}
	if len(s.Params) == 0 {
		return name
	}
	return name + "(" + strings.Join(s.Params, ", ") + ")"
}

// renderReference renders an entity reference in its canonical arrow form:
// ->E, ->E?, ->[E], or <->E.
func renderReference(r Reference) string {
	if r.ManyToMany {
		return "<->" + r.Target
	}
	if r.Many {
		return "->[" + r.Target + "]"
	}
	out := "->" + r.Target
	if r.Optional {
		out += "?"
	}
	return out
}

// RenderDecorator returns the canonical "@name(args)" string for a Decorator.
// Arguments are rendered from each DecoratorArg's preserved Raw fragment where
// present, so the result round-trips the original source form.
func RenderDecorator(d Decorator) string {
	if len(d.Args) == 0 {
		return "@" + d.Name
	}
	parts := make([]string, len(d.Args))
	for i, a := range d.Args {
		parts[i] = renderDecoratorArg(a)
	}
	return "@" + d.Name + "(" + strings.Join(parts, ", ") + ")"
}

// renderDecoratorArg renders a single decorator argument. The parser preserves
// the original source fragment in Raw, which is the most faithful canonical
// form; only when Raw is empty do we reconstruct from the typed fields.
func renderDecoratorArg(a DecoratorArg) string {
	if a.Raw != "" {
		return a.Raw
	}
	switch a.Kind {
	case ArgKV:
		return a.Key + "=" + a.Value
	case ArgIdent:
		return a.Ident
	case ArgRange:
		sep := ".."
		switch {
		case a.LoExcl && a.HiExcl:
			sep = "<..<"
		case a.LoExcl:
			sep = "<.."
		case a.HiExcl:
			sep = "..<"
		}
		return a.From + sep + a.To
	default:
		return literalString(a.Literal)
	}
}

// RenderDecorators renders each decorator in decs to its canonical string,
// preserving declaration order.
func RenderDecorators(decs []Decorator) []string {
	if len(decs) == 0 {
		return nil
	}
	out := make([]string, len(decs))
	for i, d := range decs {
		out[i] = RenderDecorator(d)
	}
	return out
}

// literalString renders a decorator literal value without pulling in fmt.
func literalString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return itoa64(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return ""
	}
}

// itoa formats a non-negative-or-negative int as decimal without fmt.
func itoa(n int) string { return itoa64(int64(n)) }

// itoa64 formats an int64 as decimal without fmt.
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
