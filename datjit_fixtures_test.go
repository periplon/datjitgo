package datjit_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/jmcarbo/datjitgo"
)

// update toggles writing golden files instead of comparing against them.
// Run `go test -run TestFixtures -update .` once, inspect the diff, commit.
var update = flag.Bool("update", false, "update golden files")

// uuidRegex matches canonical UUIDv4 strings (any variant nibble). Used to
// strip top-level identifier fields whose values vary across runs even with
// a fixed seed, because uuid.NewRandomFromReader mixes in wall-clock bits
// on some platforms.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func TestFixtures(t *testing.T) {
	matches, err := filepath.Glob("testdata/fixtures/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixtures found under testdata/fixtures")
	}
	for _, fx := range matches {
		name := strings.TrimSuffix(filepath.Base(fx), ".yaml")
		if strings.HasPrefix(name, "llm_") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			svc := datjit.NewDefault()
			b, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := svc.Parse(bytes.NewReader(b), fx)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			seed := int64(42)
			doc.Seed = &seed

			ds, err := svc.Generate(doc)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}

			var buf bytes.Buffer
			if err := svc.Write(ds, doc, "json", &buf, datjit.WriteOpts{Pretty: true}); err != nil {
				t.Fatalf("write: %v", err)
			}

			stripped, err := stripNondeterministicFields(buf.Bytes())
			if err != nil {
				t.Fatalf("strip: %v", err)
			}

			golden := filepath.Join("testdata/golden", name+".json")
			if *update {
				if err := os.MkdirAll("testdata/golden", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, stripped, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("golden missing (%v); run with -update to create", err)
			}
			if diff := cmp.Diff(string(want), string(stripped)); diff != "" {
				t.Fatalf("fixture drift:\n%s", diff)
			}
		})
	}
}

// stripNondeterministicFields walks the JSON payload preserving key order
// and removes any "id" field whose value is a canonical UUID. The input is
// expected to be a pretty-printed JSON object (the top-level shape the
// facade emits). Output is re-encoded with 2-space indent to match.
func stripNondeterministicFields(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, err := decodeOrdered(dec)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	// Ensure we consumed the whole payload.
	if _, err := dec.Token(); err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("trailing data: %w", err)
	}
	stripIDs(v)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}

// orderedMap is a JSON object that preserves insertion order across a
// decode/encode round trip. We use it instead of map[string]any so the
// golden file's key ordering survives intact.
type orderedMap struct {
	keys   []string
	values map[string]any
}

func newOrderedMap() *orderedMap {
	return &orderedMap{values: map[string]any{}}
}

func (o *orderedMap) set(k string, v any) {
	if _, ok := o.values[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.values[k] = v
}

func (o *orderedMap) delete(k string) {
	if _, ok := o.values[k]; !ok {
		return
	}
	delete(o.values, k)
	for i, kk := range o.keys {
		if kk == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			return
		}
	}
}

// MarshalJSON emits the map in insertion order. Kept minimal — json.Marshal
// takes care of each value's serialisation recursively.
func (o *orderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(o.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// decodeOrdered reads a single JSON value from dec and returns it as a Go
// value where every object is an *orderedMap. Arrays are []any; scalars are
// passed through verbatim.
func decodeOrdered(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			m := newOrderedMap()
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("expected string key, got %T", keyTok)
				}
				val, err := decodeOrdered(dec)
				if err != nil {
					return nil, err
				}
				m.set(k, val)
			}
			// consume the closing '}'
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return m, nil
		case '[':
			var arr []any
			for dec.More() {
				v, err := decodeOrdered(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			// consume the closing ']'
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
	}
	return tok, nil
}

// stripIDs walks v (which may be an *orderedMap, []any, or scalar) and
// removes any "id" field whose value is a canonical UUID string. Every
// nested object is visited so ids inside embedded references are also
// removed. Ordering of surviving keys is preserved.
func stripIDs(v any) {
	switch vv := v.(type) {
	case *orderedMap:
		if raw, ok := vv.values["id"]; ok {
			if s, ok := raw.(string); ok && uuidRegex.MatchString(s) {
				vv.delete("id")
			}
		}
		for _, k := range vv.keys {
			stripIDs(vv.values[k])
		}
	case []any:
		for _, item := range vv {
			stripIDs(item)
		}
	}
}
