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

func Null() Value                 { return Value{Kind: KindNull} }
func Bool(b bool) Value           { return Value{Kind: KindBool, B: b} }
func Int(i int64) Value           { return Value{Kind: KindInt, I: i} }
func Float(f float64) Value       { return Value{Kind: KindFloat, F: f} }
func Str(s string) Value          { return Value{Kind: KindString, S: s} }
func UUID(u uuid.UUID) Value      { return Value{Kind: KindUUID, U: u} }
func Time(t time.Time) Value      { return Value{Kind: KindTime, T: t} }
func Dec(d decimal.Decimal) Value { return Value{Kind: KindDecimal, D: d} }
func List(xs []Value) Value       { return Value{Kind: KindList, L: xs} }
func Obj(o *Object) Value         { return Value{Kind: KindObject, O: o} }

func (v Value) IsNull() bool { return v.Kind == KindNull }

// Object is an ordered map of string → Value. Order is preserved so JSON/
// YAML output matches schema declaration order.
type Object struct {
	keys []string
	m    map[string]Value
}

func NewObject() *Object { return &Object{m: map[string]Value{}} }

func (o *Object) Set(k string, v Value) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *Object) Get(k string) (Value, bool) { v, ok := o.m[k]; return v, ok }
func (o *Object) Has(k string) bool          { _, ok := o.m[k]; return ok }
func (o *Object) Keys() []string             { return append([]string(nil), o.keys...) }
func (o *Object) Len() int                   { return len(o.keys) }

func (o *Object) Each(fn func(k string, v Value) bool) {
	for _, k := range o.keys {
		if !fn(k, o.m[k]) {
			return
		}
	}
}

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

func NewDataset() *Dataset {
	return &Dataset{Entities: &DatasetMap{m: map[string][]*Object{}}}
}

func (d *DatasetMap) Set(k string, rows []*Object) {
	if _, ok := d.m[k]; !ok {
		d.keys = append(d.keys, k)
	}
	d.m[k] = rows
}

func (d *DatasetMap) Get(k string) ([]*Object, bool) { r, ok := d.m[k]; return r, ok }
func (d *DatasetMap) Has(k string) bool              { _, ok := d.m[k]; return ok }
func (d *DatasetMap) Keys() []string                 { return append([]string(nil), d.keys...) }
func (d *DatasetMap) Len() int                       { return len(d.keys) }

func (d *DatasetMap) Each(fn func(k string, rows []*Object) bool) {
	for _, k := range d.keys {
		if !fn(k, d.m[k]) {
			return
		}
	}
}
