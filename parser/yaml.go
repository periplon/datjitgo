package parser

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	derrs "github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
)

// parseDocument walks a decoded yaml.Node tree and builds a *model.Document.
// It is separated from Parser.Parse so tests can drive it with a pre-built
// node if needed.
func parseDocument(name string, root *yaml.Node) (*model.Document, error) {
	top := topMapping(root)
	if top == nil {
		return nil, locErr(name, root, "expected YAML mapping at document root")
	}

	doc := model.NewDocument()

	// Track seen keys so we can handle key ordering deterministically.
	for i := 0; i+1 < len(top.Content); i += 2 {
		k := top.Content[i]
		v := top.Content[i+1]
		key := k.Value
		switch key {
		case "domain":
			doc.Domain = scalarString(v)
		case "version":
			doc.Version = scalarString(v)
		case "seed":
			n, err := scalarInt64(v)
			if err != nil {
				return nil, locErr(name, v, "invalid seed: %v", err)
			}
			seed := n
			doc.Seed = &seed
		case "locale":
			doc.Locale = scalarString(v)
		case "volume":
			if err := parseVolume(name, v, doc.Volume); err != nil {
				return nil, err
			}
		case "enums":
			if err := parseEnums(name, v, doc.Enums); err != nil {
				return nil, err
			}
		case "types":
			if err := parseTypes(name, v, doc.Types); err != nil {
				return nil, err
			}
		case "entities":
			if err := parseEntities(name, v, doc.Entities); err != nil {
				return nil, err
			}
		case "rules":
			rules, err := parseRules(name, v)
			if err != nil {
				return nil, err
			}
			doc.Rules = rules
		case "tools":
			tools, err := parseTools(name, v)
			if err != nil {
				return nil, err
			}
			doc.Tools = tools
		case "generation":
			gen, err := parseGeneration(name, v)
			if err != nil {
				return nil, err
			}
			doc.Generation = gen
		default:
			// Unknown top-level keys are permitted for forward compatibility
			// (e.g. mcp_tools in the Rust code). Silently ignoring keeps the
			// parser resilient to future extensions.
		}
	}

	if doc.Domain == "" {
		return nil, locErr(name, root, "missing required field: domain")
	}
	if doc.Locale == "" {
		doc.Locale = "en-US"
	}
	// Propagate the top-level seed to the generation config if not set there.
	if doc.Seed != nil && doc.Generation.Seed == nil {
		seed := *doc.Seed
		doc.Generation.Seed = &seed
	}

	return doc, nil
}

// topMapping returns the mapping node at the top of a decoded document.
// yaml.v3 wraps the actual document in a DocumentNode with a single Content
// entry.
func topMapping(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	n := root
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

// scalarString returns the string value of a scalar node, or "" otherwise.
func scalarString(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

func scalarInt64(n *yaml.Node) (int64, error) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf("expected integer scalar")
	}
	return strconv.ParseInt(n.Value, 10, 64)
}

// parseVolume populates the volume map from the YAML node.
func parseVolume(name string, n *yaml.Node, out map[string]model.VolumeSpec) error {
	if n.Kind != yaml.MappingNode {
		return locErr(name, n, "volume: expected mapping")
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		entityName := k.Value
		spec, err := parseVolumeValue(name, v)
		if err != nil {
			return err
		}
		out[entityName] = spec
	}
	return nil
}

func parseVolumeValue(name string, v *yaml.Node) (model.VolumeSpec, error) {
	if v.Kind == yaml.ScalarNode {
		// Null / `~` → inferred
		if v.Tag == "!!null" || v.Value == "~" || v.Value == "" && v.Tag == "!!null" {
			return model.VolumeSpec{Inferred: true}, nil
		}
		// Range form
		if strings.Contains(v.Value, "..") {
			parts := strings.SplitN(v.Value, "..", 2)
			lo, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return model.VolumeSpec{}, locErr(name, v, "volume range lo: %v", err)
			}
			hi, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return model.VolumeSpec{}, locErr(name, v, "volume range hi: %v", err)
			}
			if lo < 0 || hi < 0 {
				return model.VolumeSpec{}, locErr(name, v, "volume range cannot be negative")
			}
			if lo > hi {
				return model.VolumeSpec{}, locErr(name, v, "volume range lower bound exceeds upper bound")
			}
			return model.VolumeSpec{Min: lo, Max: hi}, nil
		}
		// Plain integer
		n, err := strconv.Atoi(v.Value)
		if err != nil {
			return model.VolumeSpec{}, locErr(name, v, "invalid volume: %q", v.Value)
		}
		if n < 0 {
			return model.VolumeSpec{}, locErr(name, v, "volume cannot be negative")
		}
		return model.VolumeSpec{Exact: n}, nil
	}
	return model.VolumeSpec{}, locErr(name, v, "unsupported volume spec")
}

