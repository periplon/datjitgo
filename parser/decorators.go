package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jmcarbo/datjitgo/core/model"
)

// splitTypeAndDecorators splits a field shorthand like
//
//	"currency.usd @range(1..5000) @dist(lognormal)"
//
// into the type fragment (`currency.usd`) and a parsed slice of decorators.
// It respects `()`, `[]`, `{}` nesting depth and single/double quoted strings
// so commas, `@` and other metacharacters inside those spans don't confuse
// the tokenizer.
func splitTypeAndDecorators(src string) (string, []model.Decorator, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", nil, fmt.Errorf("empty field specification")
	}

	var (
		typePart         strings.Builder
		currentDecorator strings.Builder
		decoratorStrs    []string
		inDecorator      bool
		depth            int
		inQuote          byte // 0 when not in a quoted string
	)

	flushDecorator := func() {
		s := strings.TrimSpace(currentDecorator.String())
		if s != "" {
			decoratorStrs = append(decoratorStrs, s)
		}
		currentDecorator.Reset()
	}

	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inQuote != 0:
			if c == inQuote && (i == 0 || src[i-1] != '\\') {
				inQuote = 0
			}
			if inDecorator {
				currentDecorator.WriteByte(c)
			} else {
				typePart.WriteByte(c)
			}
		case c == '"' || c == '\'':
			inQuote = c
			if inDecorator {
				currentDecorator.WriteByte(c)
			} else {
				typePart.WriteByte(c)
			}
		case c == '@' && depth == 0:
			if inDecorator {
				flushDecorator()
			}
			inDecorator = true
			currentDecorator.WriteByte('@')
		case c == '(' || c == '[' || c == '{':
			depth++
			if inDecorator {
				currentDecorator.WriteByte(c)
			} else {
				typePart.WriteByte(c)
			}
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth < 0 {
				return "", nil, fmt.Errorf("unbalanced brackets in %q", src)
			}
			if inDecorator {
				currentDecorator.WriteByte(c)
			} else {
				typePart.WriteByte(c)
			}
		case c == ' ' && inDecorator && depth == 0 && currentDecorator.Len() > 1:
			flushDecorator()
			inDecorator = false
		default:
			if inDecorator {
				currentDecorator.WriteByte(c)
			} else {
				typePart.WriteByte(c)
			}
		}
	}
	if inDecorator {
		flushDecorator()
	}
	if depth != 0 {
		return "", nil, fmt.Errorf("unbalanced brackets in %q", src)
	}
	if inQuote != 0 {
		return "", nil, fmt.Errorf("unterminated string in %q", src)
	}

	decs := make([]model.Decorator, 0, len(decoratorStrs))
	for _, s := range decoratorStrs {
		d, err := parseDecorator(s)
		if err != nil {
			return "", nil, err
		}
		decs = append(decs, d)
	}
	return strings.TrimSpace(typePart.String()), decs, nil
}

// parseDecorator parses a single `@name` or `@name(args)` string into a
// model.Decorator. It does not evaluate the arguments against a decorator
// registry — that is the validator's job. Arguments are classified into
// range, key-value, identifier or literal forms.
func parseDecorator(src string) (model.Decorator, error) {
	src = strings.TrimSpace(src)
	if !strings.HasPrefix(src, "@") {
		return model.Decorator{}, fmt.Errorf("decorator missing '@': %q", src)
	}
	src = src[1:]

	open := strings.IndexByte(src, '(')
	if open < 0 {
		return model.Decorator{Name: strings.TrimSpace(src)}, nil
	}
	if !strings.HasSuffix(src, ")") {
		return model.Decorator{}, fmt.Errorf("unclosed decorator arguments: @%s", src)
	}
	name := strings.TrimSpace(src[:open])
	body := src[open+1 : len(src)-1]

	args, err := parseDecoratorArgs(name, body)
	if err != nil {
		return model.Decorator{}, fmt.Errorf("decorator @%s: %w", name, err)
	}
	return model.Decorator{Name: name, Args: args}, nil
}

// parseDecoratorArgs tokenises a decorator argument list. Some decorators
// (notably @pattern and @llm) accept a free-form string argument that may
// contain commas — for those we keep the first quoted arg as a single
// string literal and only split the rest on top-level commas.
func parseDecoratorArgs(decName, body string) ([]model.DecoratorArg, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, nil
	}

	parts, err := splitTopLevel(body, ',')
	if err != nil {
		return nil, err
	}

	// For @pattern — one argument, the entire body including any commas.
	if decName == "pattern" {
		raw := strings.TrimSpace(body)
		unq := stripQuotes(raw)
		return []model.DecoratorArg{{Kind: model.ArgLiteral, Raw: raw, Literal: unq}}, nil
	}

	// For @llm and @llm_values the first argument is a quoted prompt which
	// itself may contain commas — rejoin leading parts until the quoted
	// string closes, then classify the remaining parts normally.
	if decName == "llm" || decName == "llm_values" {
		parts = rejoinQuotedFirst(parts)
	}

	args := make([]model.DecoratorArg, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		args = append(args, classifyArg(p))
	}
	return args, nil
}

// rejoinQuotedFirst handles "Hello, world" coming out of splitTopLevel as
// two parts because the splitter treats quoted strings as transparent. We
// peek at the first part: if it starts with a quote and does not end with
// a matching one, concatenate subsequent parts until it does.
func rejoinQuotedFirst(parts []string) []string {
	if len(parts) == 0 {
		return parts
	}
	first := strings.TrimSpace(parts[0])
	if first == "" {
		return parts
	}
	q := first[0]
	if q != '"' && q != '\'' {
		return parts
	}
	// Already closed?
	if len(first) >= 2 && first[len(first)-1] == q && countUnescaped(first, q) == 2 {
		return parts
	}
	merged := first
	i := 1
	for ; i < len(parts); i++ {
		merged += "," + parts[i]
		// count unescaped quotes in merged
		if countUnescaped(merged, q) >= 2 {
			i++
			break
		}
	}
	out := []string{merged}
	out = append(out, parts[i:]...)
	return out
}

