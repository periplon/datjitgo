package output_test

import (
	"log"
	"os"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
	"github.com/periplon/datjitgo/output"
)

// tinyDataset returns a one-entity, one-row dataset with stable scalar
// values so example output is byte-deterministic.
func tinyDataset() (*value.Dataset, *model.Document) {
	doc := model.NewDocument()
	ent := model.NewEntity("User")
	ent.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimInt}})
	ent.Fields.Set("name", &model.Field{Name: "name", Type: model.Primitive{Kind: model.PrimString}})
	doc.Entities.Set(ent.Name, ent)

	row := value.NewObject()
	row.Set("id", value.Int(1))
	row.Set("name", value.Str("Ada"))

	ds := value.NewDataset()
	ds.Entities.Set("User", []*value.Object{row})
	return ds, doc
}

// ExampleNewJSON serialises a tiny dataset as JSON to stdout.
func ExampleNewJSON() {
	ds, doc := tinyDataset()
	w := output.NewJSON()
	if err := w.Write(ds, doc, os.Stdout, ports.WriteOptions{}); err != nil {
		log.Fatal(err)
	}
	// Output: {"User":[{"id":1,"name":"Ada"}]}
}

// ExampleNewCSV serialises a tiny dataset as CSV to stdout.
func ExampleNewCSV() {
	ds, doc := tinyDataset()
	w := output.NewCSV()
	if err := w.Write(ds, doc, os.Stdout, ports.WriteOptions{}); err != nil {
		log.Fatal(err)
	}
	// Output:
	// id,name
	// 1,Ada
}