// parseEnums populates the enums map. Accepts list form and variant-object form.
func parseEnums(name string, n *yaml.Node, out *model.OrderedMap[string, model.EnumDef]) error {
	if n.Kind != yaml.MappingNode {
		return locErr(name, n, "enums: expected mapping")
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		enumName := k.Value
		variants, err := parseEnumVariants(name, v)
		if err != nil {
			return err
		}
		out.Set(enumName, model.EnumDef{Name: enumName, Variants: variants})
	}
	return nil
}

func parseEnumVariants(name string, n *yaml.Node) ([]model.EnumVariant, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, locErr(name, n, "enums: expected list of variants")
	}
	out := make([]model.EnumVariant, 0, len(n.Content))
	for _, item := range n.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			out = append(out, model.EnumVariant{Value: item.Value})
		case yaml.MappingNode:
			v := model.EnumVariant{}
			for i := 0; i+1 < len(item.Content); i += 2 {
				k := item.Content[i]
				val := item.Content[i+1]
				switch k.Value {
				case "value":
					v.Value = scalarString(val)
				case "label":
					v.Label = scalarString(val)
				case "weight":
					if val.Kind == yaml.ScalarNode {
						if f, err := strconv.ParseFloat(val.Value, 64); err == nil {
							v.Weight = &f
						}
					}
				case "description":
					v.Description = scalarString(val)
				}
			}
			if v.Value == "" {
				return nil, locErr(name, item, "enum variant missing 'value'")
			}
			out = append(out, v)
		default:
			return nil, locErr(name, item, "unsupported enum variant")
		}
	}
	return out, nil
}

// parseTypes parses the reusable types section. Each entry is an
// entity-shaped map; we reuse parseFields and wrap into an *Entity.
func parseTypes(name string, n *yaml.Node, out *model.OrderedMap[string, *model.Entity]) error {
	if n.Kind != yaml.MappingNode {
		return locErr(name, n, "types: expected mapping")
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		typeName := k.Value
		if v.Kind != yaml.MappingNode {
			return locErr(name, v, "types[%s]: expected mapping", typeName)
		}
		entity := model.NewEntity(typeName)
		if err := parseFieldsInto(name, v, nil, entity.Fields); err != nil {
			return err
		}
		out.Set(typeName, entity)
	}
	return nil
}

// parseEntities walks the entities section preserving declaration order.
func parseEntities(name string, n *yaml.Node, out *model.OrderedMap[string, *model.Entity]) error {
	if n.Kind != yaml.MappingNode {
		return locErr(name, n, "entities: expected mapping")
	}
	reserved := map[string]struct{}{
		"_meta":      {},
		"_coherence": {},
		"_triggers":  {},
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		entityName := k.Value
		if v.Kind != yaml.MappingNode {
			return locErr(name, v, "entities[%s]: expected mapping", entityName)
		}
		entity := model.NewEntity(entityName)

		// First pass: extract _meta / _coherence so they do not leak into fields.
		for j := 0; j+1 < len(v.Content); j += 2 {
			kk := v.Content[j]
			vv := v.Content[j+1]
			switch kk.Value {
			case "_meta":
				metaStr := scalarString(vv)
				if metaStr == "" {
					continue
				}
				// split_type_and_decorators expects a type prefix; seed with a
				// placeholder so @foo tokens are recognised as decorators.
				_, decs, err := splitTypeAndDecorators("_ " + metaStr)
				if err != nil {
					return locErr(name, vv, "entity %s _meta: %v", entityName, err)
				}
				entity.Meta = decs
			case "_coherence":
				if vv.Kind != yaml.MappingNode {
					return locErr(name, vv, "entity %s _coherence: expected mapping", entityName)
				}
				for kkk := 0; kkk+1 < len(vv.Content); kkk += 2 {
					groupKey := vv.Content[kkk]
					groupVal := vv.Content[kkk+1]
					if groupVal.Kind != yaml.SequenceNode {
						return locErr(name, groupVal, "coherence group %s: expected list", groupKey.Value)
					}
					members := make([]string, 0, len(groupVal.Content))
					for _, g := range groupVal.Content {
						members = append(members, g.Value)
					}
					entity.Coherence.Set(groupKey.Value, members)
				}
			}
		}

		// Second pass: ordinary fields.
		if err := parseFieldsInto(name, v, reserved, entity.Fields); err != nil {
			return err
		}
		out.Set(entityName, entity)
	}
	return nil
}

