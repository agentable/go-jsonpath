// Package parser provides a recursive descent parser for RFC 9535 JSONPath
// expressions. It consumes tokens from the lexer and produces an AST.
package parser

import (
	jsonv1 "encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/agentable/go-jsonpath/internal/ast"
	"github.com/agentable/go-jsonpath/internal/lexer"
)

var (
	// ErrParseEnd is returned when a parse error occurs at the end of input.
	ErrParseEnd = errors.New("parse error at end")
	// ErrParsePosition is returned when a parse error occurs at a specific position.
	ErrParsePosition = errors.New("parse error at position")
	// ErrFunction is returned when function lookup or validation fails at parse time.
	ErrFunction = errors.New("function error")
	// ErrUnknownFunction is returned when an unknown function is referenced.
	ErrUnknownFunction = errors.New("unknown function")
)

// Error describes a parser failure with source location and underlying cause.
type Error struct {
	Offset  int
	Reason  string
	Snippet string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Snippet != "" {
		return fmt.Sprintf("%s at position %d near %s", e.Reason, e.Offset, e.Snippet)
	}
	return fmt.Sprintf("%s at position %d", e.Reason, e.Offset)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

const maxJSONInteger int64 = 1<<53 - 1

// Parser parses JSONPath expressions into AST nodes.
type Parser struct {
	src    string
	tokens []lexer.Token
	pos    int
	funcs  map[string]ast.Function

	trackSingular          bool
	firstNonSingularOffset int
}

// New creates a new Parser for the given source string.
func New(src string, funcs map[string]ast.Function) (*Parser, error) {
	lex := lexer.New(src)
	// Typical JSONPath expressions need about one token per 3-4 characters.
	tokens := make([]lexer.Token, 0, len(src)/3+1)
	for {
		tok := lex.Scan()
		tokens = append(tokens, tok)
		if tok.Kind == lexer.EOF || tok.Kind == lexer.Invalid {
			break
		}
	}

	if len(tokens) > 0 && tokens[len(tokens)-1].Kind == lexer.Invalid {
		tok := tokens[len(tokens)-1]
		return nil, &Error{
			Offset:  tok.Start,
			Reason:  tok.Value,
			Snippet: contextSnippet(src, tok.Start),
			Cause:   errors.Join(ErrParsePosition, tok.Err()),
		}
	}

	return &Parser{
		src:    src,
		tokens: tokens,
		pos:    0,
		funcs:  funcs,
	}, nil
}

// isBlankSpace reports whether b is RFC 9535 blank space (SP / HTAB / LF / CR).
func isBlankSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// Parse parses a JSONPath query and returns the AST.
func (p *Parser) Parse() (*ast.PathQuery, error) {
	if len(p.src) > 0 && isBlankSpace(p.src[0]) {
		return nil, p.errorAt(0, "leading whitespace not allowed", ErrParsePosition)
	}
	if len(p.src) > 0 && isBlankSpace(p.src[len(p.src)-1]) {
		return nil, p.errorAt(len(p.src)-1, "trailing whitespace not allowed", ErrParsePosition)
	}

	if !p.match(lexer.Dollar) {
		return nil, p.error("expected $")
	}

	query, err := p.parseQueryAfterRoot(true)
	if err != nil {
		return nil, err
	}

	if !p.isAtEnd() {
		return nil, p.error("unexpected token after path")
	}

	return query, nil
}

// parseSegments parses zero or more segments.
func (p *Parser) parseSegments() ([]ast.Segment, error) {
	var segments []ast.Segment

	for !p.isAtEnd() {
		switch {
		case p.match(lexer.DotDot):
			sel, err := p.parseDescendantSegment()
			if err != nil {
				return nil, err
			}
			segments = append(segments, sel)
		case p.match(lexer.LeftBracket):
			sel, err := p.parseBracketedSelection()
			if err != nil {
				return nil, err
			}
			segments = append(segments, ast.Child(sel...))
		case p.match(lexer.Dot):
			sel, err := p.parseDotChild()
			if err != nil {
				return nil, err
			}
			segments = append(segments, ast.Child(sel))
		default:
			return segments, nil
		}
	}

	return segments, nil
}

// parseDescendantSegment parses a descendant segment after "..".
func (p *Parser) parseDescendantSegment() (ast.Segment, error) {
	p.markNonSingular(p.previous().Start)
	if err := p.rejectWhitespaceAfterPrevious("whitespace not allowed after .."); err != nil {
		return ast.Segment{}, err
	}

	switch {
	case p.match(lexer.LeftBracket):
		sel, err := p.parseBracketedSelection()
		if err != nil {
			return ast.Segment{}, err
		}
		return ast.Descendant(sel...), nil
	case p.match(lexer.Star):
		return ast.Descendant(ast.WildcardSelector()), nil
	case p.checkMemberName():
		name := p.advance().Val(p.src)
		return ast.Descendant(ast.NameSelector(name)), nil
	default:
		return ast.Segment{}, p.error("expected [, *, or identifier after ..")
	}
}

// parseDotChild parses a dot-child selector (. followed by * or identifier).
func (p *Parser) parseDotChild() (ast.Selector, error) {
	if err := p.rejectWhitespaceAfterPrevious("whitespace not allowed after ."); err != nil {
		return ast.Selector{}, err
	}

	if p.match(lexer.Star) {
		p.markNonSingular(p.previous().Start)
		return ast.WildcardSelector(), nil
	}
	if p.checkMemberName() {
		name := p.advance().Val(p.src)
		return ast.NameSelector(name), nil
	}
	return ast.Selector{}, p.error("expected * or identifier after .")
}

func (p *Parser) checkMemberName() bool {
	switch p.peek().Kind {
	case lexer.Ident, lexer.True, lexer.False, lexer.Null:
		return true
	default:
		return false
	}
}

// parseBracketedSelection parses selectors inside brackets.
func (p *Parser) parseBracketedSelection() ([]ast.Selector, error) {
	var selectors []ast.Selector

	for {
		sel, err := p.parseSelector()
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, sel)

		if !p.match(lexer.Comma) {
			break
		}
		p.markNonSingular(p.previous().Start)
	}

	if !p.match(lexer.RightBracket) {
		return nil, p.error("expected ] or ,")
	}

	return selectors, nil
}

