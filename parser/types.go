package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jmcarbo/datjitgo/core/model"
)

// parseTypeExpr parses a DDL type expression string into a model.TypeExpr.
//
// Precedence (loosest to tightest):
//  1. Union  `T1 | T2`
//  2. Nullable `T?`
//  3. Compound `[T]`, `{K:V}`, `(T1, T2)`
//  4. Reference `->E`, `->E?`, `->[E]`, `<->E`, `->self`, `->self?`
//  5. Inline enum `enum(a, b, c)`
//  6. Parameterised primitive/semantic `int(32)`, `decimal(10,2)`, `currency(USD)`
//  7. Bare primitive/semantic/named.
func parseTypeExpr(src string) (model.TypeExpr, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty type expression")
	}

	// 1. Union
	if parts, err := splitTopLevel(src, '|'); err == nil && len(parts) > 1 {
		variants := make([]model.TypeExpr, 0, len(parts))
		for _, p := range parts {
			te, err := parseTypeExpr(strings.TrimSpace(p))
			if err != nil {
				return nil, err
			}
			variants = append(variants, te)
		}
		return model.Union{Variants: variants}, nil
	}

	// 2. Nullable — T? (but not ->T? which is handled in reference parsing)
	if strings.HasSuffix(src, "?") && !strings.HasPrefix(src, "->") && !strings.HasPrefix(src, "<->") {
		inner, err := parseTypeExpr(strings.TrimSpace(src[:len(src)-1]))
		if err != nil {
			return nil, err
		}
		return model.Nullable{Inner: inner}, nil
	}

	// 3. Compound: list, map, tuple.
	if strings.HasPrefix(src, "[") && strings.HasSuffix(src, "]") && matchesEnclosure(src, '[', ']') {
		inner, err := parseTypeExpr(strings.TrimSpace(src[1 : len(src)-1]))
		if err != nil {
			return nil, err
		}
		return model.List{Element: inner}, nil
	}
	if strings.HasPrefix(src, "{") && strings.HasSuffix(src, "}") && matchesEnclosure(src, '{', '}') {
		body := src[1 : len(src)-1]
		// Split on top-level ':' — first occurrence only, since value types
		// themselves may contain colons (e.g. nested maps).
		idx, err := firstTopLevel(body, ':')
		if err != nil {
			return nil, err
		}
		if idx < 0 {
			return nil, fmt.Errorf("map type missing ':': %q", src)
		}
		k, err := parseTypeExpr(strings.TrimSpace(body[:idx]))
		if err != nil {
			return nil, err
		}
		v, err := parseTypeExpr(strings.TrimSpace(body[idx+1:]))
		if err != nil {
			return nil, err
		}
		return model.Map{Key: k, Value: v}, nil
	}
	if strings.HasPrefix(src, "(") && strings.HasSuffix(src, ")") && matchesEnclosure(src, '(', ')') {
		body := src[1 : len(src)-1]
		parts, err := splitTopLevel(body, ',')
		if err != nil {
			return nil, err
		}
		elems := make([]model.TypeExpr, 0, len(parts))
		for _, p := range parts {
			te, err := parseTypeExpr(strings.TrimSpace(p))
			if err != nil {
				return nil, err
			}
			elems = append(elems, te)
		}
		return model.Tuple{Elements: elems}, nil
	}

	// 4. References
	if strings.HasPrefix(src, "<->") {
		target := strings.TrimSpace(src[3:])
		if target == "" {
			return nil, fmt.Errorf("many-to-many reference missing target")
		}
		return model.Reference{Target: target, ManyToMany: true}, nil
	}
	if strings.HasPrefix(src, "->") {
		return parseReference(strings.TrimSpace(src[2:]))
	}

	// 5. Inline enum.
	if strings.HasPrefix(src, "enum(") && strings.HasSuffix(src, ")") {
		body := src[5 : len(src)-1]
		parts, err := splitTopLevel(body, ',')
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(parts))
		for _, p := range parts {
			values = append(values, strings.TrimSpace(p))
		}
		return model.EnumInline{Values: values}, nil
	}

	// 6. Parameterised form: name(args)
	if open := strings.IndexByte(src, '('); open > 0 && strings.HasSuffix(src, ")") {
		head := src[:open]
		body := src[open+1 : len(src)-1]
		if te, ok, err := tryParameterisedPrimitive(head, body); ok {
			return te, err
		}
		// Otherwise, treat as a semantic type with params. `head` may be
		// dotted (e.g. accounting.group).
		ns, tag := splitSemanticName(head)
		if ns != "" {
			parts, err := splitTopLevel(body, ',')
			if err != nil {
				return nil, err
			}
			params := make([]string, 0, len(parts))
			for _, p := range parts {
				params = append(params, strings.TrimSpace(p))
			}
			return model.Semantic{Namespace: ns, Tag: tag, Params: params}, nil
		}
	}

	// 7. Bare atom.
	return parseAtom(src)
}

// parseReference parses everything after the `->` prefix.
func parseReference(src string) (model.TypeExpr, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty reference target")
	}
	// ->self / ->self?
	if src == "self" {
		return model.Reference{Target: "self"}, nil
	}
	if src == "self?" {
		return model.Reference{Target: "self", Optional: true}, nil
	}
	// ->[Entity]
	if strings.HasPrefix(src, "[") && strings.HasSuffix(src, "]") {
		target := strings.TrimSpace(src[1 : len(src)-1])
		if target == "" {
			return nil, fmt.Errorf("has-many reference missing target")
		}
		return model.Reference{Target: target, Many: true}, nil
	}
	// ->Entity?
	if strings.HasSuffix(src, "?") {
		return model.Reference{Target: strings.TrimSpace(src[:len(src)-1]), Optional: true}, nil
	}
	return model.Reference{Target: src}, nil
}