// parseFieldsInto walks a mapping node's key/value pairs, skipping any keys
// in `skip`, and appends the resulting *Field entries to `out` in declaration
// order.
func parseFieldsInto(name string, n *yaml.Node, skip map[string]struct{}, out *model.OrderedMap[string, *model.Field]) error {
	if n.Kind != yaml.MappingNode {
		return locErr(name, n, "expected mapping of fields")
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		fieldName := k.Value
		if _, ok := skip[fieldName]; ok {
			continue
		}
		field, err := parseField(name, fieldName, v)
		if err != nil {
			return err
		}
		out.Set(fieldName, field)
	}
	return nil
}

// parseField builds a *model.Field from either shorthand ("type @dec") or
// expanded-mapping form.
func parseField(name, fieldName string, v *yaml.Node) (*model.Field, error) {
	f := &model.Field{Name: fieldName}

	var typeStr string
	switch v.Kind {
	case yaml.ScalarNode:
		typeStr = v.Value
	case yaml.MappingNode:
		for i := 0; i+1 < len(v.Content); i += 2 {
			k := v.Content[i]
			val := v.Content[i+1]
			switch k.Value {
			case "type":
				typeStr = scalarString(val)
			case "label":
				f.Label = scalarString(val)
			case "description":
				f.Description = scalarString(val)
			case "default_chain":
				dc, err := parseDefaultChain(name, v, val)
				if err != nil {
					return nil, err
				}
				f.DefaultChain = dc
			case "compute":
				branches, err := parseComputeBranches(name, val)
				if err != nil {
					return nil, err
				}
				f.Compute = branches
			}
		}
		if typeStr == "" {
			return nil, locErr(name, v, "field %s: mapping form requires 'type'", fieldName)
		}
	default:
		return nil, locErr(name, v, "field %s: expected scalar or mapping", fieldName)
	}

	typeFrag, decs, err := splitTypeAndDecorators(typeStr)
	if err != nil {
		return nil, locErr(name, v, "field %s: %v", fieldName, err)
	}
	// Trailing nullable shorthand — but references handle their own `?`.
	extraNullable := false
	if strings.HasSuffix(typeFrag, "?") && !strings.HasPrefix(typeFrag, "->") && !strings.HasPrefix(typeFrag, "<->") {
		extraNullable = true
		typeFrag = strings.TrimSuffix(typeFrag, "?")
	}
	te, err := parseTypeExpr(typeFrag)
	if err != nil {
		return nil, locErr(name, v, "field %s: %v", fieldName, err)
	}
	if extraNullable {
		te = model.Nullable{Inner: te}
	}
	f.Type = te
	f.Decorators = decs
	return f, nil
}

// parseDefaultChain constructs a DefaultChainSpec, reading `when` and
// `fallback` from the owning mapping.
func parseDefaultChain(name string, owner, chainNode *yaml.Node) (*model.DefaultChainSpec, error) {
	if chainNode.Kind != yaml.SequenceNode {
		return nil, locErr(name, chainNode, "default_chain: expected list")
	}
	sources := make([]string, 0, len(chainNode.Content))
	for _, s := range chainNode.Content {
		sources = append(sources, scalarString(s))
	}
	if len(sources) == 0 {
		return nil, locErr(name, chainNode, "default_chain: requires at least one source")
	}
	spec := &model.DefaultChainSpec{Sources: sources}
	// owner is the field's value mapping; peek siblings for when/fallback.
	if owner.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(owner.Content); i += 2 {
			k := owner.Content[i]
			v := owner.Content[i+1]
			switch k.Value {
			case "when":
				spec.When = scalarString(v)
			case "fallback":
				spec.Fallback = scalarString(v)
			}
		}
	}
	return spec, nil
}

// parseComputeBranches walks a compute: list into ComputeBranch entries.
func parseComputeBranches(name string, n *yaml.Node) ([]model.ComputeBranch, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, locErr(name, n, "compute: expected list")
	}
	out := make([]model.ComputeBranch, 0, len(n.Content))
	for _, item := range n.Content {
		if item.Kind != yaml.MappingNode {
			return nil, locErr(name, item, "compute branch: expected mapping")
		}
		branch := model.ComputeBranch{}
		var haveWhen, haveValue, haveElse bool
		for i := 0; i+1 < len(item.Content); i += 2 {
			k := item.Content[i]
			v := item.Content[i+1]
			switch k.Value {
			case "when":
				branch.When = scalarString(v)
				haveWhen = true
			case "value":
				branch.Value = scalarString(v)
				haveValue = true
			case "else":
				branch.Value = scalarString(v)
				haveElse = true
			}
		}
		switch {
		case haveElse:
			branch.When = ""
		case haveWhen && haveValue:
			// ok
		default:
			return nil, locErr(name, item, "compute branch must have 'when'+'value' or 'else'")
		}
		out = append(out, branch)
	}
	if len(out) == 0 {
		return nil, locErr(name, n, "compute: requires at least one branch")
	}
	return out, nil
}

