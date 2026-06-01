package datjittest

import (
	"os"
	"path/filepath"
	"testing"

	datjit "github.com/periplon/datjitgo"
)

const helperSchema = `domain: helper_test
version: 0.1.0
seed: 42

volume:
  User: 2

entities:
  User:
    id: int @range(7..7)
`

func TestMustGenerateReturnsDataset(t *testing.T) {
	ds := MustGenerate(t, helperSchema, datjit.WithSeed(99))
	rows, ok := ds.Entities.Get("User")
	if !ok {
		t.Fatal("missing User entity")
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
}

func TestMustRowsReturnsEntityRowsAsMaps(t *testing.T) {
	rows := MustRows(t, helperSchema, "User", datjit.WithSeed(99))
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if got := rows[0]["id"]; got != int64(7) {
		t.Fatalf("first id = %#v, want int64(7)", got)
	}
}

func TestGoldenJSONHelpers(t *testing.T) {
	goldenPath := filepath.Join(t.TempDir(), "user.json")

	UpdateGoldenJSON(t, goldenPath, helperSchema, datjit.WithSeed(99))

	got, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "User": [
    {
      "id": 7
    },
    {
      "id": 7
    }
  ]
}
`
	if string(got) != want {
		t.Fatalf("golden contents:\n%s\nwant:\n%s", got, want)
	}

	AssertGoldenJSON(t, goldenPath, helperSchema, datjit.WithSeed(99))
}
