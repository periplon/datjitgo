package output

import (
	"encoding/csv"
	"io"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

// CSV is the CSV output writer. One section per entity: a header row with the
// declared field names followed by one row per record. Sections are separated
// by a single blank line. Requires a Document so field declaration order is
// unambiguous.
type CSV struct{}

// NewCSV returns a new CSV writer.
func NewCSV() *CSV { return &CSV{} }

// Format returns "csv".
func (*CSV) Format() string { return "csv" }

// Write serialises ds to w. Null cells are emitted as empty strings; bools as
// true/false; decimals unquoted; compound values JSON-encoded into a single
// cell. Returns a validation error when doc is nil (field types required).
func (c *CSV) Write(ds *value.Dataset, doc *model.Document, w io.Writer, opts ports.WriteOptions) error {
	if doc == nil {
		return &errors.Error{Kind: errors.KindValidation, Message: "csv writer requires Document"}
	}
	if ds == nil {
		return nil
	}
	order := entityOrder(ds, doc, opts.EntityFilter)

	cw := csv.NewWriter(w)
	cw.UseCRLF = false

	for i, name := range order {
		rows, _ := ds.Entities.Get(name)
		fields := fieldOrder(rows, doc, name)

		if i > 0 {
			// Blank line separator between entity sections.
			if err := writeAll(w, []byte("\n")); err != nil {
				return err
			}
		}

		if err := cw.Write(fields); err != nil {
			return wrapIO(err, "csv header %s", name)
		}

		record := make([]string, len(fields))
		for _, row := range rows {
			for j, k := range fields {
				if row == nil {
					record[j] = ""
					continue
				}
				v, ok := row.Get(k)
				if !ok || v.IsNull() {
					record[j] = ""
					continue
				}
				s, err := renderValueScalar(v)
				if err != nil {
					return wrapIO(err, "csv %s.%s", name, k)
				}
				record[j] = s
			}
			if err := cw.Write(record); err != nil {
				return wrapIO(err, "csv row %s", name)
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return wrapIO(err, "csv flush %s", name)
		}
	}
	return nil
}