// parseRules accepts both string-shorthand and mapping-shape rules. Mapping
// rules are flattened into the Expr field verbatim (as a JSON-ish expression
// string) so the validator/generator can interpret them in later phases.
func parseRules(name string, n *yaml.Node) ([]model.Rule, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, locErr(name, n, "rules: expected list")
	}
	rules := make([]model.Rule, 0, len(n.Content))
	for _, item := range n.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			rule, err := parseRuleString(name, item)
			if err != nil {
				return nil, err
			}
			rules = append(rules, rule)
		case yaml.MappingNode:
			rule, err := parseRuleMapping(name, item)
			if err != nil {
				return nil, err
			}
			rules = append(rules, rule)
		default:
			return nil, locErr(name, item, "rule: expected scalar or mapping")
		}
	}
	return rules, nil
}

func parseRuleString(name string, n *yaml.Node) (model.Rule, error) {
	raw := n.Value
	sev := model.RuleStrict
	prob := 0.0

	// Detect modifier tokens at the tail.
	if idx := strings.Index(raw, "@probability("); idx >= 0 {
		tail := raw[idx+len("@probability("):]
		end := strings.IndexByte(tail, ')')
		if end < 0 {
			return model.Rule{}, locErr(name, n, "rule: unterminated @probability()")
		}
		p, err := strconv.ParseFloat(strings.TrimSpace(tail[:end]), 64)
		if err != nil {
			return model.Rule{}, locErr(name, n, "rule: invalid probability %q", tail[:end])
		}
		prob = p
		sev = model.RuleProbabilistic
		raw = strings.TrimSpace(raw[:idx])
	} else if idx := strings.LastIndex(raw, "@strict"); idx >= 0 {
		sev = model.RuleStrict
		raw = strings.TrimSpace(raw[:idx])
	} else if idx := strings.LastIndex(raw, "@warn"); idx >= 0 {
		sev = model.RuleWarn
		raw = strings.TrimSpace(raw[:idx])
	}
	return model.Rule{Expr: raw, Severity: sev, Probability: prob}, nil
}

// parseRuleMapping turns a mapping-shaped rule back into a Rule with a
// synthetic expression string. The generator/validator in later phases
// understands both forms via the mapping round-trip.
func parseRuleMapping(name string, n *yaml.Node) (model.Rule, error) {
	var when, assert, errMsg, severity string
	var prob *float64
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		switch k.Value {
		case "when":
			when = scalarString(v)
		case "assert":
			assert = scalarString(v)
		case "error":
			errMsg = scalarString(v)
		case "severity":
			severity = scalarString(v)
		case "probability":
			if v.Kind == yaml.ScalarNode {
				if p, err := strconv.ParseFloat(v.Value, 64); err == nil {
					prob = &p
				}
			}
		case "cross_row":
			// Cross-row rules carry their own shape; we preserve the raw
			// YAML text in CrossRow so downstream phases can re-parse.
			out, err := yaml.Marshal(n)
			if err != nil {
				return model.Rule{}, locErr(name, n, "cross_row rule: %v", err)
			}
			return model.Rule{
				Kind:         model.RuleKindCrossRow,
				CrossRow:     string(out),
				ErrorMessage: errMsg,
				Severity:     model.RuleStrict,
			}, nil
		}
	}
	expr := assert
	if when != "" {
		expr = "if " + when + " then " + assert
	}
	sev := model.RuleStrict
	p := 0.0
	switch {
	case severity == "warn":
		sev = model.RuleWarn
	case strings.HasPrefix(severity, "probability("):
		inner := strings.TrimSuffix(strings.TrimPrefix(severity, "probability("), ")")
		if f, err := strconv.ParseFloat(strings.TrimSpace(inner), 64); err == nil {
			sev = model.RuleProbabilistic
			p = f
		}
	case prob != nil:
		sev = model.RuleProbabilistic
		p = *prob
	}
	return model.Rule{Expr: expr, ErrorMessage: errMsg, Severity: sev, Probability: p}, nil
}

