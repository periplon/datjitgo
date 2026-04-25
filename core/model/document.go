package model

// VolumeSpec describes how many rows to generate for an entity.
// Exactly one of Exact or (Min,Max) is set, unless Inferred is true (volume
// derived from parent count decorators).
type VolumeSpec struct {
	Exact    int
	Min      int
	Max      int
	Inferred bool
}

// IsRange reports whether the volume is a range.
func (v VolumeSpec) IsRange() bool { return v.Min != 0 || v.Max != 0 }

// GenerationConfig mirrors the `generation:` block in the DDL document.
type GenerationConfig struct {
	Seed         *int64
	Locale       string
	Locales      map[string]int // weighted locale distribution
	NullStrategy string         // realistic|never|sparse
	IDFormat     string         // uuid|sequential|cuid|ulid
	DateFormat   string
	CurrencyFmt  string
	LLM          *LLMConfig
}

// LLMConfig configures optional live LLM generation. It is inert unless the
// caller explicitly installs an LLM provider.
type LLMConfig struct {
	Provider    string
	Endpoint    string
	Model       string
	APIKey      string
	Temperature *float64
	TimeoutSecs *int
	MaxTokens   *int
}

// ToolOverride is a pass-through of the `tools:` section kept as raw YAML
// content so `inspect` can surface it without the engine needing to
// interpret tool semantics in phase 1.
type ToolOverride struct {
	Raw map[string]any
}

// Document is the fully parsed DDL document.
type Document struct {
	Domain     string
	Version    string
	Seed       *int64
	Locale     string
	Volume     map[string]VolumeSpec
	Entities   *OrderedMap[string, *Entity]
	Enums      *OrderedMap[string, EnumDef]
	Types      *OrderedMap[string, *Entity]
	Rules      []Rule
	Tools      map[string]ToolOverride
	Generation GenerationConfig
}

// NewDocument returns a Document with initialised maps so callers can Set
// without a nil check.
func NewDocument() *Document {
	return &Document{
		Volume:   map[string]VolumeSpec{},
		Entities: NewOrderedMap[string, *Entity](),
		Enums:    NewOrderedMap[string, EnumDef](),
		Types:    NewOrderedMap[string, *Entity](),
		Tools:    map[string]ToolOverride{},
	}
}

// Inspection is a summary produced by the inspect pipeline.
type Inspection struct {
	Domain      string
	Version     string
	EntityCount int
	Entities    []EntitySummary
	Enums       []EnumDef
	Rules       []Rule
}

// EntitySummary is a one-line-per-entity projection for inspect output.
type EntitySummary struct {
	Name         string
	FieldCount   int
	Dependencies []string
	VolumePlan   VolumeSpec
}
