package generator

import (
	"context"
	"testing"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
	"github.com/periplon/datjitgo/corpus"
)

type recordingLLM struct {
	requests []ports.LLMRequest
}

func (r *recordingLLM) Complete(_ context.Context, req ports.LLMRequest) (string, error) {
	r.requests = append(r.requests, req)
	return "live:" + req.Prompt, nil
}

func TestEngineLiveLLMUsesProviderAndDecoratorOverrides(t *testing.T) {
	temp := 0.2
	maxTokens := 20
	llm := &recordingLLM{}
	doc := model.NewDocument()
	doc.Generation.LLM = &model.LLMConfig{
		Provider:    "openai",
		Endpoint:    "http://example.test/v1",
		Model:       "base",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}
	doc.Volume["Post"] = model.VolumeSpec{Exact: 1}
	ent := model.NewEntity("Post")
	ent.Fields.Set("title", &model.Field{
		Name: "title",
		Type: model.Primitive{Kind: model.PrimString},
		Decorators: []model.Decorator{{Name: "llm", Args: []model.DecoratorArg{
			{Kind: model.ArgLiteral, Raw: `"write title"`, Literal: "write title"},
			{Kind: model.ArgKV, Key: "model", Value: "override"},
			{Kind: model.ArgKV, Key: "temperature", Value: "0.9"},
		}}},
	})
	doc.Entities.Set("Post", ent)

	ds, err := New(corpus.NewEmbedded()).WithLLMProvider(llm).Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := ds.Entities.Get("Post")
	got, _ := rows[0].Get("title")
	if got.S != "live:write title" {
		t.Fatalf("value = %q", got.S)
	}
	if len(llm.requests) != 1 {
		t.Fatalf("requests = %d", len(llm.requests))
	}
	req := llm.requests[0]
	if req.Provider != "openai" || req.Endpoint != "http://example.test/v1" || req.Model != "override" || req.Prompt != "write title" {
		t.Fatalf("request = %+v", req)
	}
	if req.Temperature == nil || *req.Temperature != 0.9 || req.MaxTokens == nil || *req.MaxTokens != 20 {
		t.Fatalf("request knobs = %+v", req)
	}
}

func TestEngineLiveLLMValuesUsesProvider(t *testing.T) {
	llm := &recordingLLM{}
	doc := model.NewDocument()
	doc.Generation.LLM = &model.LLMConfig{Provider: "lmstudio", Endpoint: "http://localhost:1234/v1", Model: "local"}
	doc.Volume["Post"] = model.VolumeSpec{Exact: 1}
	ent := model.NewEntity("Post")
	ent.Fields.Set("category", &model.Field{
		Name: "category",
		Type: model.Primitive{Kind: model.PrimString},
		Decorators: []model.Decorator{{Name: "llm_values", Args: []model.DecoratorArg{
			{Kind: model.ArgLiteral, Raw: "2", Literal: int64(2)},
			{Kind: model.ArgLiteral, Raw: `"category"`, Literal: "category"},
		}}},
	})
	doc.Entities.Set("Post", ent)

	ds, err := New(corpus.NewEmbedded()).WithLLMProvider(llm).Generate(doc, ports.GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := ds.Entities.Get("Post")
	got, _ := rows[0].Get("category")
	if got.Kind != value.KindString || got.S == "" {
		t.Fatalf("value = %+v", got)
	}
	if len(llm.requests) != 2 {
		t.Fatalf("requests = %d", len(llm.requests))
	}
}
