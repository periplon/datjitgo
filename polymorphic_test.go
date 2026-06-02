package datjit_test

import (
	"bytes"
	"encoding/json"
	"testing"

	datjit "github.com/periplon/datjitgo"
)

const polySchema = `
domain: poly_test
version: 0.1.0
seed: 7
volume:
  User: 4
  Org: 4
  Comment: 12
entities:
  User:
    id: uuid @primary
    name: string
  Org:
    id: uuid @primary
    name: string
  Comment:
    id: uuid @primary
    author: "->User | ->Org"
`

func generatePolyJSON(t *testing.T) map[string][]map[string]any {
	t.Helper()
	svc := datjit.NewDefault()
	doc, err := svc.Parse(bytes.NewReader([]byte(polySchema)), "poly.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	var buf bytes.Buffer
	if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{}); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out map[string][]map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	return out
}

func TestPolymorphic_DiscriminatorMatchesTarget(t *testing.T) {
	out := generatePolyJSON(t)

	idSet := func(rows []map[string]any) map[string]bool {
		s := map[string]bool{}
		for _, r := range rows {
			if id, ok := r["id"].(string); ok {
				s[id] = true
			}
		}
		return s
	}
	users := idSet(out["User"])
	orgs := idSet(out["Org"])

	comments := out["Comment"]
	if len(comments) == 0 {
		t.Fatal("no comments generated")
	}
	sawUser, sawOrg := false, false
	for i, c := range comments {
		disc, ok := c["author_type"]
		if !ok {
			t.Fatalf("comment %d missing author_type discriminator", i)
		}
		author, _ := c["author"].(string)
		switch disc {
		case "User":
			sawUser = true
			if !users[author] {
				t.Fatalf("comment %d author_type=User but author %q is not a User id", i, author)
			}
		case "Org":
			sawOrg = true
			if !orgs[author] {
				t.Fatalf("comment %d author_type=Org but author %q is not an Org id", i, author)
			}
		default:
			t.Fatalf("comment %d unexpected author_type %v", i, disc)
		}
	}
	// With 12 comments over a 2-way uniform choice, both targets are
	// overwhelmingly likely; assert to catch a stuck/constant discriminator.
	if !sawUser || !sawOrg {
		t.Fatalf("expected both User and Org targets; sawUser=%v sawOrg=%v", sawUser, sawOrg)
	}
}

func TestPolymorphic_Deterministic(t *testing.T) {
	a := generatePolyJSON(t)
	b := generatePolyJSON(t)
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(ab) != string(bb) {
		t.Fatal("polymorphic generation is not deterministic across runs with the same seed")
	}
}
