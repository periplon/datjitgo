package generator

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/value"
)

// exprNode is a node in the compiled expression AST. Shapes are intentionally
// few — keeping the AST small makes the evaluator easier to reason about.
type exprNode struct {
	kind     exprKind
	str      string // name / literal / field path segment
	val      value.Value
	children []exprNode
	op       string // for binary/unary ops
}

type exprKind int

const (
	exprLit exprKind = iota
	exprPath
	exprFunc
	exprBinary
	exprUnary
	exprIn
)

// ParseExpr is the exported validator-facing entry point: it parses src as
// an expression and returns an error if the source is not syntactically
// well-formed. The returned AST is intentionally opaque; callers that need
// to evaluate expressions should go through a full Generate.
func ParseExpr(src string) error {
	_, err := parseExpr(src)
	return err
}

// parseExpr is the top-level entry point: it compiles src into an exprNode.
// Errors are wrapped as *errors.Error{Kind: KindGeneration}.
func parseExpr(src string) (exprNode, error) {
	p := newExprParser(src)
	node, err := p.parseExpression(0)
	if err != nil {
		return exprNode{}, err
	}
	p.skipSpace()
	if p.pos != len(p.src) {
		return exprNode{}, generationf("expr %q: trailing input at pos %d", src, p.pos)
	}
	return node, nil
}

// --- Pratt parser ---

type exprParser struct {
	src string
	pos int
}

func newExprParser(src string) *exprParser { return &exprParser{src: src} }

// Operator precedences (higher binds tighter).
const (
	precNone       = 0
	precOr         = 1
	precAnd        = 2
	precCompareEq  = 3
	precCompareOrd = 4
	precIn         = 5
	precAddSub     = 6
	precMulDiv     = 7
	precUnary      = 8
)

func (p *exprParser) parseExpression(minPrec int) (exprNode, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return exprNode{}, err
	}
	for {
		p.skipSpace()
		prec, op, width := p.peekOp()
		if prec == precNone || prec < minPrec {
			break
		}
		p.pos += width
		if op == "in" {
			list, err := p.parseList()
			if err != nil {
				return exprNode{}, err
			}
			left = exprNode{kind: exprIn, children: append([]exprNode{left}, list...)}
			continue
		}
		right, err := p.parseExpression(prec + 1)
		if err != nil {
			return exprNode{}, err
		}
		left = exprNode{kind: exprBinary, op: op, children: []exprNode{left, right}}
	}
	return left, nil
}

// parsePrefix handles literals, identifiers (path or function), parentheses,
// unary not/minus.
func (p *exprParser) parsePrefix() (exprNode, error) {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return exprNode{}, generationf("expr: unexpected end of input")
	}
	c := p.src[p.pos]

	switch {
	case c == '(':
		p.pos++
		node, err := p.parseExpression(0)
		if err != nil {
			return exprNode{}, err
		}
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return exprNode{}, generationf("expr: expected ')'")
		}
		p.pos++
		return node, nil
	case c == '-':
		p.pos++
		inner, err := p.parseExpression(precUnary)
		if err != nil {
			return exprNode{}, err
		}
		return exprNode{kind: exprUnary, op: "-", children: []exprNode{inner}}, nil
	case c == '"' || c == '\'':
		return p.parseString(c)
	case c >= '0' && c <= '9':
		return p.parseNumber()
	}

	// Keyword / identifier / function.
	if isIdentStart(c) {
		start := p.pos
		for p.pos < len(p.src) && isIdentPart(p.src[p.pos]) {
			p.pos++
		}
		word := p.src[start:p.pos]
		switch word {
		case "null":
			return exprNode{kind: exprLit, val: value.Null()}, nil
		case "true":
			return exprNode{kind: exprLit, val: value.Bool(true)}, nil
		case "false":
			return exprNode{kind: exprLit, val: value.Bool(false)}, nil
		case "not":
			inner, err := p.parseExpression(precUnary)
			if err != nil {
				return exprNode{}, err
			}
			return exprNode{kind: exprUnary, op: "not", children: []exprNode{inner}}, nil
		}
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == '(' {
			// Function call
			p.pos++
			args, err := p.parseArgs(')')
			if err != nil {
				return exprNode{}, err
			}
			return exprNode{kind: exprFunc, str: word, children: args}, nil
		}
		// Path: consume subsequent `.ident` segments.
		path := []string{word}
		for p.pos < len(p.src) && p.src[p.pos] == '.' {
			p.pos++
			segStart := p.pos
			for p.pos < len(p.src) && isIdentPart(p.src[p.pos]) {
				p.pos++
			}
			if p.pos == segStart {
				return exprNode{}, generationf("expr: invalid path after '.'")
			}
			path = append(path, p.src[segStart:p.pos])
		}
		return exprNode{kind: exprPath, str: strings.Join(path, ".")}, nil
	}

	return exprNode{}, generationf("expr: unexpected character %q at %d", c, p.pos)
}

