// Package value defines the runtime Value type used by the generator and
// output writers. Values are explicitly tagged rather than interface-typed
// to avoid per-field heap allocations in the hot path.
package value

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kind is the tag discriminating the payload of a Value.
type Kind int

// The Kind constants enumerate every payload a Value can carry.
const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindUUID
	KindTime
	KindDecimal
	KindList
	KindObject
)

// Value is a tagged union. Only the field matching Kind is meaningful.
type Value struct {
	Kind Kind
	B    bool
	I    int64
	F    float64
	S    string
	U    uuid.UUID
	T    time.Time
	D    decimal.Decimal
	L    []Value
	O    *Object
}

// Null returns the null Value.
func Null() Value { return Value{Kind: KindNull} }

// Bool returns a Value wrapping the bool b.
func Bool(b bool) Value { return Value{Kind: KindBool, B: b} }

// Int returns a Value wrapping the int64 i.
func Int(i int64) Value { return Value{Kind: KindInt, I: i} }

// Float returns a Value wrapping the float64 f.
func Float(f float64) Value { return Value{Kind: KindFloat, F: f} }

// Str returns a Value wrapping the string s.
func Str(s string) Value { return Value{Kind: KindString, S: s} }

// UUID returns a Value wrapping the uuid.UUID u.
func UUID(u uuid.UUID) Value { return Value{Kind: KindUUID, U: u} }

// Time returns a Value wrapping the time.Time t.
func Time(t time.Time) Value { return Value{Kind: KindTime, T: t} }

// Dec returns a Value wrapping the decimal.Decimal d.
func Dec(d decimal.Decimal) Value { return Value{Kind: KindDecimal, D: d} }

// List returns a Value wrapping the slice xs.
func List(xs []Value) Value { return Value{Kind: KindList, L: xs} }

// Obj returns a Value wrapping the Object o.
func Obj(o *Object) Value { return Value{Kind: KindObject, O: o} }

// IsNull reports whether v is the null Value.
func (v Value) IsNull() bool { return v.Kind == KindNull }

// Object is an ordered map of string → Value. Order is preserved so JSON/
// YAML output matches schema declaration order.
type Object struct {
	keys []string
	m    map[string]Value
}

// NewObject returns an empty Object.
func NewObject() *Object { return &Object{m: map[string]Value{}} }

// Set stores v under key k, appending k to the key order on first insert.
func (o *Object) Set(k string, v Value) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

// Get returns the value for k and whether it is present.
func (o *Object) Get(k string) (Value, bool) { v, ok := o.m[k]; return v, ok }

// Has reports whether k is present.
func (o *Object) Has(k string) bool { _, ok := o.m[k]; return ok }

// Keys returns a copy of the keys in insertion order.
func (o *Object) Keys() []string { return append([]string(nil), o.keys...) }

// Len returns the number of keys.
func (o *Object) Len() int { return len(o.keys) }

// Each calls fn for each key/value in insertion order, stopping early if fn
// returns false.
func (o *Object) Each(fn func(k string, v Value) bool) {
	for _, k := range o.keys {
		if !fn(k, o.m[k]) {
			return
		}
	}
}

// Delete removes k from the object, preserving the order of remaining keys.
func (o *Object) Delete(k string) {
	if _, ok := o.m[k]; !ok {
		return
	}
	delete(o.m, k)
	for i, kk := range o.keys {
		if kk == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			return
		}
	}
}

// Dataset is the fully generated output — entity name → row objects.
type Dataset struct {
	Entities *DatasetMap
}

// DatasetMap is an ordered map preserving entity order from the document.
type DatasetMap struct {
	keys []string
	m    map[string][]*Object
}

// NewDataset returns an empty Dataset with an initialized entity map.
func NewDataset() *Dataset {
	return &Dataset{Entities: &DatasetMap{m: map[string][]*Object{}}}
}

// Set stores rows under entity name k, appending k to the order on first insert.
func (d *DatasetMap) Set(k string, rows []*Object) {
	if _, ok := d.m[k]; !ok {
		d.keys = append(d.keys, k)
	}
	d.m[k] = rows
}

// Get returns the rows for entity k and whether it is present.
func (d *DatasetMap) Get(k string) ([]*Object, bool) { r, ok := d.m[k]; return r, ok }

// Has reports whether entity k is present.
func (d *DatasetMap) Has(k string) bool { _, ok := d.m[k]; return ok }

// Keys returns a copy of the entity names in insertion order.
func (d *DatasetMap) Keys() []string { return append([]string(nil), d.keys...) }

// Len returns the number of entities.
func (d *DatasetMap) Len() int { return len(d.keys) }

// Each calls fn for each entity in insertion order, stopping early if fn
// returns false.
func (d *DatasetMap) Each(fn func(k string, rows []*Object) bool) {
	for _, k := range d.keys {
		if !fn(k, d.m[k]) {
			return
		}
	}
}