// parseSelector parses a single selector.
func (p *Parser) parseSelector() (ast.Selector, error) {
	if p.match(lexer.Star) {
		p.markNonSingular(p.previous().Start)
		return ast.WildcardSelector(), nil
	}

	if p.match(lexer.Question) {
		p.markNonSingular(p.previous().Start)
		expr, err := p.parseFilterExpr()
		if err != nil {
			return ast.Selector{}, err
		}
		return ast.FilterSelector(expr), nil
	}

	if p.check(lexer.String) {
		name := p.advance().Value
		return ast.NameSelector(name), nil
	}

	if p.check(lexer.Int) {
		return p.parseIndexOrSlice()
	}

	if p.match(lexer.Colon) {
		p.markNonSingular(p.previous().Start)
		return p.parseSlice(0, false)
	}

	return ast.Selector{}, p.error("expected selector")
}

// parseFilterExpr parses a filter expression: logical-or-expr
func (p *Parser) parseFilterExpr() (*ast.FilterExpr, error) {
	or, err := p.parseLogicalOr()
	if err != nil {
		return nil, err
	}
	return &ast.FilterExpr{Or: or}, nil
}

// parseLogicalOr parses: logical-and-expr *( "||" logical-and-expr )
func (p *Parser) parseLogicalOr() (ast.LogicalOr, error) {
	var ands []ast.LogicalAnd

	and, err := p.parseLogicalAnd()
	if err != nil {
		return nil, err
	}
	ands = append(ands, and)

	for p.match(lexer.Or) {
		and, err := p.parseLogicalAnd()
		if err != nil {
			return nil, err
		}
		ands = append(ands, and)
	}

	return ands, nil
}

// parseLogicalAnd parses: basic-expr *( "&&" basic-expr )
func (p *Parser) parseLogicalAnd() (ast.LogicalAnd, error) {
	var exprs []ast.BasicExpr

	expr, err := p.parseBasicExpr()
	if err != nil {
		return nil, err
	}
	exprs = append(exprs, expr)

	for p.match(lexer.And) {
		expr, err := p.parseBasicExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
	}

	return exprs, nil
}

