package generator

import (
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
)

func TestLLMPreprocessAndFallbacks(t *testing.T) {
	doc := model.NewDocument()
	ent := model.NewEntity("Article")
	ent.Meta = []model.Decorator{{Name: "llm", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: "write", Raw: "write"}}}}
	ent.Fields.Set("title", &model.Field{Name: "title", Type: model.Primitive{Kind: model.PrimString}})
	ent.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimString}, Decorators: []model.Decorator{{Name: "primary"}}})
	ent.Fields.Set("ref", &model.Field{Name: "ref", Type: model.Reference{Target: "Article"}})
	doc.Entities.Set("Article", ent)
	preprocessLLM(doc)
	title, _ := ent.Fields.Get("title")
	if findLLM(title.Decorators) == nil {
		t.Fatal("title should inherit entity llm")
	}
	id, _ := ent.Fields.Get("id")
	if findLLM(id.Decorators) != nil {
		t.Fatal("primary field should not inherit llm")
	}
	ref, _ := ent.Fields.Get("ref")
	if shouldInheritEntityLLM(ref) {
		t.Fatal("reference should not inherit llm")
	}

	eng := New(nil)
	rng := NewRand(1)
	if got := eng.stubLLMValue(model.Decorator{Name: "llm"}, rng); got == "" {
		t.Fatal("stub value should not be empty")
	}
	values := eng.llmValuesExpand(model.Decorator{Name: "llm_values", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: int64(0), Raw: "0"}}}, rng)
	if len(values) != 1 || values[0] == "" {
		t.Fatalf("llm values fallback: %v", values)
	}
	values = eng.llmValuesExpand(model.Decorator{Name: "llm_values", Args: []model.DecoratorArg{{Kind: model.ArgLiteral, Literal: float64(2), Raw: "2.0"}}}, rng)
	if len(values) != 2 {
		t.Fatalf("float count llm values: %v", values)
	}
}