func (p *exprParser) parseArgs(closer byte) ([]exprNode, error) {
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == closer {
		p.pos++
		return nil, nil
	}
	var out []exprNode
	for {
		node, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		out = append(out, node)
		p.skipSpace()
		if p.pos >= len(p.src) {
			return nil, generationf("expr: unclosed call")
		}
		if p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.src[p.pos] == closer {
			p.pos++
			return out, nil
		}
		return nil, generationf("expr: expected ',' or %q, got %q", closer, p.src[p.pos])
	}
}

func (p *exprParser) parseList() ([]exprNode, error) {
	p.skipSpace()
	if p.pos >= len(p.src) || p.src[p.pos] != '[' {
		return nil, generationf("expr: 'in' requires '[list]'")
	}
	p.pos++
	return p.parseArgs(']')
}

func (p *exprParser) parseString(q byte) (exprNode, error) {
	p.pos++ // consume opening quote
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] != q {
		if p.src[p.pos] == '\\' && p.pos+1 < len(p.src) {
			p.pos += 2
			continue
		}
		p.pos++
	}
	if p.pos >= len(p.src) {
		return exprNode{}, generationf("expr: unterminated string")
	}
	raw := p.src[start:p.pos]
	p.pos++ // closing quote
	return exprNode{kind: exprLit, val: value.Str(unescape(raw))}, nil
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case '\'':
				b.WriteByte('\'')
			default:
				b.WriteByte(s[i+1])
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func (p *exprParser) parseNumber() (exprNode, error) {
	start := p.pos
	isFloat := false
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c >= '0' && c <= '9':
			p.pos++
		case c == '.' && !isFloat:
			isFloat = true
			p.pos++
		default:
			goto done
		}
	}
done:
	raw := p.src[start:p.pos]
	if isFloat {
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return exprNode{}, generationf("expr: bad float %q", raw)
		}
		return exprNode{kind: exprLit, val: value.Float(f)}, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return exprNode{}, generationf("expr: bad int %q", raw)
	}
	return exprNode{kind: exprLit, val: value.Int(n)}, nil
}

// peekOp returns the precedence, canonical op string, and consumed width for
// a binary operator starting at p.pos (or precNone if none).
func (p *exprParser) peekOp() (int, string, int) {
	s := p.src[p.pos:]
	// Two-char operators first.
	if strings.HasPrefix(s, "==") {
		return precCompareEq, "==", 2
	}
	if strings.HasPrefix(s, "!=") {
		return precCompareEq, "!=", 2
	}
	if strings.HasPrefix(s, "<=") {
		return precCompareOrd, "<=", 2
	}
	if strings.HasPrefix(s, ">=") {
		return precCompareOrd, ">=", 2
	}
	if strings.HasPrefix(s, "and") && !hasIdentAfter(s, 3) {
		return precAnd, "and", 3
	}
	if strings.HasPrefix(s, "or") && !hasIdentAfter(s, 2) {
		return precOr, "or", 2
	}
	if strings.HasPrefix(s, "in") && !hasIdentAfter(s, 2) {
		return precIn, "in", 2
	}
	if len(s) == 0 {
		return precNone, "", 0
	}
	switch s[0] {
	case '+':
		return precAddSub, "+", 1
	case '-':
		return precAddSub, "-", 1
	case '*':
		return precMulDiv, "*", 1
	case '/':
		return precMulDiv, "/", 1
	case '%':
		return precMulDiv, "%", 1
	case '<':
		return precCompareOrd, "<", 1
	case '>':
		return precCompareOrd, ">", 1
	}
	return precNone, "", 0
}