// parseBasicExpr parses: paren-expr / comparison-expr / test-expr
func (p *Parser) parseBasicExpr() (ast.BasicExpr, error) {
	if p.match(lexer.Not) {
		if p.match(lexer.LeftParen) {
			or, err := p.parseLogicalOr()
			if err != nil {
				return nil, err
			}
			if !p.match(lexer.RightParen) {
				return nil, p.error("expected )")
			}
			return &ast.NotParenExpr{Expr: &or}, nil
		}
		if p.check(lexer.Ident) {
			fe, err := p.parseFunctionExpr()
			if err != nil {
				return nil, err
			}
			if fe.Func().ResultType() == ast.Value {
				return nil, p.error("value function cannot be negated")
			}
			return &ast.NegFuncExpr{Func: fe}, nil
		}
		query, err := p.parseFilterQuery()
		if err != nil {
			return nil, err
		}
		return &ast.NonExistExpr{Query: query}, nil
	}

	if p.match(lexer.LeftParen) {
		or, err := p.parseLogicalOr()
		if err != nil {
			return nil, err
		}
		if !p.match(lexer.RightParen) {
			return nil, p.error("expected )")
		}
		return &ast.ParenExpr{Expr: &or}, nil
	}

	if p.check(lexer.Ident) {
		fe, err := p.parseFunctionExpr()
		if err != nil {
			return nil, err
		}

		if p.checkCompOp() {
			// RFC 9535: only value function results are comparable.
			if fe.Func().ResultType() != ast.Value {
				return nil, p.error("non-value function result cannot be compared")
			}
			op := p.parseCompOp()
			right, err := p.parseCompValue()
			if err != nil {
				return nil, err
			}
			return &ast.CompExpr{
				Left:  &ast.FuncValue{Func: fe},
				Op:    op,
				Right: right,
			}, nil
		}

		if fe.Func().ResultType() == ast.Value {
			return nil, p.errorAt(
				p.peek().Start,
				"value function must be used in comparison",
				errors.Join(ErrParsePosition, ErrFunction),
			)
		}
		return fe, nil
	}

	if p.check(lexer.At) || p.check(lexer.Dollar) {
		return p.parseTestOrComparison()
	}

	if p.check(lexer.String) || p.check(lexer.Int) || p.check(lexer.Number) ||
		p.check(lexer.True) || p.check(lexer.False) || p.check(lexer.Null) {
		return p.parseComparisonFromLiteral()
	}

	return nil, p.error("expected filter expression")
}

// parseTestOrComparison parses a test expression or comparison starting with @ or $
func (p *Parser) parseTestOrComparison() (ast.BasicExpr, error) {
	query, firstNonSingular, err := p.parseFilterQueryWithSingularOffset()
	if err != nil {
		return nil, err
	}

	if p.checkCompOp() {
		if firstNonSingular >= 0 || !query.IsSingular() {
			if firstNonSingular < 0 {
				return nil, p.error("non-singular query is not allowed in comparison")
			}
			return nil, p.nonSingularComparisonErrorAt(firstNonSingular)
		}

		op := p.parseCompOp()
		right, err := p.parseCompValue()
		if err != nil {
			return nil, err
		}
		return &ast.CompExpr{
			Left:  &ast.QueryValue{Query: query},
			Op:    op,
			Right: right,
		}, nil
	}

	return &ast.ExistExpr{Query: query}, nil
}

// parseComparisonFromLiteral parses a comparison starting with a literal
func (p *Parser) parseComparisonFromLiteral() (ast.BasicExpr, error) {
	left, err := p.parseLiteralValue()
	if err != nil {
		return nil, err
	}

	if !p.checkCompOp() {
		return nil, p.error("expected comparison operator")
	}

	op := p.parseCompOp()
	right, err := p.parseCompValue()
	if err != nil {
		return nil, err
	}

	return &ast.CompExpr{
		Left:  &ast.LiteralValue{Val: left},
		Op:    op,
		Right: right,
	}, nil
}

