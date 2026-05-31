package datjit

import (
	"bytes"
	"os"
	"strings"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

// GenerateString parses, validates, and generates a dataset from an in-memory
// schema string.
func GenerateString(schema string, opts ...Option) (*value.Dataset, *model.Document, error) {
	svc, err := New(opts...)
	if err != nil {
		return nil, nil, err
	}
	doc, err := svc.Parse(strings.NewReader(schema), "schema")
	if err != nil {
		return nil, nil, err
	}
	if err := svc.Validate(doc); err != nil {
		return nil, doc, err
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		return nil, doc, err
	}
	return ds, doc, nil
}

// GenerateMapString returns generated data from schema as plain Go maps.
func GenerateMapString(schema string, opts ...Option) (map[string][]map[string]any, error) {
	ds, _, err := GenerateString(schema, opts...)
	if err != nil {
		return nil, err
	}
	return DatasetMap(ds), nil
}

// GenerateRowsString returns generated rows for one entity from schema.
func GenerateRowsString(schema, entity string, opts ...Option) ([]map[string]any, error) {
	ds, _, err := GenerateString(schema, opts...)
	if err != nil {
		return nil, err
	}
	rows, ok := ds.Entities.Get(entity)
	if !ok {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "unknown entity: " + entity}
	}
	return RowsMap(rows), nil
}

// GenerateJSONString returns generated JSON bytes from schema.
func GenerateJSONString(schema string, opts ...Option) ([]byte, error) {
	ds, doc, err := GenerateString(schema, opts...)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := NewDefault().Write(ds, doc, "json", &buf, WriteOpts{Pretty: true}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GenerateMapFile opens path and returns generated data as plain Go maps.
func GenerateMapFile(path string, opts ...Option) (map[string][]map[string]any, error) {
	ds, _, err := GenerateFile(path, opts...)
	if err != nil {
		return nil, err
	}
	return DatasetMap(ds), nil
}

// GenerateRowsFile opens path and returns generated rows for one entity.
func GenerateRowsFile(path, entity string, opts ...Option) ([]map[string]any, error) {
	ds, _, err := GenerateFile(path, opts...)
	if err != nil {
		return nil, err
	}
	rows, ok := ds.Entities.Get(entity)
	if !ok {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "unknown entity: " + entity}
	}
	return RowsMap(rows), nil
}

// GenerateJSONFile opens path and returns generated JSON bytes.
func GenerateJSONFile(path string, opts ...Option) ([]byte, error) {
	ds, doc, err := GenerateFile(path, opts...)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := NewDefault().Write(ds, doc, "json", &buf, WriteOpts{Pretty: true}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteJSONFile writes generated JSON from schemaPath to outputPath.
func WriteJSONFile(outputPath, schemaPath string, opts ...Option) error {
	raw, err := GenerateJSONFile(schemaPath, opts...)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, raw, 0o644)
}

// WriteFile writes generated data from schemaPath to outputPath in format.
func WriteFile(outputPath, schemaPath, format string, opts ...Option) error {
	svc, err := New(opts...)
	if err != nil {
		return err
	}
	docBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return &errors.Error{Kind: errors.KindIO, Message: "open " + schemaPath + ": " + err.Error(), Cause: err}
	}
	doc, err := svc.Parse(bytes.NewReader(docBytes), schemaPath)
	if err != nil {
		return err
	}
	if err := svc.Validate(doc); err != nil {
		return err
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return svc.Write(ds, doc, format, f, WriteOpts{Pretty: true})
}

// GenerateFile is the package-level convenience form of Service.GenerateFile.
func GenerateFile(path string, opts ...Option) (*value.Dataset, *model.Document, error) {
	svc, err := New(opts...)
	if err != nil {
		return nil, nil, err
	}
	docBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, &errors.Error{Kind: errors.KindIO, Message: "open " + path + ": " + err.Error(), Cause: err}
	}
	doc, err := svc.Parse(bytes.NewReader(docBytes), path)
	if err != nil {
		return nil, nil, err
	}
	if err := svc.Validate(doc); err != nil {
		return nil, doc, err
	}
	ds, err := svc.Generate(doc)
	if err != nil {
		return nil, doc, err
	}
	return ds, doc, nil
}