func hasIdentAfter(s string, n int) bool {
	if len(s) <= n {
		return false
	}
	return isIdentPart(s[n])
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			p.pos++
			continue
		}
		break
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// --- Evaluator ---

// evalEnv is the lookup context for expression evaluation.
type evalEnv struct {
	row  *value.Object
	data map[string][]*value.Object
}

func evalExpr(node exprNode, env evalEnv) (value.Value, error) {
	switch node.kind {
	case exprLit:
		return node.val, nil
	case exprPath:
		return resolvePath(node.str, env), nil
	case exprUnary:
		inner, err := evalExpr(node.children[0], env)
		if err != nil {
			return value.Null(), err
		}
		switch node.op {
		case "-":
			switch inner.Kind {
			case value.KindInt:
				return value.Int(-inner.I), nil
			case value.KindFloat:
				return value.Float(-inner.F), nil
			default:
				// non-numeric kinds: handled by the trailing return
			}
			return value.Null(), generationf("expr: cannot negate %v", inner.Kind)
		case "not":
			return value.Bool(!truthy(inner)), nil
		}
	case exprBinary:
		l, err := evalExpr(node.children[0], env)
		if err != nil {
			return value.Null(), err
		}
		r, err := evalExpr(node.children[1], env)
		if err != nil {
			return value.Null(), err
		}
		return evalBinary(node.op, l, r)
	case exprFunc:
		args := make([]value.Value, 0, len(node.children))
		// "if" short-circuits — evaluate condition first.
		if node.str == "if" {
			return evalIf(node.children, env)
		}
		for _, c := range node.children {
			v, err := evalExpr(c, env)
			if err != nil {
				return value.Null(), err
			}
			args = append(args, v)
		}
		return evalFunc(node.str, args, node.children, env)
	case exprIn:
		if len(node.children) < 1 {
			return value.Bool(false), nil
		}
		left, err := evalExpr(node.children[0], env)
		if err != nil {
			return value.Null(), err
		}
		for _, c := range node.children[1:] {
			v, err := evalExpr(c, env)
			if err != nil {
				return value.Null(), err
			}
			if valuesEqual(left, v) {
				return value.Bool(true), nil
			}
		}
		return value.Bool(false), nil
	}
	return value.Null(), nil
}

// resolvePath looks up a dotted path. A single segment is a row field; two
// segments are either an entity.field (aggregate context) or a reference
// traversal. For phase 1 we accept both shapes by trying row first.
func resolvePath(path string, env evalEnv) value.Value {
	segments := strings.Split(path, ".")
	if len(segments) == 1 {
		if env.row != nil {
			if v, ok := env.row.Get(segments[0]); ok {
				return v
			}
		}
		// Could also be an entity reference -> null for the direct case.
		return value.Null()
	}
	// Two-segment: entity.field (aggregate over all rows) OR ref.field
	first, second := segments[0], segments[1]
	if env.row != nil {
		if v, ok := env.row.Get(first); ok && v.Kind == value.KindList {
			// Ref list — return list of projected fields.
			out := make([]value.Value, 0, len(v.L))
			for _, it := range v.L {
				if it.Kind == value.KindObject {
					if inner, ok := it.O.Get(second); ok {
						out = append(out, inner)
					}
				}
			}
			return value.List(out)
		}
	}
	if rows, ok := env.data[first]; ok {
		// Direct entity projection; callers may aggregate via sum/avg.
		out := make([]value.Value, 0, len(rows))
		for _, row := range rows {
			if v, ok := row.Get(second); ok {
				out = append(out, v)
			}
		}
		return value.List(out)
	}
	return value.Null()
}

