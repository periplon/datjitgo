package output

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// NDJSON is the newline-delimited JSON writer. One JSON object is emitted per
// row, terminated by a single LF. Entity and field order are preserved.
// Matches the Rust reference shape: rows are emitted as-is (no synthetic
// _entity field).
type NDJSON struct{}

// NewNDJSON returns a new NDJSON writer.
func NewNDJSON() *NDJSON { return &NDJSON{} }

// Format returns "ndjson".
func (*NDJSON) Format() string { return "ndjson" }

// Write emits one JSON object per row. Opts.Pretty is ignored — each record
// is always a single line.
func (n *NDJSON) Write(ds *value.Dataset, doc *model.Document, w io.Writer, opts ports.WriteOptions) error {
	if ds == nil {
		return nil
	}
	order := entityOrder(ds, doc, opts.EntityFilter)
	for _, name := range order {
		rows, _ := ds.Entities.Get(name)
		fields := fieldOrder(rows, doc, name)
		for _, row := range rows {
			obj := newOrderedJSONObject(len(fields))
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
			var buf bytes.Buffer
			e := json.NewEncoder(&buf)
			e.SetEscapeHTML(false)
			if err := e.Encode(obj); err != nil {
				return wrapIO(err, "ndjson encode")
			}
			// json.Encoder already appends a '\n'.
			if err := writeAll(w, buf.Bytes()); err != nil {
				return err
			}
		}
	}
	return nil
}