// parseFunctionExpr parses a function call.
func (p *Parser) parseFunctionExpr() (*ast.FuncExpr, error) {
	nameToken := p.advance()
	name := nameToken.Val(p.src)

	if !p.isAtEnd() && nameToken.End < p.peek().Start {
		return nil, p.error("whitespace not allowed between function name and (")
	}

	if !p.match(lexer.LeftParen) {
		return nil, p.error("expected ( after function name")
	}

	funcObj, ok := p.funcs[name]
	if !ok {
		return nil, p.errorAt(nameToken.Start, "unknown function "+name, errors.Join(ErrParsePosition, ErrFunction, ErrUnknownFunction))
	}

	var args []any
	if !p.check(lexer.RightParen) {
		for {
			if len(args) >= funcObj.ParameterCount() {
				err := fmt.Errorf(
					"expected %d, got at least %d: %w",
					funcObj.ParameterCount(), len(args)+1, ast.ErrArgCount,
				)
				return nil, p.errorAt(
					nameToken.Start,
					name+": "+err.Error(),
					errors.Join(ErrParsePosition, ErrFunction, err),
				)
			}
			arg, err := p.parseFunctionArg(funcObj, len(args))
			if err != nil {
				return nil, err
			}
			args = append(args, arg)

			if !p.match(lexer.Comma) {
				break
			}
		}
	}

	if !p.match(lexer.RightParen) {
		return nil, p.error("expected )")
	}

	if err := checkFunctionArguments(funcObj, args); err != nil {
		return nil, p.errorAt(nameToken.Start, name+": "+err.Error(), errors.Join(ErrParsePosition, ErrFunction, err))
	}

	return ast.NewFuncExpr(funcObj, args...), nil
}

func checkFunctionArguments(fn ast.Function, args []any) error {
	if len(args) != fn.ParameterCount() {
		return fmt.Errorf("expected %d, got %d: %w", fn.ParameterCount(), len(args), ast.ErrArgCount)
	}
	for i, arg := range args {
		target := fn.ParameterType(i)
		if !functionArgConvertsTo(arg, target) {
			return fmt.Errorf("argument %d cannot convert to %s: %w", i+1, target, ast.ErrArgType)
		}
	}
	return nil
}

func functionArgConvertsTo(arg any, target ast.FuncType) bool {
	switch arg := arg.(type) {
	case *ast.PathQuery:
		return target == ast.Nodes || (target == ast.Value && arg.IsSingular())
	case *ast.FuncExpr:
		return arg.ResultType() == target
	case ast.LogicalOr, ast.LogicalAnd, ast.BasicExpr:
		return target == ast.Logical
	default:
		return target == ast.Value
	}
}

// parseFunctionArg parses a function argument using its declared parameter type.
func (p *Parser) parseFunctionArg(fn ast.Function, index int) (any, error) {
	target := fn.ParameterType(index)
	if p.startsExplicitLogicalFunctionArgument() {
		return p.parseLogicalOr()
	}
	if p.check(lexer.At) || p.check(lexer.Dollar) {
		if target == ast.Logical {
			return p.parseLogicalOr()
		}
		start := p.pos
		query, err := p.parseFilterQuery()
		if err != nil {
			return nil, err
		}
		if p.continuesLogicalExpression() {
			p.pos = start
			return p.parseLogicalOr()
		}
		return query, nil
	}

	if p.check(lexer.Ident) {
		if target == ast.Logical {
			return p.parseLogicalOr()
		}
		start := p.pos
		fn, err := p.parseFunctionExpr()
		if err != nil {
			return nil, err
		}
		if p.continuesLogicalExpression() {
			p.pos = start
			return p.parseLogicalOr()
		}
		return fn, nil
	}

	return p.parseLiteralValue()
}

func (p *Parser) startsExplicitLogicalFunctionArgument() bool {
	switch p.peek().Kind {
	case lexer.Not, lexer.LeftParen:
		return true
	case lexer.String, lexer.Int, lexer.Number, lexer.True, lexer.False, lexer.Null:
		if p.pos+1 >= len(p.tokens) {
			return false
		}
		return isCompOp(p.tokens[p.pos+1].Kind)
	default:
		return false
	}
}

func (p *Parser) continuesLogicalExpression() bool {
	return p.checkCompOp() || p.check(lexer.And) || p.check(lexer.Or)
}