// tryParameterisedPrimitive recognises the fixed set of primitive types that
// accept parameters. Returns (expr, matched, err).
func tryParameterisedPrimitive(head, body string) (model.TypeExpr, bool, error) {
	switch head {
	case "int":
		n, err := strconv.Atoi(strings.TrimSpace(body))
		if err != nil {
			return nil, true, fmt.Errorf("invalid int() bit width %q: %w", body, err)
		}
		return model.Primitive{Kind: model.PrimInt, Params: []int{n}}, true, nil
	case "float":
		n, err := strconv.Atoi(strings.TrimSpace(body))
		if err != nil {
			return nil, true, fmt.Errorf("invalid float() bit width %q: %w", body, err)
		}
		return model.Primitive{Kind: model.PrimFloat, Params: []int{n}}, true, nil
	case "string":
		n, err := strconv.Atoi(strings.TrimSpace(body))
		if err != nil {
			return nil, true, fmt.Errorf("invalid string() max length %q: %w", body, err)
		}
		return model.Primitive{Kind: model.PrimString, Params: []int{n}}, true, nil
	case "bytes":
		n, err := strconv.Atoi(strings.TrimSpace(body))
		if err != nil {
			return nil, true, fmt.Errorf("invalid bytes() max length %q: %w", body, err)
		}
		return model.Primitive{Kind: model.PrimBytes, Params: []int{n}}, true, nil
	case "decimal":
		parts, err := splitTopLevel(body, ',')
		if err != nil {
			return nil, true, err
		}
		if len(parts) != 2 {
			return nil, true, fmt.Errorf("decimal expects (precision, scale); got %q", body)
		}
		p, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, true, fmt.Errorf("invalid decimal precision %q: %w", parts[0], err)
		}
		s, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, true, fmt.Errorf("invalid decimal scale %q: %w", parts[1], err)
		}
		return model.Primitive{Kind: model.PrimDecimal, Params: []int{p, s}}, true, nil
	}
	return nil, false, nil
}

// parseAtom handles bare names: primitives, semantic types, and named types.
func parseAtom(src string) (model.TypeExpr, error) {
	if kind, ok := lookupPrimitive(src); ok {
		return model.Primitive{Kind: kind}, nil
	}
	// Dotted semantic types have the shape `ns.tag` with lowercase parts.
	if strings.Contains(src, ".") {
		ns, tag := splitSemanticName(src)
		if ns != "" {
			return model.Semantic{Namespace: ns, Tag: tag}, nil
		}
	}
	// Single lowercase identifier → semantic (email, phone, slug, url, ...).
	if isLowerIdent(src) {
		return model.Semantic{Namespace: src}, nil
	}
	// Otherwise a user-defined named type / enum reference.
	return model.NamedType{Name: src}, nil
}

// lookupPrimitive returns the PrimKind for a bare primitive name.
func lookupPrimitive(s string) (model.PrimKind, bool) {
	switch s {
	case "string":
		return model.PrimString, true
	case "int":
		return model.PrimInt, true
	case "float":
		return model.PrimFloat, true
	case "bool":
		return model.PrimBool, true
	case "datetime":
		return model.PrimDatetime, true
	case "date":
		return model.PrimDate, true
	case "time":
		return model.PrimTime, true
	case "duration":
		return model.PrimDuration, true
	case "uuid":
		return model.PrimUUID, true
	case "bytes":
		return model.PrimBytes, true
	case "decimal":
		return model.PrimDecimal, true
	case "null":
		return model.PrimNull, true
	case "any":
		return model.PrimAny, true
	}
	return 0, false
}

// splitSemanticName separates `ns.tag` into (ns, tag). Returns ("", "") when
// the input is not a valid lowercase-dotted semantic name.
func splitSemanticName(src string) (string, string) {
	if !isLowerIdentDotted(src) {
		return "", ""
	}
	if i := strings.IndexByte(src, '.'); i >= 0 {
		return src[:i], src[i+1:]
	}
	return src, ""
}

func isLowerIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// isLowerIdentDotted accepts `foo.bar.baz` where each segment is
// lowercase-ident-shaped.
func isLowerIdentDotted(s string) bool {
	if s == "" {
		return false
	}
	for _, seg := range strings.Split(s, ".") {
		if !isLowerIdent(seg) {
			return false
		}
	}
	return true
}

// matchesEnclosure checks that the first byte of src matches `open` and its
// complementary closer at the end, with balanced depth in between. This lets
// us avoid treating `[int] | [string]` as a list when it's actually a union.
func matchesEnclosure(src string, open, close byte) bool {
	if len(src) < 2 || src[0] != open || src[len(src)-1] != close {
		return false
	}
	depth := 0
	var inQuote byte
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inQuote != 0:
			if c == inQuote && (i == 0 || src[i-1] != '\\') {
				inQuote = 0
			}
		case c == '"' || c == '\'':
			inQuote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 && i != len(src)-1 {
				return false
			}
		}
	}
	return depth == 0
}

// firstTopLevel returns the index of the first occurrence of sep at
// depth 0 and outside quotes, or -1 if absent.
func firstTopLevel(s string, sep byte) (int, error) {
	depth := 0
	var inQuote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote != 0:
			if c == inQuote && (i == 0 || s[i-1] != '\\') {
				inQuote = 0
			}
		case c == '"' || c == '\'':
			inQuote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth < 0 {
				return -1, fmt.Errorf("unbalanced brackets in %q", s)
			}
		case c == sep && depth == 0:
			return i, nil
		}
	}
	return -1, nil
}
