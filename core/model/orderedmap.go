package model

// OrderedMap preserves insertion order of keys. Overwriting an existing key
// keeps its original position. Used throughout the domain model to keep YAML
// declaration order stable so generated output is deterministic.
type OrderedMap[K comparable, V any] struct {
	keys []K
	m    map[K]V
}

// NewOrderedMap returns an empty OrderedMap.
func NewOrderedMap[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{m: map[K]V{}}
}

// Set assigns v to k, appending k to the insertion order on first write.
func (o *OrderedMap[K, V]) Set(k K, v V) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

// Get returns the value for k and a flag indicating whether the key exists.
func (o *OrderedMap[K, V]) Get(k K) (V, bool) {
	v, ok := o.m[k]
	return v, ok
}

// Has reports whether k is present in the map.
func (o *OrderedMap[K, V]) Has(k K) bool {
	_, ok := o.m[k]
	return ok
}

// Len returns the number of keys in the map.
func (o *OrderedMap[K, V]) Len() int { return len(o.keys) }

// Keys returns a copy of the keys in insertion order.
func (o *OrderedMap[K, V]) Keys() []K { return append([]K(nil), o.keys...) }

// Each iterates in insertion order. Return false from fn to stop early.
func (o *OrderedMap[K, V]) Each(fn func(K, V) bool) {
	for _, k := range o.keys {
		if !fn(k, o.m[k]) {
			return
		}
	}
}

// Delete removes k from the map and its slot from the insertion order.
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
