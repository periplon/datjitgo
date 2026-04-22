package output

import (
	"encoding/json"
	"io"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// JSON is the JSON output writer. The top-level shape is an object keyed by
// entity name, each mapped to an array of row objects. Entity and field order
// are preserved.
type JSON struct{}

// NewJSON returns a new JSON writer.
func NewJSON() *JSON { return &JSON{} }

// Format returns "json".
func (*JSON) Format() string { return "json" }

// Write serialises ds to w as a single JSON document. Pretty output uses a
// 2-space indent. A streaming encoder is used so large datasets do not need
// to be materialised as a single []byte before writing.
func (j *JSON) Write(ds *value.Dataset, doc *model.Document, w io.Writer, opts ports.WriteOptions) error {
	if ds == nil {
		return writeAll(w, []byte("{}\n"))
	}

	order := entityOrder(ds, doc, opts.EntityFilter)

	top := newOrderedJSONObject(len(order))
	for _, name := range order {
		rows, _ := ds.Entities.Get(name)
		fields := fieldOrder(rows, doc, name)
		encoded := make([]any, 0, len(rows))
		for _, row := range rows {
			obj := newOrderedJSONObject(len(fields))
			// When the document gives us authoritative field order we use
			// it, else we fall back to the row's own key order.
			keys := fields
			if len(keys) == 0 && row != nil {
				keys = row.Keys()
			}
			for _, k := range keys {
				v, ok := row.Get(k)
				if !ok {
					continue
				}
				enc, err := encodeValueJSON(v)
				if err != nil {
					return wrapIO(err, "encode %s.%s", name, k)
				}
				obj.Set(k, enc)
			}
			encoded = append(encoded, obj)
		}
		top.Set(name, encoded)
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if opts.Pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(top); err != nil {
		return wrapIO(err, "json encode")
	}
	return nil
}
