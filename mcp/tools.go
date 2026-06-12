package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	datjit "github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/model"
	djruntime "github.com/periplon/datjitgo/runtime"
)

// Input caps protecting the server from pathological requests. They are part
// of the tool contract documented in the spec (§3).
const (
	// maxSchemaBytes caps the size of a DDL schema string accepted by a tool.
	maxSchemaBytes = 512 << 10
	// maxTotalRows caps the total number of rows a single generate call may
	// request across all entities.
	maxTotalRows = 100_000
	// maxSampleCount caps the number of values sample may return.
	maxSampleCount = 100
	// defaultSampleCount is the sample count when the caller omits it.
	defaultSampleCount = 5
)

// toolError is a sentinel-style error type whose text is surfaced to the
// client as a tool result with isError:true rather than as a JSON-RPC error.
// It is used for input the agent can fix (bad schema, unknown semantic,
// exceeded caps) — distinct from protocol errors (unknown method/tool).
type toolError struct{ msg string }

func (e *toolError) Error() string { return e.msg }

// toolErrorf constructs a toolError from a printf-style format string.
func toolErrorf(format string, a ...any) error {
	return &toolError{msg: fmt.Sprintf(format, a...)}
}

// toolDef describes one MCP tool: its name, human description, JSON Schema for
// inputs, and the handler that turns decoded params into a text payload.
type toolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
	// handle runs the tool. It returns the text payload on success; a
	// *toolError is rendered as an isError result, any other error becomes a
	// JSON-RPC error.
	handle func(ctx context.Context, svc *datjit.Service, rt djruntime.Runtime, params json.RawMessage) (string, error)
}

// registry is the ordered set of tools exposed over MCP.
type registry struct {
	tools  []toolDef
	byName map[string]*toolDef
}

// newRegistry builds the four-tool registry described in the spec (§3).
func newRegistry() *registry {
	reg := &registry{byName: map[string]*toolDef{}}
	reg.add(generateTool())
	reg.add(validateTool())
	reg.add(inspectTool())
	reg.add(sampleTool())
	return reg
}

// add appends t to the registry and indexes it by name.
func (r *registry) add(t toolDef) {
	r.tools = append(r.tools, t)
	r.byName[t.Name] = &r.tools[len(r.tools)-1]
}

// lookup returns the tool with the given name, or nil if unknown.
func (r *registry) lookup(name string) *toolDef {
	return r.byName[name]
}

// list returns the tools/list payload entries in registration order.
func (r *registry) list() []map[string]any {
	out := make([]map[string]any, 0, len(r.tools))
	for i := range r.tools {
		t := &r.tools[i]
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return out
}

// readString extracts a required string field from a decoded params map,
// enforcing the schema-size cap when capBytes > 0.
func readString(m map[string]any, key string, required bool, capBytes int) (string, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		if required {
			return "", toolErrorf("missing required %q", key)
		}
		return "", nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", toolErrorf("%q must be a string", key)
	}
	if capBytes > 0 && len(s) > capBytes {
		return "", toolErrorf("%q exceeds %d bytes", key, capBytes)
	}
	return s, nil
}

// readInt extracts an optional integer field. JSON numbers decode as float64;
// non-integral values are rejected.
func readInt(m map[string]any, key string) (int64, bool, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return 0, false, nil
	}
	f, ok := raw.(float64)
	if !ok {
		return 0, false, toolErrorf("%q must be an integer", key)
	}
	if f != float64(int64(f)) {
		return 0, false, toolErrorf("%q must be an integer", key)
	}
	return int64(f), true, nil
}

// decodeParams unmarshals params into a generic map. Absent params decode to an
// empty map so optional-only tools work with no arguments.
func decodeParams(params json.RawMessage) (map[string]any, error) {
	m := map[string]any{}
	if len(params) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(params, &m); err != nil {
		return nil, toolErrorf("invalid arguments: %v", err)
	}
	return m, nil
}

// parseSchema parses a DDL schema string into a document via the façade.
// Parse errors are surfaced as tool errors so agents can self-correct.
func parseSchema(svc *datjit.Service, schema string) (*model.Document, error) {
	doc, err := svc.Parse(strings.NewReader(schema), "<mcp>")
	if err != nil {
		return nil, &toolError{msg: err.Error()}
	}
	return doc, nil
}