func evalBinary(op string, l, r value.Value) (value.Value, error) {
	switch op {
	case "+", "-", "*", "/", "%":
		return evalArithmetic(op, l, r)
	case "==", "!=", "<", ">", "<=", ">=":
		return evalComparison(op, l, r)
	case "and", "or":
		return evalLogical(op, l, r)
	}
	return value.Null(), generationf("expr: unknown op %q", op)
}

// evalArithmetic handles +, -, *, / and % (with + doubling as string concat).
func evalArithmetic(op string, l, r value.Value) (value.Value, error) {
	switch op {
	case "+":
		// String concatenation if either side is a string.
		if l.Kind == value.KindString || r.Kind == value.KindString {
			return value.Str(valueDisplay(l) + valueDisplay(r)), nil
		}
		return numericOp(l, r, func(a, b int64) int64 { return a + b }, func(a, b float64) float64 { return a + b })
	case "-":
		return numericOp(l, r, func(a, b int64) int64 { return a - b }, func(a, b float64) float64 { return a - b })
	case "*":
		return numericOp(l, r, func(a, b int64) int64 { return a * b }, func(a, b float64) float64 { return a * b })
	case "/":
		if isZero(r) {
			return value.Null(), generationf("expr: division by zero")
		}
		return numericOp(l, r, func(a, b int64) int64 { return a / b }, func(a, b float64) float64 { return a / b })
	case "%":
		if isZero(r) {
			return value.Null(), generationf("expr: modulo by zero")
		}
		return numericOp(l, r, func(a, b int64) int64 { return a % b }, func(a, b float64) float64 {
			n := int64(a / b)
			return a - float64(n)*b
		})
	}
	return value.Null(), generationf("expr: unknown op %q", op)
}

// evalComparison handles ==, !=, <, >, <= and >=.
func evalComparison(op string, l, r value.Value) (value.Value, error) {
	switch op {
	case "==":
		return value.Bool(valuesEqual(l, r)), nil
	case "!=":
		return value.Bool(!valuesEqual(l, r)), nil
	case "<":
		c, err := compareValues(l, r)
		if err != nil {
			return value.Null(), err
		}
		return value.Bool(c < 0), nil
	case ">":
		c, err := compareValues(l, r)
		if err != nil {
			return value.Null(), err
		}
		return value.Bool(c > 0), nil
	case "<=":
		c, err := compareValues(l, r)
		if err != nil {
			return value.Null(), err
		}
		return value.Bool(c <= 0), nil
	case ">=":
		c, err := compareValues(l, r)
		if err != nil {
			return value.Null(), err
		}
		return value.Bool(c >= 0), nil
	}
	return value.Null(), generationf("expr: unknown op %q", op)
}

// evalLogical handles the boolean and/or operators.
func evalLogical(op string, l, r value.Value) (value.Value, error) {
	switch op {
	case "and":
		return value.Bool(truthy(l) && truthy(r)), nil
	case "or":
		return value.Bool(truthy(l) || truthy(r)), nil
	}
	return value.Null(), generationf("expr: unknown op %q", op)
}

func numericOp(l, r value.Value, intOp func(a, b int64) int64, floatOp func(a, b float64) float64) (value.Value, error) {
	if l.Kind == value.KindInt && r.Kind == value.KindInt {
		return value.Int(intOp(l.I, r.I)), nil
	}
	la, okA := asFloat(l)
	ra, okB := asFloat(r)
	if !okA || !okB {
		return value.Null(), generationf("expr: non-numeric operand: %v, %v", l.Kind, r.Kind)
	}
	return value.Float(floatOp(la, ra)), nil
}

func isZero(v value.Value) bool {
	switch v.Kind {
	case value.KindInt:
		return v.I == 0
	case value.KindFloat:
		return v.F == 0
	default:
		// non-numeric kinds: handled by the trailing return
	}
	return false
}

