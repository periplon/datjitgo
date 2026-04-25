package generator

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// generateLLMValue returns live provider output when an LLM provider has been
// configured. Otherwise it preserves the deterministic offline stub behavior.
func (e *Engine) generateLLMValue(d model.Decorator, st *generationState, rng ports.Randomizer) (string, error) {
	if e.llm == nil {
		return e.stubLLMValue(d, rng), nil
	}
	req, err := buildLLMRequest(st.llmConfig, d)
	if err != nil {
		return "", err
	}
	text, err := e.llm.Complete(context.Background(), req)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", &errors.Error{Kind: errors.KindGeneration, Message: "llm provider returned empty content"}
	}
	return text, nil
}

// generateLLMValues returns N live completions when configured, or N
// deterministic stub values otherwise.
func (e *Engine) generateLLMValues(d model.Decorator, st *generationState, rng ports.Randomizer) ([]string, error) {
	n, prompt := llmValuesArgs(d)
	if e.llm == nil {
		return e.llmValuesExpand(*withLLMValuesArgs(d, n, prompt), rng), nil
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		item := d
		item.Args = overrideLLMPrompt(item.Args, fmt.Sprintf("%s\nReturn option %d of %d.", prompt, i+1, n))
		text, err := e.generateLLMValue(item, st, rng.Substream("llm_values:item:"+strconv.Itoa(i)))
		if err != nil {
			return nil, err
		}
		out = append(out, text)
	}
	return out, nil
}

func buildLLMRequest(cfg *model.LLMConfig, d model.Decorator) (ports.LLMRequest, error) {
	var req ports.LLMRequest
	if cfg != nil {
		req.Provider = cfg.Provider
		req.Endpoint = cfg.Endpoint
		req.Model = cfg.Model
		req.APIKey = cfg.APIKey
		req.Temperature = cfg.Temperature
		req.MaxTokens = cfg.MaxTokens
		req.TimeoutSecs = cfg.TimeoutSecs
	}
	if len(d.Args) > 0 {
		req.Prompt = decoratorLiteralString(d.Args[0])
	}
	for _, arg := range d.Args[1:] {
		if arg.Kind != model.ArgKV {
			continue
		}
		val := strings.TrimSpace(arg.Value)
		switch strings.ToLower(arg.Key) {
		case "provider":
			req.Provider = stripQuotes(val)
		case "endpoint":
			req.Endpoint = stripQuotes(val)
		case "model":
			req.Model = stripQuotes(val)
		case "api_key":
			req.APIKey = stripQuotes(val)
		case "temperature":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return req, &errors.Error{Kind: errors.KindValidation, Message: "invalid llm temperature: " + val, Cause: err}
			}
			req.Temperature = &f
		case "max_tokens":
			n, err := strconv.Atoi(val)
			if err != nil {
				return req, &errors.Error{Kind: errors.KindValidation, Message: "invalid llm max_tokens: " + val, Cause: err}
			}
			req.MaxTokens = &n
		case "timeout_secs":
			n, err := strconv.Atoi(val)
			if err != nil {
				return req, &errors.Error{Kind: errors.KindValidation, Message: "invalid llm timeout_secs: " + val, Cause: err}
			}
			req.TimeoutSecs = &n
		}
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return req, &errors.Error{Kind: errors.KindValidation, Message: "@llm prompt is required"}
	}
	return req, nil
}

func llmValuesArgs(d model.Decorator) (int, string) {
	n := 5
	prompt := ""
	if len(d.Args) > 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(d.Args[0].Raw)); err == nil {
			n = v
		} else if f, err := strconv.ParseFloat(strings.TrimSpace(d.Args[0].Raw), 64); err == nil {
			n = int(f)
		}
	}
	if len(d.Args) > 1 {
		prompt = decoratorLiteralString(d.Args[1])
	}
	if n <= 0 {
		n = 1
	}
	return n, prompt
}

func overrideLLMPrompt(args []model.DecoratorArg, prompt string) []model.DecoratorArg {
	out := make([]model.DecoratorArg, 0, len(args))
	out = append(out, model.DecoratorArg{Kind: model.ArgLiteral, Raw: strconv.Quote(prompt), Literal: prompt})
	if len(args) > 2 {
		out = append(out, args[2:]...)
	}
	return out
}

