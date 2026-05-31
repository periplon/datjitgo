package generator

import (
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
	"github.com/periplon/datjitgo/corpus"
)

func TestGenerateDoesNotMutateDocumentForEntityLevelLLM(t *testing.T) {
	doc := model.NewDocument()
	doc.Entities.Set("Article", model.NewEntity("Article"))
	ent, _ := doc.Entities.Get("Article")
	ent.Meta = append(ent.Meta, model.Decorator{Name: "llm", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: "write an article"}}})
	ent.Fields.Set("title", &model.Field{Name: "title", Type: model.Primitive{Kind: model.PrimString}})
	doc.Volume["Article"] = model.VolumeSpec{Exact: 1}

	if _, err := New(corpus.NewEmbedded()).Generate(doc, ports.GenerateOptions{}); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	field, _ := ent.Fields.Get("title")
	if got := len(field.Decorators); got != 0 {
		t.Fatalf("Generate mutated title decorators: got %d want 0", got)
	}
}

func TestStrictRuleViolationReturnsError(t *testing.T) {
	doc := model.NewDocument()
	ent := model.NewEntity("User")
	ent.Fields.Set("age", &model.Field{
		Name:       "age",
		Type:       model.Primitive{Kind: model.PrimInt},
		Decorators: []model.Decorator{{Name: "range", Args: []model.DecoratorArg{{Kind: model.ArgRange, From: "0", To: "10"}}}},
	})
	doc.Entities.Set("User", ent)
	doc.Volume["User"] = model.VolumeSpec{Exact: 1}
	doc.Rules = []model.Rule{{Kind: model.RuleKindExpr, Expr: "User.age >= 18", Severity: model.RuleStrict}}

	_, err := New(corpus.NewEmbedded()).Generate(doc, ports.GenerateOptions{})
	if err == nil {
		t.Fatal("Generate succeeded; expected strict rule violation")
	}
	if !stderrors.Is(err, errors.ErrRuleViolated) {
		t.Fatalf("Generate error should match ErrRuleViolated: %v", err)
	}
	if !strings.Contains(err.Error(), "User.age >= 18") {
		t.Fatalf("error %q does not mention violated rule", err)
	}
}

func TestNamedTypeGeneratesObject(t *testing.T) {
	doc := model.NewDocument()
	address := model.NewEntity("Address")
	address.Fields.Set("city", &model.Field{Name: "city", Type: model.Primitive{Kind: model.PrimString}, Decorators: []model.Decorator{{Name: "values", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: "Paris"}}}}})
	address.Fields.Set("zip", &model.Field{Name: "zip", Type: model.Primitive{Kind: model.PrimString}, Decorators: []model.Decorator{{Name: "values", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: "75001"}}}}})
	doc.Types.Set("Address", address)
	user := model.NewEntity("User")
	user.Fields.Set("address", &model.Field{Name: "address", Type: model.NamedType{Name: "Address"}})
	doc.Entities.Set("User", user)
	doc.Volume["User"] = model.VolumeSpec{Exact: 1}

	ds, err := New(corpus.NewEmbedded()).Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	rows, _ := ds.Entities.Get("User")
	got, _ := rows[0].Get("address")
	if got.Kind != value.KindObject {
		t.Fatalf("address kind = %v, want object", got.Kind)
	}
	if city, _ := got.O.Get("city"); city.Kind != value.KindString || city.S != "Paris" {
		t.Fatalf("address.city = %#v, want Paris", city)
	}
}

func TestDateRangeDecoratorConstrainsDatePrimitive(t *testing.T) {
	f := &model.Field{
		Name:       "starts_on",
		Type:       model.Primitive{Kind: model.PrimDate},
		Decorators: []model.Decorator{{Name: "range", Args: []model.DecoratorArg{{Kind: model.ArgRange, From: "2024-01-01", To: "2024-01-31"}}}},
	}
	got := New(corpus.NewEmbedded()).generatePrimitiveField(f, model.Primitive{Kind: model.PrimDate}, NewRand(7))
	got = applyRange(got, f.Decorators)
	lo := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	hi := time.Date(2024, 1, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if got.Kind != value.KindTime || got.T.Before(lo) || got.T.After(hi) {
		t.Fatalf("date = %#v, want within January 2024", got)
	}
}

func TestCurrencySemanticHonorsNumericParams(t *testing.T) {
	got, err := New(corpus.NewEmbedded()).generateSemantic(model.Semantic{
		Namespace: "currency",
		Tag:       "usd",
		Params:    []string{"10", "20"},
	}, NewRand(9))
	if err != nil {
		t.Fatalf("generateSemantic returned error: %v", err)
	}
	if got.Kind != value.KindFloat || got.F < 10 || got.F > 20 {
		t.Fatalf("currency.usd(10,20) = %#v, want float in range", got)
	}
}