// parseFilterQuery parses a query starting with @ or $
func (p *Parser) parseFilterQuery() (*ast.PathQuery, error) {
	if !p.match(lexer.Dollar) && !p.match(lexer.At) {
		return nil, p.error("expected $ or @")
	}

	return p.parseQueryAfterRoot(p.previous().Kind == lexer.Dollar)
}

func (p *Parser) parseQueryAfterRoot(isRoot bool) (*ast.PathQuery, error) {
	segments, err := p.parseSegments()
	if err != nil {
		return nil, err
	}

	return ast.NewPathQuery(isRoot, segments...), nil
}

func (p *Parser) parseFilterQueryWithSingularOffset() (*ast.PathQuery, int, error) {
	wasTracking := p.trackSingular
	previousOffset := p.firstNonSingularOffset
	p.trackSingular = true
	p.firstNonSingularOffset = -1

	query, err := p.parseFilterQuery()
	firstNonSingular := p.firstNonSingularOffset

	p.trackSingular = wasTracking
	p.firstNonSingularOffset = previousOffset
	return query, firstNonSingular, err
}

// parseCompValue parses a comparable value (literal, query, or function)
func (p *Parser) parseCompValue() (ast.CompValue, error) {
	if p.check(lexer.Ident) {
		fe, err := p.parseFunctionExpr()
		if err != nil {
			return nil, err
		}
		// RFC 9535: only value function results are comparable.
		if fe.Func().ResultType() != ast.Value {
			return nil, p.error("non-value function result cannot be compared")
		}
		return &ast.FuncValue{Func: fe}, nil
	}

	if p.check(lexer.At) || p.check(lexer.Dollar) {
		query, firstNonSingular, err := p.parseFilterQueryWithSingularOffset()
		if err != nil {
			return nil, err
		}
		if firstNonSingular >= 0 || !query.IsSingular() {
			if firstNonSingular < 0 {
				return nil, p.error("non-singular query is not allowed in comparison")
			}
			return nil, p.nonSingularComparisonErrorAt(firstNonSingular)
		}
		return &ast.QueryValue{Query: query}, nil
	}

	val, err := p.parseLiteralValue()
	if err != nil {
		return nil, err
	}
	return &ast.LiteralValue{Val: val}, nil
}

// parseLiteralValue parses a literal value
func (p *Parser) parseLiteralValue() (any, error) {
	if p.match(lexer.String) {
		return p.previous().Value, nil
	}
	if p.check(lexer.Int) {
		tok := p.advance()
		n, err := strconv.ParseInt(tok.Val(p.src), 10, 64)
		if err != nil {
			return nil, p.errorAt(tok.Start, "invalid integer", errors.Join(ErrParsePosition, err))
		}
		return n, nil
	}
	if p.check(lexer.Number) {
		tok := p.advance()
		s := tok.Val(p.src)
		_, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, p.errorAt(tok.Start, "invalid number", errors.Join(ErrParsePosition, err))
		}
		return jsonv1.Number(s), nil
	}
	if p.match(lexer.True) {
		return true, nil
	}
	if p.match(lexer.False) {
		return false, nil
	}
	if p.match(lexer.Null) {
		return ast.JSONNull(), nil
	}
	return nil, p.error("expected literal value")
}

func (p *Parser) checkCompOp() bool {
	return isCompOp(p.peek().Kind)
}

func isCompOp(kind lexer.Kind) bool {
	switch kind {
	case lexer.Equal, lexer.NotEqual, lexer.Less, lexer.LessEqual, lexer.Greater, lexer.GreaterEqual:
		return true
	default:
		return false
	}
}

func (p *Parser) parseCompOp() ast.CompOp {
	if p.match(lexer.Equal) {
		return ast.Equal
	}
	if p.match(lexer.NotEqual) {
		return ast.NotEqual
	}
	if p.match(lexer.Less) {
		return ast.Less
	}
	if p.match(lexer.LessEqual) {
		return ast.LessEqual
	}
	if p.match(lexer.Greater) {
		return ast.Greater
	}
	if p.match(lexer.GreaterEqual) {
		return ast.GreaterEqual
	}
	return ast.Equal // shouldn't reach here
}