// generateTool implements the `generate` tool (spec §3.1).
func generateTool() toolDef {
	return toolDef{
		Name: "generate",
		Description: "Generate a deterministic synthetic dataset from a datjit DDL schema. " +
			"The schema is a YAML string. Output is one text block in the chosen format. " +
			"Generation is offline and seeded: pass seed to vary the output (default 0). " +
			"Minimal schema example:\n" +
			"domain: demo\nentities:\n  User:\n    id: uuid @primary\n    name: person.full\n    email: email",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"schema": map[string]any{
					"type":        "string",
					"description": "datjit DDL schema (YAML string)",
				},
				"format": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "csv", "ndjson", "yaml", "sql"},
					"description": "output format (default json)",
				},
				"seed": map[string]any{
					"type":        "integer",
					"description": "generation seed; same seed yields byte-identical output (default 0)",
				},
				"entity": map[string]any{
					"type":        "string",
					"description": "emit only rows for this entity",
				},
				"volumes": map[string]any{
					"type":                 "object",
					"description":          "per-entity row count overrides (entity name -> integer)",
					"additionalProperties": map[string]any{"type": "integer"},
				},
				"pretty": map[string]any{
					"type":        "boolean",
					"description": "human-friendly output where the format supports it",
				},
				"sql_dialect": map[string]any{
					"type":        "string",
					"enum":        []string{"postgres", "mysql", "sqlite"},
					"description": "SQL dialect when format is sql (default postgres)",
				},
			},
			"required": []string{"schema"},
		},
		handle: handleGenerate,
	}
}

// handleGenerate parses, validates, generates, and serialises per the request.
func handleGenerate(_ context.Context, svc *datjit.Service, _ djruntime.Runtime, params json.RawMessage) (string, error) {
	m, err := decodeParams(params)
	if err != nil {
		return "", err
	}
	schema, err := readString(m, "schema", true, maxSchemaBytes)
	if err != nil {
		return "", err
	}
	format, err := readString(m, "format", false, 0)
	if err != nil {
		return "", err
	}
	if format == "" {
		format = "json"
	}
	entity, err := readString(m, "entity", false, 0)
	if err != nil {
		return "", err
	}
	sqlDialect, err := readString(m, "sql_dialect", false, 0)
	if err != nil {
		return "", err
	}
	if sqlDialect == "" {
		sqlDialect = "postgres"
	}
	pretty := false
	if raw, ok := m["pretty"]; ok && raw != nil {
		b, ok := raw.(bool)
		if !ok {
			return "", toolErrorf("%q must be a boolean", "pretty")
		}
		pretty = b
	}
	volumes, err := readVolumes(m)
	if err != nil {
		return "", err
	}

	// Seed defaults to 0 (not time-based) for reproducible agent retries.
	seed := int64(0)
	if s, ok, err := readInt(m, "seed"); err != nil {
		return "", err
	} else if ok {
		seed = s
	}

	opts := []datjit.Option{datjit.WithSeed(seed)}
	if len(volumes) > 0 {
		opts = append(opts, datjit.WithVolume(volumes))
	}
	gsvc, err := datjit.New(opts...)
	if err != nil {
		return "", err
	}

	if !formatSupported(gsvc, format) {
		return "", toolErrorf("unknown format %q (available: %s)", format, strings.Join(gsvc.Formats(), ", "))
	}

	doc, err := parseSchema(gsvc, schema)
	if err != nil {
		return "", err
	}
	if entity != "" && !doc.Entities.Has(entity) {
		return "", toolErrorf("unknown entity %q (available: %s)", entity, strings.Join(doc.Entities.Keys(), ", "))
	}
	if total := plannedTotal(doc, volumes); total > maxTotalRows {
		return "", toolErrorf("requested volume %d exceeds cap %d", total, maxTotalRows)
	}
	if err := gsvc.Validate(doc); err != nil {
		return "", &toolError{msg: err.Error()}
	}

	ds, err := gsvc.Generate(doc)
	if err != nil {
		return "", &toolError{msg: err.Error()}
	}

	var buf strings.Builder
	wo := datjit.WriteOpts{
		Pretty:       pretty,
		SQLDialect:   sqlDialect,
		EntityFilter: entity,
	}
	if err := gsvc.Write(ds, doc, format, &buf, wo); err != nil {
		return "", &toolError{msg: err.Error()}
	}
	return buf.String(), nil
}

// readVolumes decodes the optional volumes object into a string->int map,
// rejecting non-integral or negative counts.
func readVolumes(m map[string]any) (map[string]int, error) {
	raw, ok := m["volumes"]
	if !ok || raw == nil {
		return nil, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, toolErrorf("%q must be an object", "volumes")
	}
	out := make(map[string]int, len(obj))
	for k, v := range obj {
		f, ok := v.(float64)
		if !ok || f != float64(int64(f)) {
			return nil, toolErrorf("volume for %q must be an integer", k)
		}
		if f < 0 {
			return nil, toolErrorf("volume for %q must not be negative", k)
		}
		out[k] = int(f)
	}
	return out, nil
}

// plannedTotal sums the effective per-entity volumes the way Generate would,
// honouring overrides, document volumes, then the default of 10.
func plannedTotal(doc *model.Document, override map[string]int) int {
	total := 0
	doc.Entities.Each(func(name string, _ *model.Entity) bool {
		if v, ok := override[name]; ok {
			total += v
			return true
		}
		if v, ok := doc.Volume[name]; ok {
			if v.Exact > 0 {
				total += v.Exact
			} else if v.Min != 0 || v.Max != 0 {
				total += (v.Min + v.Max) / 2
			} else {
				total += 10
			}
			return true
		}
		total += 10
		return true
	})
	return total
}