func asFloat(v value.Value) (float64, bool) {
	switch v.Kind {
	case value.KindInt:
		return float64(v.I), true
	case value.KindFloat:
		return v.F, true
	case value.KindDecimal:
		f, _ := v.D.Float64()
		return f, true
	default:
		// non-numeric kinds: handled by the trailing return
	}
	return 0, false
}

func valuesEqual(a, b value.Value) bool {
	if a.Kind == b.Kind {
		switch a.Kind {
		case value.KindNull:
			return true
		case value.KindBool:
			return a.B == b.B
		case value.KindInt:
			return a.I == b.I
		case value.KindFloat:
			return a.F == b.F
		case value.KindString:
			return a.S == b.S
		default:
			// other kinds fall through to the cross-kind handling below
		}
	}
	// Numeric cross-kind equality.
	if fa, okA := asFloat(a); okA {
		if fb, okB := asFloat(b); okB {
			return fa == fb
		}
	}
	// Null vs anything — only equal if both null.
	if a.Kind == value.KindNull || b.Kind == value.KindNull {
		return a.Kind == b.Kind
	}
	return valueDisplay(a) == valueDisplay(b)
}

func compareValues(a, b value.Value) (int, error) {
	if fa, ok := asFloat(a); ok {
		if fb, okB := asFloat(b); okB {
			switch {
			case fa < fb:
				return -1, nil
			case fa > fb:
				return 1, nil
			}
			return 0, nil
		}
	}
	if a.Kind == value.KindString && b.Kind == value.KindString {
		return strings.Compare(a.S, b.S), nil
	}
	if a.Kind == value.KindTime && b.Kind == value.KindTime {
		switch {
		case a.T.Before(b.T):
			return -1, nil
		case a.T.After(b.T):
			return 1, nil
		}
		return 0, nil
	}
	return 0, generationf("expr: cannot compare %v and %v", a.Kind, b.Kind)
}

// truthy follows the same rules as the Rust evaluator.
func truthy(v value.Value) bool {
	switch v.Kind {
	case value.KindBool:
		return v.B
	case value.KindNull:
		return false
	case value.KindInt:
		return v.I != 0
	case value.KindFloat:
		return v.F != 0
	case value.KindString:
		return v.S != ""
	default:
		// other kinds fall through to the trailing return
	}
	return true
}

func evalIf(children []exprNode, env evalEnv) (value.Value, error) {
	if len(children) < 3 {
		return value.Null(), generationf("if() requires 3 arguments")
	}
	cond, err := evalExpr(children[0], env)
	if err != nil {
		return value.Null(), err
	}
	if truthy(cond) {
		return evalExpr(children[1], env)
	}
	return evalExpr(children[2], env)
}

