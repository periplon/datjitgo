package model

// OrderedMap preserves insertion order of keys. Overwriting an existing key
// keeps its original position. Used throughout the domain model to keep YAML
// declaration order stable so generated output is deterministic.
type OrderedMap[K comparable, V any] struct {
	keys []K
	m    map[K]V
}

func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{m: map[K]V{}}
}

func (o *OrderedMap[K, V]) Set(k K, v V) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *OrderedMap[K, V]) Get(k K) (V, bool) {
	v, ok := o.m[k]
	return v, ok
}

func (o *OrderedMap[K, V]) Has(k K) bool {
	_, ok := o.m[k]
	return ok
}

func (o *OrderedMap[K, V]) Len() int    { return len(o.keys) }
func (o *OrderedMap[K, V]) Keys() []K   { return append([]K(nil), o.keys...) }

// Each iterates in insertion order. Return false from fn to stop early.
func (o *OrderedMap[K, V]) Each(fn func(K, V) bool) {
	for _, k := range o.keys {
		if !fn(k, o.m[k]) {
			return
		}
	}
}

func (o *OrderedMap[K, V]) Delete(k K) {
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
