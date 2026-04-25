package datjit_test

import (
	"fmt"
	"log"
	"strings"

	datjit "github.com/jmcarbo/datjitgo"
)

// ExampleGenerateMapString shows the simplest one-call API: turn a YAML
// schema string into entity-keyed Go maps with a fixed seed for
// reproducibility.
func ExampleGenerateMapString() {
	schema := `domain: example
volume:
  User: 3
entities:
  User:
    id: int @primary
    name: person.full
`
	data, err := datjit.GenerateMapString(schema, datjit.WithSeed(42))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(data["User"]))
	// Output: 3
}

// ExampleGenerateRowsFile generates rows for one entity from a YAML fixture
// on disk. The fixture's volume declares 10 User rows.
func ExampleGenerateRowsFile() {
	rows, err := datjit.GenerateRowsFile(
		"testdata/fixtures/minimal.yaml",
		"User",
		datjit.WithSeed(42),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(rows))
	// Output: 10
}

// ExampleService_Generate drives the Service facade explicitly: parse a
// schema from an io.Reader, validate it, and generate the dataset.
func ExampleService_Generate() {
	schema := `domain: example
volume:
  Item: 1
entities:
  Item:
    id: int @primary
    name: person.full
`
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(schema), "schema.yaml")
	if err != nil {
		log.Fatal(err)
	}
	if err := svc.Validate(doc); err != nil {
		log.Fatal(err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ds.Entities.Len())
	// Output: 1
}
