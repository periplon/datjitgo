package datjit_test

import (
	"bytes"
	"strings"
	"testing"

	datjit "github.com/periplon/datjitgo"
)

const dirtyFacadeSchema = `
domain: dirty_facade
seed: 9
volume:
  User: 20
entities:
  User:
    id: uuid @primary
    name: person.full
    email: email
`

// renderDirty parses the schema and renders JSON output through a service
// built with the given options.
func renderDirty(t *testing.T, opts ...datjit.Option) string {
	t.Helper()
	svc, err := datjit.New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.Parse(strings.NewReader(dirtyFacadeSchema), "dirty_facade.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestWithDirtyRateChangesOutput(t *testing.T) {
	clean := renderDirty(t)
	if clean != renderDirty(t) {
		t.Fatal("clean output not deterministic")
	}
	if clean != renderDirty(t, datjit.WithDirtyRate(0)) {
		t.Fatal("WithDirtyRate(0) must match the clean baseline exactly")
	}
	dirty := renderDirty(t, datjit.WithDirtyRate(1))
	if dirty == clean {
		t.Fatal("WithDirtyRate(1) produced output identical to the clean baseline")
	}
	if dirty != renderDirty(t, datjit.WithDirtyRate(1)) {
		t.Fatal("dirty output not deterministic for a fixed seed")
	}
}

func TestWithDirtyRateRejectsOutOfRange(t *testing.T) {
	for _, rate := range []float64{-0.1, 1.1} {
		if _, err := datjit.New(datjit.WithDirtyRate(rate)); err == nil {
			t.Fatalf("WithDirtyRate(%v): expected validation error", rate)
		}
	}
}
