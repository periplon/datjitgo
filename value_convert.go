package datjit

import (
	"time"

	"github.com/jmcarbo/datjitgo/core/value"
)

// ValueAny converts a generated value into plain Go data suitable for callers
// that do not want to work with core/value directly.
func ValueAny(v value.Value) any {
	switch v.Kind {
	case value.KindNull:
		return nil
	case value.KindBool:
		return v.B
	case value.KindInt:
		return v.I
	case value.KindFloat:
		return v.F
	case value.KindString:
		return v.S
	case value.KindUUID:
		return v.U.String()
	case value.KindTime:
		return v.T.UTC().Format(time.RFC3339Nano)
	case value.KindDecimal:
		return v.D.String()
	case value.KindList:
		out := make([]any, len(v.L))
		for i, item := range v.L {
			out[i] = ValueAny(item)
		}
		return out
	case value.KindObject:
		return ObjectMap(v.O)
	default:
		return nil
	}
}

// ObjectMap converts one generated row into a plain map.
func ObjectMap(o *value.Object) map[string]any {
	if o == nil {
		return nil
	}
	out := make(map[string]any, o.Len())
	o.Each(func(k string, v value.Value) bool {
		out[k] = ValueAny(v)
		return true
	})
	return out
}

// RowsMap converts generated row objects into plain maps.
func RowsMap(rows []*value.Object) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = ObjectMap(row)
	}
	return out
}

// DatasetMap converts a generated dataset into entity-name keyed plain maps.
func DatasetMap(ds *value.Dataset) map[string][]map[string]any {
	if ds == nil || ds.Entities == nil {
		return nil
	}
	out := make(map[string][]map[string]any, ds.Entities.Len())
	ds.Entities.Each(func(entity string, rows []*value.Object) bool {
		out[entity] = RowsMap(rows)
		return true
	})
	return out
}
