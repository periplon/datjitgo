package datjittest_test

import (
	"fmt"
	"testing"

	datjit "github.com/jmcarbo/datjitgo"
	"github.com/jmcarbo/datjitgo/datjittest"
)

// ExampleMustRows shows the test-helper sugar for generating fixture rows
// from an inline schema. The helper fails the surrounding *testing.T on any
// pipeline error, so call sites stay focused on assertions.
func ExampleMustRows() {
	schema := `domain: example
volume:
  User: 2
entities:
  User:
    id: int @primary
    name: person.full
`
	t := &testing.T{}
	rows := datjittest.MustRows(t, schema, "User", datjit.WithSeed(42))
	fmt.Println(len(rows))
	// Output: 2
}
