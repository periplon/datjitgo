package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// YAML is the YAML output writer. A single document is emitted containing a
// top-level mapping from entity name to an array of row mappings, with key
// order preserved via manual *yaml.Node construction.
type YAML struct{}

// NewYAML returns a new YAML writer.
func NewYAML() *YAML { return &YAML{} }

// Format returns "yaml".
func (*YAML) Format() string { return "yaml" }

// Write serialises ds to w as a YAML document.
func (y *YAML) Write(ds *value.Dataset, doc *model.Document, w io.Writer, opts ports.WriteOptions) error {
	if ds == nil {
		return writeAll(w, []byte("{}\n"))
	}
	order := entityOrder(ds, doc, opts.EntityFilter)

	top := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range order {
		rows, _ := ds.Entities.Get(name)
		fields := fieldOrder(rows, doc, name)

		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, row := range rows {
			keys := fields
			if len(keys) == 0 && row != nil {
				keys = row.Keys()
			}
			mapping := &yaml.Node{Kind: yaml.MappingNode}
			for _, k := range keys {
				v, ok := row.Get(k)
				if !ok {
					continue
				}
				node, err := valueToYAMLNode(v)
				if err != nil {
					return wrapIO(err, "yaml %s.%s", name, k)
				}
				mapping.Content = append(mapping.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: k, Tag: "!!str"},
					node,
				)
			}
			seq.Content = append(seq.Content, mapping)
		}

		top.Content = append(top.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: name, Tag: "!!str"},
			seq,
		)
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(top); err != nil {
		return wrapIO(err, "yaml encode")
	}
	if err := enc.Close(); err != nil {
		return wrapIO(err, "yaml close")
	}
	return nil
}

func valueToYAMLNode(v value.Value) (*yaml.Node, error) {
	switch v.Kind {
	case value.KindNull:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case value.KindBool:
		val := "false"
		if v.B {
			val = "true"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: val}, nil
	case value.KindInt:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%d", v.I)}, nil
	case value.KindFloat:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: formatFloat(v.F)}, nil
	case value.KindString:
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v.S}
		if yamlNeedsQuoting(v.S) {
			n.Style = yaml.DoubleQuotedStyle
		}
		return n, nil
	case value.KindUUID:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v.U.String()}, nil
	case value.KindTime:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v.T.UTC().Format(time.RFC3339Nano), Style: yaml.DoubleQuotedStyle}, nil
	case value.KindDecimal:
		// Render decimals as bare numeric literals to avoid float rounding.
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: v.D.String()}, nil
	case value.KindList:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range v.L {
			child, err := valueToYAMLNode(item)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, child)
		}
		return seq, nil
	case value.KindObject:
		mapping := &yaml.Node{Kind: yaml.MappingNode}
		if v.O != nil {
			var encErr error
			v.O.Each(func(k string, child value.Value) bool {
				childNode, err := valueToYAMLNode(child)
				if err != nil {
					encErr = err
					return false
				}
				mapping.Content = append(mapping.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: k, Tag: "!!str"},
					childNode,
				)
				return true
			})
			if encErr != nil {
				return nil, encErr
			}
		}
		return mapping, nil
	default:
		return nil, fmt.Errorf("unknown value kind %d", v.Kind)
	}
}

// yamlNeedsQuoting returns true when a string must be quoted to avoid being
// misinterpreted by a YAML parser (e.g. looking like a bool, number, or
// containing flow indicators / leading whitespace).
func yamlNeedsQuoting(s string) bool {
	if s == "" {
		return true
	}
	trim := strings.TrimSpace(s)
	switch strings.ToLower(trim) {
	case "true", "false", "null", "~", "yes", "no", "on", "off":
		return true
	}
	// Leading or trailing whitespace requires quoting.
	if trim != s {
		return true
	}
	// Characters that start indicator tokens at the beginning need quoting.
	if strings.ContainsAny(s[:1], "!&*-?:|>'\"%@`#,[]{}") {
		return true
	}
	// Flow-sequence / mapping indicators anywhere force quoting to be safe.
	if strings.ContainsAny(s, ":\n\t") {
		return true
	}
	return false
}