// parseIndexOrSlice parses an index or slice selector starting with an integer.
func (p *Parser) parseIndexOrSlice() (ast.Selector, error) {
	start, err := p.parseJSONInteger(p.advance())
	if err != nil {
		return ast.Selector{}, err
	}

	if p.match(lexer.Colon) {
		p.markNonSingular(p.previous().Start)
		return p.parseSlice(start, true)
	}

	return ast.IndexSelector(start), nil
}

func (p *Parser) parseJSONInteger(tok lexer.Token) (int64, error) {
	text := tok.Val(p.src)
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, p.errorAt(tok.Start, "invalid integer", errors.Join(ErrParsePosition, err))
	}
	if n == 0 && text[0] == '-' {
		return 0, p.errorAt(tok.Start, "-0 is not allowed", ErrParsePosition)
	}
	if n < -maxJSONInteger || n > maxJSONInteger {
		return 0, p.errorAt(tok.Start, "index out of range", ErrParsePosition)
	}
	return n, nil
}

func (p *Parser) markNonSingular(offset int) {
	if p.trackSingular && p.firstNonSingularOffset < 0 {
		p.firstNonSingularOffset = offset
	}
}

func (p *Parser) nonSingularComparisonErrorAt(offset int) error {
	return p.errorAt(offset, "non-singular query is not allowed in comparison", ErrParsePosition)
}

func (p *Parser) parseSlice(start int64, hasStart bool) (ast.Selector, error) {
	args := ast.SliceArgs{
		Start:    start,
		HasStart: hasStart,
	}

	if p.check(lexer.Int) {
		end, err := p.parseJSONInteger(p.advance())
		if err != nil {
			return ast.Selector{}, err
		}
		args.End = end
		args.HasEnd = true
	}

	if p.match(lexer.Colon) && p.check(lexer.Int) {
		step, err := p.parseJSONInteger(p.advance())
		if err != nil {
			return ast.Selector{}, err
		}
		args.Step = step
		args.HasStep = true
	}

	return ast.SliceSelector(args), nil
}

func (p *Parser) match(kinds ...lexer.Kind) bool {
	if p.isAtEnd() || !slices.Contains(kinds, p.peek().Kind) {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) check(kind lexer.Kind) bool {
	if p.isAtEnd() {
		return false
	}
	return p.peek().Kind == kind
}

func (p *Parser) advance() lexer.Token {
	if !p.isAtEnd() {
		p.pos++
	}
	return p.previous()
}

func (p *Parser) isAtEnd() bool {
	return p.pos >= len(p.tokens) || p.peek().Kind == lexer.EOF
}

func (p *Parser) peek() lexer.Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return lexer.Token{Kind: lexer.EOF}
}

func (p *Parser) previous() lexer.Token {
	if p.pos > 0 && p.pos <= len(p.tokens) {
		return p.tokens[p.pos-1]
	}
	return lexer.Token{Kind: lexer.Invalid}
}

func (p *Parser) rejectWhitespaceAfterPrevious(msg string) error {
	prev := p.previous()
	if p.isAtEnd() || prev.End >= p.peek().Start {
		return nil
	}
	return p.error(msg)
}

func (p *Parser) error(msg string) error {
	tok := p.peek()
	if tok.Kind == lexer.EOF {
		return p.errorAt(len(p.src), msg, ErrParseEnd)
	}
	return p.errorAt(tok.Start, msg, ErrParsePosition)
}

func (p *Parser) errorAt(pos int, msg string, cause error) error {
	return &Error{
		Offset:  pos,
		Reason:  msg,
		Snippet: contextSnippet(p.src, pos),
		Cause:   cause,
	}
}

// contextSnippet returns a short source snippet around pos for error messages.
func contextSnippet(src string, pos int) string {
	const radius = 15
	start := max(0, pos-radius)
	end := min(len(src), pos+radius)

	var buf strings.Builder
	buf.WriteString("'")
	if start > 0 {
		buf.WriteString("...")
	}
	buf.WriteString(src[start:end])
	if end < len(src) {
		buf.WriteString("...")
	}
	buf.WriteString("'")
	return buf.String()
}
