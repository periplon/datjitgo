package model

// DecoratorArgKind tags the shape of a decorator argument after parsing.
type DecoratorArgKind int

const (
	// ArgLiteral — a literal scalar (string/int/float/bool). Literal holds the typed value.
	ArgLiteral DecoratorArgKind = iota
	// ArgRange — a range expression lo..hi with optional exclusive endpoints.
	ArgRange
	// ArgKV — a key=value argument (e.g. mu=35, sigma=12).
	ArgKV
	// ArgIdent — a bare identifier (e.g. "normal" as first arg to @dist).
	ArgIdent
)

// DecoratorArg carries a parsed decorator argument. Raw is the original source
// fragment preserved for error reporting and round-trip rendering.
type DecoratorArg struct {
	Kind    DecoratorArgKind
	Raw     string
	Literal any // string | int64 | float64 | bool when Kind == ArgLiteral
	Ident   string
	// Key/Value for ArgKV
	Key   string
	Value string
	// Range fields for ArgRange
	From   string
	To     string
	LoExcl bool
	HiExcl bool
}

// Decorator is a parsed @name(args) annotation.
type Decorator struct {
	Name string
	Args []DecoratorArg
}

// ArgByKey returns the value associated with key in the first KV arg
// matching key. Returns "" if no match.
func (d Decorator) ArgByKey(key string) (string, bool) {
	for _, a := range d.Args {
		if a.Kind == ArgKV && a.Key == key {
			return a.Value, true
		}
	}
	return "", false
}

// HasDecorator returns true if decs contains a decorator with the given name.
func HasDecorator(decs []Decorator, name string) bool {
	for _, d := range decs {
		if d.Name == name {
			return true
		}
	}
	return false
}

// FindDecorator returns the first decorator with the given name, or nil.
func FindDecorator(decs []Decorator, name string) *Decorator {
	for i := range decs {
		if decs[i].Name == name {
			return &decs[i]
		}
	}
	return nil
}
