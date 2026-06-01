package errors

import (
	goerrors "errors"
	"testing"
)

func TestErrorFormat(t *testing.T) {
	e := &Error{Kind: KindParse, Message: "bad syntax", Location: &Location{File: "x.yaml", Line: 3, Col: 5}}
	want := "parse error at x.yaml:3:5: bad syntax"
	if got := e.Error(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestErrorFormatEntityField(t *testing.T) {
	e := &Error{Kind: KindGeneration, Entity: "User", Field: "email", Message: "boom"}
	got := e.Error()
	want := "generation error [User.email]: boom"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestErrorUnwrap(t *testing.T) {
	base := goerrors.New("io failure")
	e := &Error{Kind: KindIO, Message: "read", Cause: base}
	if !goerrors.Is(e, base) {
		t.Fatal("Is(base) should be true")
	}
}

func TestSentinels(t *testing.T) {
	e := &Error{Kind: KindUniquenessExhausted, Entity: "User", Field: "email"}
	if !goerrors.Is(e, ErrUniquenessExhausted) {
		t.Fatal("sentinel match failed")
	}
	if goerrors.Is(e, ErrParse) {
		t.Fatal("wrong sentinel matched")
	}
}

func TestErrorKindStringsAndHelpers(t *testing.T) {
	kinds := []ErrorKind{
		KindUnknown,
		KindParse,
		KindValidation,
		KindGeneration,
		KindUniquenessExhausted,
		KindRuleViolated,
		KindIO,
		KindFeatureDeferred,
		KindCorpusMissing,
		KindCyclicDependency,
	}
	for _, k := range kinds {
		if k.String() == "" {
			t.Fatalf("empty string for %v", int(k))
		}
	}
	err := Generationf("bad %s", "value")
	if err.Kind != KindGeneration || err.Message != "bad value" {
		t.Fatalf("Generationf: %+v", err)
	}
}

func TestParsefValidationf(t *testing.T) {
	p := Parsef(&Location{File: "f", Line: 1, Col: 2}, "bad %s", "token")
	if p.Kind != KindParse || p.Message != "bad token" {
		t.Fatalf("Parsef: %+v", p)
	}
	v := Validationf("field %q missing", "x")
	if v.Kind != KindValidation || v.Message != `field "x" missing` {
		t.Fatalf("Validationf: %+v", v)
	}
}

func TestWrap(t *testing.T) {
	base := goerrors.New("boom")
	w := Wrap(KindGeneration, base, "while processing %s", "users")
	if !goerrors.Is(w.Cause, base) || w.Kind != KindGeneration {
		t.Fatalf("Wrap: %+v", w)
	}
	if !goerrors.Is(w, base) {
		t.Fatal("wrapped cause should match")
	}
}
