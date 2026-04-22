package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
	"github.com/jmcarbo/datjitgo/corpus"
)

func newTestEngine() *Engine {
	return New(corpus.NewEmbedded())
}

func TestSemanticEmailShape(t *testing.T) {
	eng := newTestEngine()
	re := regexp.MustCompile(`^[a-z0-9._-]+@[a-z0-9.-]+\.[a-z]+$`)
	rng := NewRand(42)
	for i := 0; i < 50; i++ {
		v, err := eng.generateSemantic(model.Semantic{Namespace: "email"}, rng)
		if err != nil {
			t.Fatal(err)
		}
		if v.Kind != value.KindString {
			t.Fatalf("email not string: %v", v)
		}
		if !re.MatchString(strings.ToLower(v.S)) {
			t.Fatalf("email malformed: %q", v.S)
		}
	}
}

func TestSemanticPersonFullHasSpace(t *testing.T) {
	eng := newTestEngine()
	rng := NewRand(3)
	for i := 0; i < 20; i++ {
		v, err := eng.generateSemantic(model.Semantic{Namespace: "person", Tag: "full"}, rng)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(v.S, " ") {
			t.Fatalf("full name lacks space: %q", v.S)
		}
	}
}

func TestSemanticColorHex(t *testing.T) {
	eng := newTestEngine()
	re := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	rng := NewRand(5)
	for i := 0; i < 20; i++ {
		v, err := eng.generateSemantic(model.Semantic{Namespace: "color", Tag: "hex"}, rng)
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(v.S) {
			t.Fatalf("color.hex malformed: %q", v.S)
		}
	}
}

func TestSemanticPhoneShape(t *testing.T) {
	eng := newTestEngine()
	re := regexp.MustCompile(`^\+1-\d{3}-\d{3}-\d{4}$`)
	rng := NewRand(9)
	for i := 0; i < 20; i++ {
		v, err := eng.generateSemantic(model.Semantic{Namespace: "phone"}, rng)
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(v.S) {
			t.Fatalf("phone malformed: %q", v.S)
		}
	}
}

func TestSemanticIPv4(t *testing.T) {
	eng := newTestEngine()
	re := regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
	rng := NewRand(11)
	for i := 0; i < 10; i++ {
		v, err := eng.generateSemantic(model.Semantic{Namespace: "ipv4"}, rng)
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(v.S) {
			t.Fatalf("ipv4 malformed: %q", v.S)
		}
	}
}

func TestSemanticMD5(t *testing.T) {
	eng := newTestEngine()
	rng := NewRand(13)
	v, err := eng.generateSemantic(model.Semantic{Namespace: "hash", Tag: "md5"}, rng)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.S) != 32 {
		t.Fatalf("md5 length: %d", len(v.S))
	}
}

func TestSemanticTimezoneFromCorpus(t *testing.T) {
	eng := newTestEngine()
	rng := NewRand(17)
	v, err := eng.generateSemantic(model.Semantic{Namespace: "timezone"}, rng)
	if err != nil {
		t.Fatal(err)
	}
	if v.S == "" {
		t.Fatalf("empty timezone")
	}
}
