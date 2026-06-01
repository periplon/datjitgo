package datjit_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	datjit "github.com/periplon/datjitgo"
	derrs "github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

type testWriter struct{ format string }

func (w testWriter) Format() string { return w.format }
func (w testWriter) Write(_ *value.Dataset, _ *model.Document, out io.Writer, _ ports.WriteOptions) error {
	_, err := out.Write([]byte("custom"))
	return err
}

type testGenerator struct{}

func (testGenerator) Generate(*model.Document, ports.GenerateOptions) (*value.Dataset, error) {
	return value.NewDataset(), nil
}

type testLLMProvider struct{}

func (testLLMProvider) Complete(context.Context, ports.LLMRequest) (string, error) {
	return "ok", nil
}

func TestOptionNilAdaptersReturnErrors(t *testing.T) {
	if _, err := datjit.New(datjit.WithParser(nil)); err == nil {
		t.Fatal("expected nil parser error")
	}
	if _, err := datjit.New(datjit.WithGenerator(nil)); err == nil {
		t.Fatal("expected nil generator error")
	}
	if _, err := datjit.New(datjit.WithWriter(nil)); err == nil {
		t.Fatal("expected nil writer error")
	}
	if _, err := datjit.New(datjit.WithCorpus(nil)); err == nil {
		t.Fatal("expected nil corpus error")
	}
	if _, err := datjit.New(datjit.WithLLMProvider(nil)); err == nil {
		t.Fatal("expected nil llm provider error")
	}
}

func TestOptionsVolumeLocaleWriterAndGenerateFile(t *testing.T) {
	svc, err := datjit.New(
		datjit.WithLocale("en-US"),
		datjit.WithVolume(map[string]int{"User": 1}),
		datjit.WithWriter(testWriter{format: "custom"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(svc.Formats(), "custom") {
		t.Fatalf("custom writer not registered: %v", svc.Formats())
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(path, []byte(miniSchema), 0o600); err != nil {
		t.Fatal(err)
	}
	ds, doc, err := svc.GenerateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := ds.Entities.Get("User")
	if len(rows) != 1 {
		t.Fatalf("volume override rows=%d", len(rows))
	}
	var out strings.Builder
	if err := svc.Write(ds, doc, "custom", &out, datjit.WriteOpts{}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "custom" {
		t.Fatalf("custom writer output %q", out.String())
	}
	if svc.Corpus() == nil {
		t.Fatal("default corpus should be exposed")
	}
	if _, _, err := svc.GenerateFile(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("expected missing file error")
	}

	svc, err = datjit.New(datjit.WithVolume(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Parse(nil, "nil.yaml"); !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("Parse nil reader: %v", err)
	}
	if err := svc.Write(value.NewDataset(), nil, "json", nil, datjit.WriteOpts{}); !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("Write nil writer: %v", err)
	}
	if err := svc.Write(value.NewDataset(), nil, "missing", io.Discard, datjit.WriteOpts{}); !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("Write unknown format: %v", err)
	}

	if _, err := datjit.New(datjit.WithLLMProvider(testLLMProvider{}), datjit.WithGenerator(testGenerator{})); err != nil {
		t.Fatalf("custom generator/llm options: %v", err)
	}
}

func TestNilServiceMethodsReturnValidationErrors(t *testing.T) {
	var svc *datjit.Service
	if _, err := svc.Parse(strings.NewReader(""), ""); !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("Parse nil service: %v", err)
	}
	if _, err := svc.Generate(model.NewDocument()); !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("Generate nil service: %v", err)
	}
	if err := svc.Write(value.NewDataset(), nil, "json", io.Discard, datjit.WriteOpts{}); !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("Write nil service: %v", err)
	}
	if _, _, err := svc.GenerateFile("missing"); !errors.Is(err, derrs.ErrValidation) {
		t.Fatalf("GenerateFile nil service: %v", err)
	}
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