func countUnescaped(s string, q byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == q && (i == 0 || s[i-1] != '\\') {
			n++
		}
	}
	return n
}

// classifyArg inspects a single argument fragment and classifies it.
func classifyArg(raw string) model.DecoratorArg {
	arg := model.DecoratorArg{Raw: raw}

	// quoted string literal
	if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '\'') && raw[len(raw)-1] == raw[0] {
		arg.Kind = model.ArgLiteral
		arg.Literal = raw[1 : len(raw)-1]
		return arg
	}

	// range expressions
	if from, to, loExcl, hiExcl, ok := tryParseRange(raw); ok {
		arg.Kind = model.ArgRange
		arg.From = from
		arg.To = to
		arg.LoExcl = loExcl
		arg.HiExcl = hiExcl
		return arg
	}

	// key=value (allow Greek letter keys). Detect '=' that isn't part of
	// ==, >=, <=, != used in expressions. Simpler here: the whole fragment
	// is a decorator arg, so the first '=' delimits a KV if the key prefix
	// looks like an identifier rune sequence.
	if idx := findKVEquals(raw); idx > 0 {
		key := strings.TrimSpace(raw[:idx])
		val := strings.TrimSpace(raw[idx+1:])
		if key != "" {
			arg.Kind = model.ArgKV
			arg.Key = key
			arg.Value = val
			return arg
		}
	}

	// colon-form (model: foo) used by @llm — treat as KV.
	if idx := strings.IndexByte(raw, ':'); idx > 0 {
		key := strings.TrimSpace(raw[:idx])
		val := strings.TrimSpace(raw[idx+1:])
		// Accept only identifier-like keys so URLs such as
		// http://host don't collide.
		if isIdent(key) {
			arg.Kind = model.ArgKV
			arg.Key = key
			arg.Value = stripQuotes(val)
			return arg
		}
	}

	// integer / float / bool / null / bare ident
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		arg.Kind = model.ArgLiteral
		arg.Literal = n
		return arg
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		arg.Kind = model.ArgLiteral
		arg.Literal = f
		return arg
	}
	switch raw {
	case "true":
		arg.Kind = model.ArgLiteral
		arg.Literal = true
		return arg
	case "false":
		arg.Kind = model.ArgLiteral
		arg.Literal = false
		return arg
	}

	if isIdent(raw) {
		arg.Kind = model.ArgIdent
		arg.Ident = raw
		return arg
	}
	arg.Kind = model.ArgLiteral
	arg.Literal = raw
	return arg
}

// findKVEquals returns the byte index of the '=' that separates a KV pair,
// or -1 if the fragment isn't KV-shaped. Accepts Greek letters / underscores
// / letters / digits in the key.
func findKVEquals(s string) int {
	if len(s) == 0 || s[0] == '=' {
		return -1
	}
	idx := strings.IndexByte(s, '=')
	if idx < 0 {
		return -1
	}
	// not a comparison operator
	if idx+1 < len(s) && s[idx+1] == '=' {
		return -1
	}
	if idx > 0 && (s[idx-1] == '!' || s[idx-1] == '<' || s[idx-1] == '>') {
		return -1
	}
	prefix := strings.TrimSpace(s[:idx])
	if !isIdent(prefix) {
		return -1
	}
	return idx
}

// isIdent reports whether s is a run of identifier-ish runes (letters,
// digits, underscore). Greek letters are accepted as letters.
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' {
			continue
		}
		if isLetter(r) {
			continue
		}
		if i > 0 && (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func isLetter(r rune) bool {
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
		return true
	}
	// Greek lowercase/uppercase blocks used in distribution kwargs.
	if r >= 0x0370 && r <= 0x03FF {
		return true
	}
	return false
}

// tryParseRange checks for `lo..hi`, `lo<..hi`, `lo..<hi`, `lo<..<hi` and
// returns the numeric (as string) bounds plus exclusivity flags.
func tryParseRange(s string) (from, to string, loExcl, hiExcl bool, ok bool) {
	if idx := strings.Index(s, "<..<"); idx >= 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+4:]), true, true, true
	}
	if idx := strings.Index(s, "<.."); idx >= 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+3:]), true, false, true
	}
	if idx := strings.Index(s, "..<"); idx >= 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+3:]), false, true, true
	}
	if idx := strings.Index(s, ".."); idx >= 0 {
		// Disambiguate from float "1.5.2" — the separator must be the only ".."
		// appearing at the same level.
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+2:]), false, false, true
	}
	return "", "", false, false, false
}

// splitTopLevel splits s on separator byte sep at parenthesis depth 0 and
// outside quoted strings.
func splitTopLevel(s string, sep byte) ([]string, error) {
	var (
		parts   []string
		current strings.Builder
		depth   int
		inQuote byte
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote != 0:
			if c == inQuote && (i == 0 || s[i-1] != '\\') {
				inQuote = 0
			}
			current.WriteByte(c)
		case c == '"' || c == '\'':
			inQuote = c
			current.WriteByte(c)
		case c == '(' || c == '[' || c == '{':
			depth++
			current.WriteByte(c)
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced brackets in %q", s)
			}
			current.WriteByte(c)
		case c == sep && depth == 0:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}
	if inQuote != 0 {
		return nil, fmt.Errorf("unterminated string in %q", s)
	}
	parts = append(parts, current.String())
	return parts, nil
}

// stripQuotes removes matching surrounding single or double quotes.
func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
