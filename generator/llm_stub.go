package generator

import (
	"strconv"
	"strings"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

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