// formatSupported reports whether format is registered with svc.
func formatSupported(svc *datjit.Service, format string) bool {
	for _, name := range svc.Formats() {
		if name == format {
			return true
		}
	}
	return false
}

// validateTool implements the `validate` tool (spec §3.2).
func validateTool() toolDef {
	return toolDef{
		Name: "validate",
		Description: "Parse and statically validate a datjit DDL schema (YAML string). " +
			"Returns \"schema is valid (N entities)\" or the parse/validation diagnostic " +
			"with its location. Always a successful tool call (isError:false).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"schema": map[string]any{
					"type":        "string",
					"description": "datjit DDL schema (YAML string)",
				},
			},
			"required": []string{"schema"},
		},
		handle: handleValidate,
	}
}

// handleValidate parses and validates, returning a diagnostic string. It never
// returns a toolError: invalid schemas are a successful answer to the question.
func handleValidate(_ context.Context, svc *datjit.Service, _ djruntime.Runtime, params json.RawMessage) (string, error) {
	m, err := decodeParams(params)
	if err != nil {
		return "", err
	}
	schema, err := readString(m, "schema", true, maxSchemaBytes)
	if err != nil {
		return "", err
	}
	doc, err := svc.Parse(strings.NewReader(schema), "<mcp>")
	if err != nil {
		return err.Error(), nil
	}
	if err := svc.Validate(doc); err != nil {
		return err.Error(), nil
	}
	return fmt.Sprintf("schema is valid (%d entities)", doc.Entities.Len()), nil
}

// inspectTool implements the `inspect` tool (spec §3.3).
func inspectTool() toolDef {
	return toolDef{
		Name: "inspect",
		Description: "Summarise a datjit DDL schema as pretty JSON: entities, field counts, " +
			"dependencies, volumes, enums, and rules. No data is generated.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"schema": map[string]any{
					"type":        "string",
					"description": "datjit DDL schema (YAML string)",
				},
			},
			"required": []string{"schema"},
		},
		handle: handleInspect,
	}
}

// handleInspect parses then renders the inspection summary as pretty JSON.
func handleInspect(_ context.Context, svc *datjit.Service, _ djruntime.Runtime, params json.RawMessage) (string, error) {
	m, err := decodeParams(params)
	if err != nil {
		return "", err
	}
	schema, err := readString(m, "schema", true, maxSchemaBytes)
	if err != nil {
		return "", err
	}
	doc, err := parseSchema(svc, schema)
	if err != nil {
		return "", err
	}
	insp, err := svc.Inspect(doc)
	if err != nil {
		return "", &toolError{msg: err.Error()}
	}
	out, err := json.MarshalIndent(insp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// sampleTool implements the `sample` tool (spec §3.4).
func sampleTool() toolDef {
	return toolDef{
		Name: "sample",
		Description: "Sample values for a single datjit semantic type (e.g. \"email\", " +
			"\"person.full\", \"address.city\"). Returns a JSON array. Deterministic: " +
			"same seed yields the same array; pass seed to vary (default 0).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"semantic": map[string]any{
					"type":        "string",
					"description": "semantic type name, e.g. email or person.full",
				},
				"count": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     maxSampleCount,
					"description": "number of values to sample (1..100, default 5)",
				},
				"seed": map[string]any{
					"type":        "integer",
					"description": "base seed; same seed yields the same array (default 0)",
				},
			},
			"required": []string{"semantic"},
		},
		handle: handleSample,
	}
}

// handleSample generates count values for the semantic type, deriving a
// per-index seed from the base seed so the array is deterministic.
func handleSample(ctx context.Context, _ *datjit.Service, rt djruntime.Runtime, params json.RawMessage) (string, error) {
	m, err := decodeParams(params)
	if err != nil {
		return "", err
	}
	semantic, err := readString(m, "semantic", true, 0)
	if err != nil {
		return "", err
	}
	count := defaultSampleCount
	if c, ok, err := readInt(m, "count"); err != nil {
		return "", err
	} else if ok {
		if c < 1 || c > maxSampleCount {
			return "", toolErrorf("count must be between 1 and %d", maxSampleCount)
		}
		count = int(c)
	}
	base := int64(0)
	if s, ok, err := readInt(m, "seed"); err != nil {
		return "", err
	} else if ok {
		base = s
	}

	out := make([]any, 0, count)
	for i := 0; i < count; i++ {
		seed := base + int64(i)
		v, err := rt.GenerateValue(ctx, djruntime.ValueRequest{
			Semantic: semantic,
			Seed:     &seed,
		})
		if err != nil {
			return "", &toolError{msg: err.Error()}
		}
		out = append(out, datjit.ValueAny(v))
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