func withLLMValuesArgs(d model.Decorator, n int, prompt string) *model.Decorator {
	d.Args = []model.DecoratorArg{
		{Kind: model.ArgLiteral, Raw: strconv.Itoa(n), Literal: int64(n)},
		{Kind: model.ArgLiteral, Raw: strconv.Quote(prompt), Literal: prompt},
	}
	return &d
}

func stripQuotes(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// preprocessLLM normalises entity-level @llm decorators in-place for legacy
// callers that explicitly invoke it. Generate intentionally avoids this helper
// so parsed documents remain immutable across generation runs.
func preprocessLLM(doc *model.Document) {
	if doc == nil || doc.Entities == nil {
		return
	}
	doc.Entities.Each(func(_ string, ent *model.Entity) bool {
		entityLevelLLM := findLLM(ent.Meta)
		ent.Fields.Each(func(_ string, f *model.Field) bool {
			if entityLevelLLM != nil && shouldInheritEntityLLM(f) {
				f.Decorators = append(f.Decorators, *entityLevelLLM)
			}
			return true
		})
		return true
	})
}

// findLLM returns the first @llm decorator in decs, or nil if absent.
func findLLM(decs []model.Decorator) *model.Decorator {
	for i := range decs {
		if decs[i].Name == "llm" {
			return &decs[i]
		}
	}
	return nil
}

// shouldInheritEntityLLM is true when f is a bare `string` field with no
// competing value-producing decorator and is not a primary/auto/reference.
func shouldInheritEntityLLM(f *model.Field) bool {
	if _, ok := f.Type.(model.Reference); ok {
		return false
	}
	if p, ok := f.Type.(model.Primitive); !ok || p.Kind != model.PrimString {
		return false
	}
	blockers := []string{"llm", "pattern", "values", "derived", "primary", "auto"}
	for _, name := range blockers {
		if model.HasDecorator(f.Decorators, name) {
			return false
		}
	}
	if f.DefaultChain != nil || len(f.Compute) > 0 {
		return false
	}
	return true
}

// stubLLMValue returns a deterministic stub for an @llm(...) decoration.
// The decorator's first literal argument (prompt) is folded into an RNG
// substream so different prompts yield different (but reproducible)
// samples.
func (e *Engine) stubLLMValue(d model.Decorator, rng ports.Randomizer) string {
	prompt := ""
	if len(d.Args) > 0 {
		prompt = decoratorLiteralString(d.Args[0])
	}
	scope := "llm:" + prompt
	r := rng.Substream(scope)
	return e.sampleLLMSentence(r)
}

// sampleLLMSentence pulls one sentence from the corpus text.sentences
// pool. Falls back to a built-in lorem sentence if the corpus is
// unavailable — keeps generator output non-empty even with a nil corpus.
func (e *Engine) sampleLLMSentence(r ports.Randomizer) string {
	if e.corpus != nil {
		ctx := ports.SampleContext{Locale: e.locale, RNG: r}
		if v, err := e.corpus.Sample(ctx, "text.sentences"); err == nil && v.Kind == value.KindString {
			return v.S
		}
	}
	return fallbackLorem(r)
}

// fallbackLorem is the last-resort stub when no corpus is present.
func fallbackLorem(r ports.Randomizer) string {
	pool := []string{
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		"Suspendisse iaculis blandit nibh, vitae posuere urna aliquet vitae.",
		"Vivamus ac massa at arcu tempor facilisis eu non lacus.",
		"Aliquam tincidunt, ligula eget fermentum cursus, mi felis eleifend neque.",
	}
	return pool[int(r.IntN(int64(len(pool))))]
}

// llmValuesExpand materialises the N stub candidates an @llm_values
// decorator should hand to the @values pipeline. Called lazily from
// generateField so the RNG substream matches generation-time scope.
func (e *Engine) llmValuesExpand(d model.Decorator, rng ports.Randomizer) []string {
	n := 5
	prompt := ""
	if len(d.Args) > 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(d.Args[0].Raw)); err == nil {
			n = v
		} else if f, err := strconv.ParseFloat(strings.TrimSpace(d.Args[0].Raw), 64); err == nil {
			n = int(f)
		}
	}
	if len(d.Args) > 1 {
		prompt = decoratorLiteralString(d.Args[1])
	}
	if n <= 0 {
		n = 1
	}
	scope := "llm_values:" + prompt
	r := rng.Substream(scope)
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, e.sampleLLMSentence(r.Substream("item:"+strconv.Itoa(i))))
	}
	return out
}
