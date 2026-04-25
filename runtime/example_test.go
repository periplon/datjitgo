package runtime_test

import (
	"context"
	"fmt"
	"log"
	"strings"

	datjit "github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/runtime"
)

// ExampleDefault_GenerateDocument shows a host runtime parsing a YAML schema
// once, then driving the embeddable runtime with a per-call seed and volume
// override.
func ExampleDefault_GenerateDocument() {
	schema := `domain: example
volume:
  User: 1
entities:
  User:
    id: int @primary
    name: person.full
`
	svc := datjit.NewDefault()
	doc, err := svc.Parse(strings.NewReader(schema), "schema.yaml")
	if err != nil {
		log.Fatal(err)
	}

	rt := runtime.NewDefault()
	ds, err := rt.GenerateDocument(
		context.Background(),
		doc,
		runtime.WithSeed(42),
		runtime.WithVolume("User", 2),
	)
	if err != nil {
		log.Fatal(err)
	}

	rows, _ := ds.Entities.Get("User")
	fmt.Println(len(rows))
	// Output: 2
}
