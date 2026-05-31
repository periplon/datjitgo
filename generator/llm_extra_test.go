package generator

import (
	"context"
	"testing"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
)

type recordingLLMProvider struct {
	prompts []string
}

func (p *recordingLLMProvider) Complete(_ context.Context, req ports.LLMRequest) (string, error) {
	p.prompts = append(p.prompts, req.Prompt)
	return "live " + req.Prompt, nil
}

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

func TestGenerateLLMValuesUsesProviderAndPromptOverrides(t *testing.T) {
	provider := &recordingLLMProvider{}
	eng := New(nil).WithLLMProvider(provider)
	values, err := eng.generateLLMValues(model.Decorator{
		Name: "llm_values",
		Args: []model.DecoratorArg{
			{Kind: model.ArgLiteral, Literal: int64(2), Raw: "2"},
			{Kind: model.ArgLiteral, Literal: "pick option", Raw: `"pick option"`},
			{Kind: model.ArgKV, Key: "model", Value: `"test-model"`},
		},
	}, &generationState{}, NewRand(4))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || len(provider.prompts) != 2 {
		t.Fatalf("values=%v prompts=%v", values, provider.prompts)
	}
	if provider.prompts[0] == provider.prompts[1] || provider.prompts[0] == "" {
		t.Fatalf("prompts were not individualized: %v", provider.prompts)
	}
}

func TestWithLLMValuesArgsRewritesDecorator(t *testing.T) {
	dec := withLLMValuesArgs(model.Decorator{Name: "llm_values"}, 3, "choose")
	if dec.Name != "llm_values" || len(dec.Args) != 2 {
		t.Fatalf("decorator = %#v", dec)
	}
	if dec.Args[0].Raw != "3" || dec.Args[1].Literal != "choose" {
		t.Fatalf("args = %#v", dec.Args)
	}
}