// evalFunc implements the supported function table.
func evalFunc(name string, args []value.Value, children []exprNode, env evalEnv) (value.Value, error) {
	switch name {
	case "concat":
		var b strings.Builder
		for _, a := range args {
			b.WriteString(valueDisplay(a))
		}
		return value.Str(b.String()), nil
	case "sum":
		list := firstListOrEntity(args, children, env)
		sum := 0.0
		allInt := true
		for _, v := range list {
			if v.Kind != value.KindInt {
				allInt = false
			}
			if f, ok := asFloat(v); ok {
				sum += f
			}
		}
		if allInt {
			return value.Int(int64(sum)), nil
		}
		return value.Float(sum), nil
	case "count":
		list := firstListOrEntity(args, children, env)
		return value.Int(int64(len(list))), nil
	case "avg":
		list := firstListOrEntity(args, children, env)
		if len(list) == 0 {
			return value.Float(0), nil
		}
		sum := 0.0
		for _, v := range list {
			if f, ok := asFloat(v); ok {
				sum += f
			}
		}
		return value.Float(sum / float64(len(list))), nil
	case "min", "max":
		list := firstListOrEntity(args, children, env)
		if len(list) == 0 {
			return value.Null(), nil
		}
		best, _ := asFloat(list[0])
		for _, v := range list[1:] {
			f, ok := asFloat(v)
			if !ok {
				continue
			}
			if (name == "min" && f < best) || (name == "max" && f > best) {
				best = f
			}
		}
		return value.Float(best), nil
	case "years_since":
		if len(args) < 1 {
			return value.Int(0), nil
		}
		d := valueDisplay(args[0])
		return value.Int(yearsSince(d)), nil
	case "days_between":
		if len(args) < 2 {
			return value.Int(0), nil
		}
		return value.Int(daysBetween(valueDisplay(args[0]), valueDisplay(args[1]))), nil
	case "round":
		if len(args) < 1 {
			return value.Float(0), nil
		}
		f, _ := asFloat(args[0])
		places := int64(0)
		if len(args) >= 2 {
			if n, ok := asFloat(args[1]); ok {
				places = int64(n)
			}
		}
		return value.Float(roundTo(f, int(places))), nil
	case "lower":
		if len(args) < 1 {
			return value.Str(""), nil
		}
		return value.Str(strings.ToLower(valueDisplay(args[0]))), nil
	case "upper":
		if len(args) < 1 {
			return value.Str(""), nil
		}
		return value.Str(strings.ToUpper(valueDisplay(args[0]))), nil
	case "slug":
		if len(args) < 1 {
			return value.Str(""), nil
		}
		return value.Str(slugify(valueDisplay(args[0]))), nil
	case "starts_with":
		if len(args) < 2 {
			return value.Bool(false), nil
		}
		return value.Bool(strings.HasPrefix(valueDisplay(args[0]), valueDisplay(args[1]))), nil
	case "ends_with":
		if len(args) < 2 {
			return value.Bool(false), nil
		}
		return value.Bool(strings.HasSuffix(valueDisplay(args[0]), valueDisplay(args[1]))), nil
	}
	return value.Null(), generationf("unknown function %q", name)
}

// firstListOrEntity resolves the first argument either as an already-materialised
// list value or as an entity/entity.field path (which yields a list from env.data).
func firstListOrEntity(args []value.Value, children []exprNode, env evalEnv) []value.Value {
	if len(args) == 0 {
		return nil
	}
	if args[0].Kind == value.KindList {
		return args[0].L
	}
	// Path handling
	if len(children) > 0 && children[0].kind == exprPath {
		segments := strings.Split(children[0].str, ".")
		if len(segments) == 1 {
			if rows, ok := env.data[segments[0]]; ok {
				out := make([]value.Value, 0, len(rows))
				for _, r := range rows {
					out = append(out, firstField(r))
				}
				return out
			}
		}
		if len(segments) == 2 {
			if rows, ok := env.data[segments[0]]; ok {
				out := make([]value.Value, 0, len(rows))
				for _, r := range rows {
					if v, ok := r.Get(segments[1]); ok {
						out = append(out, v)
					}
				}
				return out
			}
		}
	}
	return []value.Value{args[0]}
}

func slugify(s string) string {
	lower := strings.ToLower(s)
	var b strings.Builder
	prevDash := true
	for _, r := range lower {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := b.String()
	out = strings.Trim(out, "-")
	return out
}

func yearsSince(date string) int64 {
	t, ok := parseDate(date)
	if !ok {
		return 0
	}
	n := now()
	years := int64(n.Year() - t.Year())
	if n.Month() < t.Month() || (n.Month() == t.Month() && n.Day() < t.Day()) {
		years--
	}
	if years < 0 {
		years = 0
	}
	return years
}

func daysBetween(a, b string) int64 {
	ta, okA := parseDate(a)
	tb, okB := parseDate(b)
	if !okA || !okB {
		return 0
	}
	diff := tb.Sub(ta).Hours() / 24
	if diff < 0 {
		diff = -diff
	}
	return int64(diff + 0.5)
}

func parseDate(s string) (t timelike, ok bool) {
	if len(s) >= 10 {
		s = s[:10]
	}
	return parseYMD(s)
}

// generationf is a small helper so parse/eval paths share a shape.
func generationf(format string, a ...any) *errors.Error {
	return &errors.Error{Kind: errors.KindGeneration, Message: fmt.Sprintf(format, a...)}
}