// parseTools returns the raw tools section as pass-through map[string]any
// wrapped in ToolOverride structs.
func parseTools(name string, n *yaml.Node) (map[string]model.ToolOverride, error) {
	if n.Kind != yaml.MappingNode {
		return nil, locErr(name, n, "tools: expected mapping")
	}
	out := map[string]model.ToolOverride{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		raw, err := nodeToAny(v)
		if err != nil {
			return nil, locErr(name, v, "tools[%s]: %v", k.Value, err)
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, locErr(name, v, "tools[%s]: expected mapping", k.Value)
		}
		out[k.Value] = model.ToolOverride{Raw: m}
	}
	return out, nil
}

// parseGeneration builds a GenerationConfig. Missing or unknown knobs keep
// their zero value so defaults applied downstream still work.
func parseGeneration(name string, n *yaml.Node) (model.GenerationConfig, error) {
	cfg := model.GenerationConfig{}
	if n.Kind != yaml.MappingNode {
		return cfg, locErr(name, n, "generation: expected mapping")
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		switch k.Value {
		case "seed":
			if s, err := scalarInt64(v); err == nil {
				cfg.Seed = &s
			}
		case "locale":
			cfg.Locale = scalarString(v)
		case "locales":
			if v.Kind == yaml.MappingNode {
				cfg.Locales = map[string]int{}
				for j := 0; j+1 < len(v.Content); j += 2 {
					kk := v.Content[j]
					vv := v.Content[j+1]
					if n, err := strconv.Atoi(vv.Value); err == nil {
						cfg.Locales[kk.Value] = n
					}
				}
			}
		case "null_strategy":
			cfg.NullStrategy = scalarString(v)
		case "id_format":
			cfg.IDFormat = scalarString(v)
		case "date_format":
			cfg.DateFormat = scalarString(v)
		case "currency_format":
			cfg.CurrencyFmt = scalarString(v)
		case "llm":
			llm, err := parseLLM(name, v)
			if err != nil {
				return cfg, err
			}
			cfg.LLM = llm
		}
	}
	return cfg, nil
}

func parseLLM(name string, n *yaml.Node) (*model.LLMConfig, error) {
	if n.Kind != yaml.MappingNode {
		return nil, locErr(name, n, "generation.llm: expected mapping")
	}
	cfg := &model.LLMConfig{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		v := n.Content[i+1]
		switch k.Value {
		case "provider":
			cfg.Provider = scalarString(v)
		case "endpoint":
			cfg.Endpoint = scalarString(v)
		case "model":
			cfg.Model = scalarString(v)
		case "api_key":
			cfg.APIKey = scalarString(v)
		case "temperature":
			if f, err := strconv.ParseFloat(v.Value, 64); err == nil {
				cfg.Temperature = &f
			}
		case "timeout_secs":
			if n, err := strconv.Atoi(v.Value); err == nil {
				cfg.TimeoutSecs = &n
			}
		case "max_tokens":
			if n, err := strconv.Atoi(v.Value); err == nil {
				cfg.MaxTokens = &n
			}
		}
	}
	return cfg, nil
}

// nodeToAny converts an arbitrary yaml.Node subtree into the usual Go
// interface shape (map[string]any / []any / scalar). Used for the opaque
// tools: section.
func nodeToAny(n *yaml.Node) (any, error) {
	if n == nil {
		return nil, nil
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return nodeToAny(n.Content[0])
	case yaml.MappingNode:
		out := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			v := n.Content[i+1]
			val, err := nodeToAny(v)
			if err != nil {
				return nil, err
			}
			out[k.Value] = val
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			val, err := nodeToAny(c)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		}
		return out, nil
	case yaml.ScalarNode:
		// Preserve typed scalars.
		switch n.Tag {
		case "!!int":
			if n, err := strconv.ParseInt(n.Value, 10, 64); err == nil {
				return n, nil
			}
		case "!!float":
			if f, err := strconv.ParseFloat(n.Value, 64); err == nil {
				return f, nil
			}
		case "!!bool":
			return n.Value == "true", nil
		case "!!null":
			return nil, nil
		}
		return n.Value, nil
	case yaml.AliasNode:
		if n.Alias != nil {
			return nodeToAny(n.Alias)
		}
	}
	return nil, nil
}

// locErr builds a parse error carrying the node's position.
func locErr(name string, n *yaml.Node, format string, args ...any) error {
	loc := &derrs.Location{File: name}
	if n != nil {
		loc.Line = n.Line
		loc.Col = n.Column
	}
	return &derrs.Error{
		Kind:     derrs.KindParse,
		Location: loc,
		Message:  fmt.Sprintf(format, args...),
	}
}
