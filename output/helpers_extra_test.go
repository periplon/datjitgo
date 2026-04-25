package output

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

func TestSQLTypeHelpersCoverDialects(t *testing.T) {
	cases := []model.TypeExpr{
		model.Nullable{Inner: model.Primitive{Kind: model.PrimInt}},
		model.Primitive{Kind: model.PrimFloat},
		model.Primitive{Kind: model.PrimBool},
		model.Primitive{Kind: model.PrimDatetime},
		model.Primitive{Kind: model.PrimDate},
		model.Primitive{Kind: model.PrimTime},
		model.Primitive{Kind: model.PrimDuration},
		model.Primitive{Kind: model.PrimUUID},
		model.Primitive{Kind: model.PrimBytes},
		model.Primitive{Kind: model.PrimDecimal, Params: []int{10, 4}},
		model.Primitive{Kind: model.PrimAny},
		model.Semantic{Namespace: "uuid"},
		model.EnumInline{Values: []string{"a"}},
		model.Reference{Target: "User"},
		model.List{Element: model.Primitive{Kind: model.PrimString}},
		model.Union{Variants: []model.TypeExpr{model.Primitive{Kind: model.PrimString}}},
	}
	for _, dialect := range []string{"postgres", "mysql", "sqlite"} {
		for _, typ := range cases {
			if got := sqlTypeFor(typ, dialect); got == "" {
				t.Fatalf("%s %#v returned empty type", dialect, typ)
			}
		}
	}
}

func TestScalarAndQuotingHelpers(t *testing.T) {
	for _, s := range []string{"", "true", " no ", "-item", "a:b", "line\nbreak"} {
		if !yamlNeedsQuoting(s) {
			t.Fatalf("%q should need YAML quoting", s)
		}
	}
	if yamlNeedsQuoting("plain") {
		t.Fatal("plain should not need quoting")
	}
	if got := hexEncode([]byte{0x0f, 0xa0}); got != "0fa0" {
		t.Fatalf("hex=%q", got)
	}
	obj := value.NewObject()
	obj.Set("x", value.Int(1))
	scalar, err := renderValueScalar(value.Obj(obj))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scalar, "x") {
		t.Fatalf("object scalar=%q", scalar)
	}
	if _, err := encodeValueJSON(value.Value{Kind: value.Kind(99)}); err == nil {
		t.Fatal("expected unknown JSON value kind error")
	}
	if _, err := renderValueScalar(value.Value{Kind: value.Kind(99)}); err == nil {
		t.Fatal("expected unknown scalar value kind error")
	}
	if err := wrapIO(nil, "noop"); err != nil {
		t.Fatalf("nil wrapIO=%v", err)
	}
	err = writeAll(errorWriter{}, []byte("x"))
	if err == nil {
		t.Fatal("expected write error")
	}
	if !stderrors.Is(err, errors.ErrIO) {
		t.Fatalf("expected IO error kind, got %v", err)
	}
}

func TestOrderHelpersFallbacksAndFilters(t *testing.T) {
	ds := value.NewDataset()
	first := value.NewObject()
	first.Set("b", value.Int(2))
	first.Set("a", value.Int(1))
	ds.Entities.Set("Second", []*value.Object{first})
	ds.Entities.Set("First", []*value.Object{{}})

	doc := model.NewDocument()
	doc.Entities.Set("First", model.NewEntity("First"))
	doc.Entities.Set("Second", model.NewEntity("Second"))
	if got := strings.Join(entityOrder(ds, doc, ""), ","); got != "First,Second" {
		t.Fatalf("document entity order=%q", got)
	}
	if got := entityOrder(ds, nil, "Missing"); got != nil {
		t.Fatalf("missing filter should return nil, got %v", got)
	}
	if got := strings.Join(entityOrder(ds, nil, "Second"), ","); got != "Second" {
		t.Fatalf("filtered order=%q", got)
	}
	if got := strings.Join(entityOrder(ds, nil, ""), ","); got != "Second,First" {
		t.Fatalf("dataset entity order=%q", got)
	}
	if got := strings.Join(fieldOrder([]*value.Object{first}, nil, "Second"), ","); got != "b,a" {
		t.Fatalf("row field order=%q", got)
	}
	if got := fieldOrder(nil, nil, "Second"); got != nil {
		t.Fatalf("empty field order=%v", got)
	}
}

func TestWritersHandleNilDatasetAndValidationBranches(t *testing.T) {
	doc := model.NewDocument()
	doc.Entities.Set("User", model.NewEntity("User"))

	var buf strings.Builder
	if err := NewJSON().Write(nil, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "{}\n" {
		t.Fatalf("nil JSON=%q", got)
	}
	buf.Reset()
	if err := NewYAML().Write(nil, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "{}\n" {
		t.Fatalf("nil YAML=%q", got)
	}
	buf.Reset()
	if err := NewNDJSON().Write(nil, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("nil NDJSON=%q", buf.String())
	}
	if err := NewCSV().Write(nil, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := NewSQL().Write(nil, doc, &buf, ports.WriteOptions{}); err != nil {
		t.Fatal(err)
	}

	ds := value.NewDataset()
	ds.Entities.Set("User", []*value.Object{})
	if err := NewSQL().Write(ds, doc, &buf, ports.WriteOptions{SQLDialect: "oracle"}); err == nil {
		t.Fatal("expected bad SQL dialect error")
	}
}

func TestWritersSurfaceUnknownValueKindErrors(t *testing.T) {
	doc := model.NewDocument()
	user := model.NewEntity("User")
	user.Fields.Set("bad", &model.Field{Name: "bad", Type: model.Primitive{Kind: model.PrimAny}})
	doc.Entities.Set("User", user)
	row := value.NewObject()
	row.Set("bad", value.Value{Kind: value.Kind(99)})
	ds := value.NewDataset()
	ds.Entities.Set("User", []*value.Object{row})

	for name, write := range map[string]func() error{
		"json":   func() error { return NewJSON().Write(ds, doc, &strings.Builder{}, ports.WriteOptions{}) },
		"ndjson": func() error { return NewNDJSON().Write(ds, doc, &strings.Builder{}, ports.WriteOptions{}) },
		"csv":    func() error { return NewCSV().Write(ds, doc, &strings.Builder{}, ports.WriteOptions{}) },
		"yaml":   func() error { return NewYAML().Write(ds, doc, &strings.Builder{}, ports.WriteOptions{}) },
		"sql":    func() error { return NewSQL().Write(ds, doc, &strings.Builder{}, ports.WriteOptions{}) },
	} {
		if err := write(); err == nil {
			t.Fatalf("%s: expected unknown kind error", name)
		}
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, stderrors.New("boom")
}
