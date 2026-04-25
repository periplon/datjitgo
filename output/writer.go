// Package output provides the ports.Writer implementations that serialise
// a value.Dataset to one of the supported formats (json, ndjson, csv, yaml,
// sql). All writers are deterministic: given identical inputs they produce
// byte-for-byte identical output.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
)

// jsonNumber is a json.Marshaler that emits its payload verbatim. Used so we
// can keep exact decimal scale in the output instead of round-tripping through
// float64.
type jsonNumber string

func (n jsonNumber) MarshalJSON() ([]byte, error) {
	return []byte(n), nil
}

// orderedJSONObject is an ordered key/value container that marshals to a JSON
// object with keys in insertion order.
type orderedJSONObject struct {
	keys   []string
	values map[string]any
}

func newOrderedJSONObject(size int) *orderedJSONObject {
	return &orderedJSONObject{
		keys:   make([]string, 0, size),
		values: make(map[string]any, size),
	}
}

func (o *orderedJSONObject) Set(k string, v any) {
	if _, ok := o.values[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.values[k] = v
}

func (o *orderedJSONObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(o.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// encodeValueJSON converts a value.Value into a Go value ready for
// encoding/json. Null → nil, decimal → jsonNumber (preserves scale), UUID →
// lowercase canonical string, time → RFC3339. Objects preserve their declared
// key order via orderedJSONObject.
func encodeValueJSON(v value.Value) (any, error) {
	switch v.Kind {
	case value.KindNull:
		return nil, nil
	case value.KindBool:
		return v.B, nil
	case value.KindInt:
		return v.I, nil
	case value.KindFloat:
		return v.F, nil
	case value.KindString:
		return v.S, nil
	case value.KindUUID:
		return v.U.String(), nil
	case value.KindTime:
		return v.T.UTC().Format(time.RFC3339Nano), nil
	case value.KindDecimal:
		return jsonNumber(v.D.String()), nil
	case value.KindList:
		out := make([]any, len(v.L))
		for i, item := range v.L {
			enc, err := encodeValueJSON(item)
			if err != nil {
				return nil, err
			}
			out[i] = enc
		}
		return out, nil
	case value.KindObject:
		if v.O == nil {
			return nil, nil
		}
		obj := newOrderedJSONObject(v.O.Len())
		var encErr error
		v.O.Each(func(k string, child value.Value) bool {
			enc, err := encodeValueJSON(child)
			if err != nil {
				encErr = err
				return false
			}
			obj.Set(k, enc)
			return true
		})
		if encErr != nil {
			return nil, encErr
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unknown value kind %d", v.Kind)
	}
}

// renderValueScalar renders a value.Value as a plain string (CSV cell, SQL
// literal decoration added elsewhere). Compound types are JSON-encoded.
func renderValueScalar(v value.Value) (string, error) {
	switch v.Kind {
	case value.KindNull:
		return "", nil
	case value.KindBool:
		if v.B {
			return "true", nil
		}
		return "false", nil
	case value.KindInt:
		return fmt.Sprintf("%d", v.I), nil
	case value.KindFloat:
		return formatFloat(v.F), nil
	case value.KindString:
		return v.S, nil
	case value.KindUUID:
		return v.U.String(), nil
	case value.KindTime:
		return v.T.UTC().Format(time.RFC3339Nano), nil
	case value.KindDecimal:
		return v.D.String(), nil
	case value.KindList, value.KindObject:
		enc, err := encodeValueJSON(v)
		if err != nil {
			return "", err
		}
		b, err := json.Marshal(enc)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("unknown value kind %d", v.Kind)
	}
}

// formatFloat formats a float64 without trailing zeros but always includes a
// decimal point if the value is not an integer. Uses strconv via fmt with %g
// which matches Go's default text float format and is deterministic.
func formatFloat(f float64) string {
	// %g is locale-free and compact.
	return strings.TrimSuffix(fmt.Sprintf("%g", f), "+00")
}

// entityOrder returns the ordered list of entity names to emit, respecting
// the optional EntityFilter, the document-declared order (if any), and
// falling back to dataset insertion order.
func entityOrder(ds *value.Dataset, doc *model.Document, filter string) []string {
	if filter != "" {
		if _, ok := ds.Entities.Get(filter); ok {
			return []string{filter}
		}
		return nil
	}

	present := make(map[string]bool, ds.Entities.Len())
	for _, k := range ds.Entities.Keys() {
		present[k] = true
	}
	var order []string
	if doc != nil {
		for _, k := range doc.Entities.Keys() {
			if present[k] {
				order = append(order, k)
			}
		}
	} else {
		order = ds.Entities.Keys()
	}
	return order
}

// fieldOrder returns the field order to use for the given entity's rows. If
// the document exposes a declaration order we prefer it; otherwise we fall
// back to the first row's key order.
func fieldOrder(rows []*value.Object, doc *model.Document, entity string) []string {
	if doc != nil {
		if ent, ok := doc.Entities.Get(entity); ok {
			keys := ent.Fields.Keys()
			if len(keys) > 0 {
				return keys
			}
		}
	}
	if len(rows) > 0 && rows[0] != nil {
		return rows[0].Keys()
	}
	return nil
}

// quoteSQLIdent quotes an identifier for the given SQL dialect.
func quoteSQLIdent(name, dialect string) string {
	switch strings.ToLower(dialect) {
	case "mysql":
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	default:
		return "\"" + strings.ReplaceAll(name, "\"", "\"\"") + "\""
	}
}

// escapeSQLString wraps a string in single quotes, doubling embedded single
// quotes.
func escapeSQLString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// wrapIO wraps a lower-level IO error from an encoder or writer into a typed
// *errors.Error so callers can pattern-match.
func wrapIO(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return errors.Wrap(errors.KindIO, err, format, args...)
}

// writeAll writes all of p to w, returning any error wrapped as KindIO.
func writeAll(w io.Writer, p []byte) error {
	_, err := w.Write(p)
	return wrapIO(err, "write output")
}
