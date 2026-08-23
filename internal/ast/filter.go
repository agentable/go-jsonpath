// Package ast defines the JSONPath abstract syntax tree.
package ast

import (
	"encoding/json/jsontext"
	"strconv"
	"strings"

	"maps"
	"slices"
)

// FilterExpr represents a filter expression tree (?logical-expr) per RFC 9535 §2.3.5.
type FilterExpr struct {
	Or LogicalOr // Or is the top-level logical-or expression.
}

// Eval evaluates the filter expression against the current node.
func (f *FilterExpr) Eval(current, root any) bool {
	return f.Or.Eval(current, root)
}

func (f *FilterExpr) writeTo(buf *strings.Builder) {
	if f == nil {
		return
	}
	f.Or.writeTo(buf)
}

// LogicalOr is a sequence of LogicalAnd expressions joined by ||.
// Short-circuits on first true.
type LogicalOr []LogicalAnd

// Eval returns true if any LogicalAnd expression is true.
func (lo LogicalOr) Eval(current, root any) bool {
	for i := range lo {
		if lo[i].Eval(current, root) {
			return true
		}
	}
	return false
}

func (lo LogicalOr) writeTo(buf *strings.Builder) {
	for i := range lo {
		if i > 0 {
			buf.WriteString(" || ")
		}
		lo[i].writeTo(buf)
	}
}

// LogicalAnd is a sequence of BasicExpr joined by &&.
// Short-circuits on first false.
type LogicalAnd []BasicExpr

// Eval returns true if all BasicExpr are true.
func (la LogicalAnd) Eval(current, root any) bool {
	for i := range la {
		if !la[i].Eval(current, root) {
			return false
		}
	}
	return true
}

func (la LogicalAnd) writeTo(buf *strings.Builder) {
	for i := range la {
		if i > 0 {
			buf.WriteString(" && ")
		}
		writeBasicExpr(buf, la[i])
	}
}

// BasicExpr is a filter expression that evaluates to a boolean.
type BasicExpr interface {
	Eval(current, root any) bool
}

type basicExprWriter interface {
	writeTo(*strings.Builder)
}

func writeBasicExpr(buf *strings.Builder, expr BasicExpr) {
	if expr == nil {
		return
	}
	if w, ok := expr.(basicExprWriter); ok {
		w.writeTo(buf)
		return
	}
	buf.WriteByte('?')
}

// ExistExpr tests if a query selects at least one node.
type ExistExpr struct {
	Query *PathQuery // Query is evaluated against the current filter context.
}

// Eval returns true if the query selects at least one node.
func (e *ExistExpr) Eval(current, root any) bool {
	return queryExists(e.Query, current, root)
}

func (e *ExistExpr) writeTo(buf *strings.Builder) {
	if e == nil || e.Query == nil {
		return
	}
	e.Query.writeTo(buf)
}

// NonExistExpr tests if a query selects no nodes.
type NonExistExpr struct {
	Query *PathQuery // Query is evaluated against the current filter context.
}

// Eval returns true if the query selects no nodes.
func (e *NonExistExpr) Eval(current, root any) bool {
	return !queryExists(e.Query, current, root)
}

func (e *NonExistExpr) writeTo(buf *strings.Builder) {
	buf.WriteByte('!')
	if e == nil || e.Query == nil {
		return
	}
	e.Query.writeTo(buf)
}

func queryExists(query *PathQuery, current, root any) bool {
	if len(query.Segments()) == 0 {
		return true
	}
	return len(query.Select(current, root)) > 0
}

// ParenExpr is a parenthesized logical expression.
type ParenExpr struct {
	Expr *LogicalOr // Expr is the enclosed logical expression.
}

// Eval evaluates the parenthesized expression.
func (p *ParenExpr) Eval(current, root any) bool {
	return p.Expr.Eval(current, root)
}

func (p *ParenExpr) writeTo(buf *strings.Builder) {
	buf.WriteByte('(')
	if p != nil && p.Expr != nil {
		p.Expr.writeTo(buf)
	}
	buf.WriteByte(')')
}

// NotParenExpr is a negated parenthesized logical expression.
type NotParenExpr struct {
	Expr *LogicalOr // Expr is the enclosed logical expression.
}

// Eval evaluates the negated parenthesized expression.
func (n *NotParenExpr) Eval(current, root any) bool {
	return !n.Expr.Eval(current, root)
}

func (n *NotParenExpr) writeTo(buf *strings.Builder) {
	buf.WriteString("!(")
	if n != nil && n.Expr != nil {
		n.Expr.writeTo(buf)
	}
	buf.WriteByte(')')
}

// NegFuncExpr is a negated logical function call expression (!match(), !search()).
type NegFuncExpr struct {
	Func *FuncExpr // Func is the logical function call being negated.
}

// Eval evaluates the negated function call.
func (n *NegFuncExpr) Eval(current, root any) bool {
	return !n.Func.Eval(current, root)
}

func (n *NegFuncExpr) writeTo(buf *strings.Builder) {
	buf.WriteByte('!')
	if n != nil && n.Func != nil {
		n.Func.writeTo(buf)
	}
}

// CompOp is a comparison operator.
type CompOp uint8

// CompOp values.
const (
	Equal        CompOp = iota // ==
	NotEqual                   // !=
	Less                       // <
	LessEqual                  // <=
	Greater                    // >
	GreaterEqual               // >=
)

func (op CompOp) String() string {
	switch op {
	case Equal:
		return "=="
	case NotEqual:
		return "!="
	case Less:
		return "<"
	case LessEqual:
		return "<="
	case Greater:
		return ">"
	case GreaterEqual:
		return ">="
	default:
		return "?"
	}
}

// CompExpr is a comparison expression.
type CompExpr struct {
	Left  CompValue // Left is the left operand.
	Op    CompOp    // Op is the comparison operator.
	Right CompValue // Right is the right operand.
}

// Eval evaluates the comparison expression.
func (c *CompExpr) Eval(current, root any) bool {
	left := c.Left.Value(current, root)
	right := c.Right.Value(current, root)

	switch c.Op {
	case Equal:
		return equalTo(left, right)
	case NotEqual:
		return !equalTo(left, right)
	case Less:
		return sameType(left, right) && lessThan(left, right)
	case LessEqual:
		return sameType(left, right) && (lessThan(left, right) || equalTo(left, right))
	case Greater:
		return sameType(left, right) && lessThan(right, left)
	case GreaterEqual:
		return sameType(left, right) && (lessThan(right, left) || equalTo(left, right))
	}
	return false
}

func (c *CompExpr) writeTo(buf *strings.Builder) {
	if c == nil {
		return
	}
	writeCompValue(buf, c.Left)
	buf.WriteByte(' ')
	buf.WriteString(c.Op.String())
	buf.WriteByte(' ')
	writeCompValue(buf, c.Right)
}

// CompValue represents a comparable value in a comparison expression.
type CompValue interface {
	Value(current, root any) runtimeValue
}

type compValueWriter interface {
	writeTo(*strings.Builder)
}

func writeCompValue(buf *strings.Builder, val CompValue) {
	if val == nil {
		return
	}
	if w, ok := val.(compValueWriter); ok {
		w.writeTo(buf)
		return
	}
	buf.WriteByte('?')
}

// LiteralValue is a literal value (string, number, bool, null).
type LiteralValue struct {
	Val any // Val is the literal JSON value.
}

// Value returns the literal value.
func (l *LiteralValue) Value(current, root any) runtimeValue {
	return runtimeValueFromAny(l.Val)
}

func (l *LiteralValue) writeTo(buf *strings.Builder) {
	if l == nil {
		return
	}
	writeLiteral(buf, l.Val)
}

// QueryValue is a singular query that produces a single value.
type QueryValue struct {
	Query *PathQuery // Query is evaluated against the current filter context.
}

// Value returns the first value selected by the query, or Nothing when the
// query does not select exactly one node.
func (q *QueryValue) Value(current, root any) runtimeValue {
	nodes := q.Query.Select(current, root)
	if len(nodes) != 1 {
		return nothingRuntimeValue()
	}
	return runtimeValueFromAny(nodes[0])
}

func (q *QueryValue) writeTo(buf *strings.Builder) {
	if q == nil || q.Query == nil {
		return
	}
	q.Query.writeTo(buf)
}

// jsonNull is a sentinel type representing a literal JSON null value.
type jsonNull struct{}

// JSONNull returns a sentinel value representing a literal JSON null.
func JSONNull() jsonNull {
	return jsonNull{}
}

// FuncValue is a function call that produces a value.
type FuncValue struct {
	Func *FuncExpr // Func is the function call to evaluate.
}

// Value returns the result of the function call.
func (f *FuncValue) Value(current, root any) runtimeValue {
	return runtimeValueFromFunctionValue(f.Func.Call(current, root))
}

func (f *FuncValue) writeTo(buf *strings.Builder) {
	if f == nil || f.Func == nil {
		return
	}
	f.Func.writeTo(buf)
}

func writeLiteral(buf *strings.Builder, val any) {
	switch v := val.(type) {
	case string:
		writeStringLiteral(buf, v)
	case bool:
		buf.WriteString(strconv.FormatBool(v))
	case nil, jsonNull:
		buf.WriteString("null")
	case int:
		buf.WriteString(strconv.Itoa(v))
	case int8:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	case int16:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	case int32:
		buf.WriteString(strconv.FormatInt(int64(v), 10))
	case int64:
		buf.WriteString(strconv.FormatInt(v, 10))
	case uint:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint64:
		buf.WriteString(strconv.FormatUint(v, 10))
	case float32:
		buf.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
	case float64:
		buf.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
	case jsontext.Value:
		buf.WriteString(string(v))
	default:
		buf.WriteByte('?')
	}
}

// sameType returns true if both values have compatible types for ordering comparison.
func sameType(a, b runtimeValue) bool {
	if a.kind != runtimeJSON || b.kind != runtimeJSON {
		return false
	}

	aIsNull := a.value == nil
	bIsNull := b.value == nil

	if aIsNull || bIsNull {
		return aIsNull && bIsNull
	}

	if isNumeric(a.value) && isNumeric(b.value) {
		return true
	}

	switch a.value.(type) {
	case string:
		_, ok := b.value.(string)
		return ok
	case bool:
		_, ok := b.value.(bool)
		return ok
	default:
		return false
	}
}

// equalTo returns true if a equals b, with numeric type coercion and deep equality.
func equalTo(a, b runtimeValue) bool {
	if a.kind == runtimeNothing || b.kind == runtimeNothing {
		return a.kind == runtimeNothing && b.kind == runtimeNothing
	}
	if a.kind != runtimeJSON || b.kind != runtimeJSON {
		return false
	}
	return jsonEqual(a.value, b.value)
}

func jsonEqual(a, b any) bool {
	a = normalizeJSONNull(a)
	b = normalizeJSONNull(b)

	aIsNumber := isNumberType(a)
	bIsNumber := isNumberType(b)
	if aIsNumber || bIsNumber {
		if !aIsNumber || !bIsNumber {
			return false
		}
		order, ok := compareJSONNumbers(a, b)
		return ok && order == 0
	}

	aArr, aIsArr := a.([]any)
	bArr, bIsArr := b.([]any)
	if aIsArr && bIsArr {
		return slices.EqualFunc(aArr, bArr, jsonEqual)
	}

	aObj, aIsObj := a.(map[string]any)
	bObj, bIsObj := b.(map[string]any)
	if aIsObj && bIsObj {
		return maps.EqualFunc(aObj, bObj, jsonEqual)
	}

	if aIsArr || bIsArr || aIsObj || bIsObj {
		return false
	}

	return a == b
}

func normalizeJSONNull(v any) any {
	if _, ok := v.(jsonNull); ok {
		return nil
	}
	return v
}

// lessThan returns true if a < b. Assumes sameType(a, b) is true.
func lessThan(a, b runtimeValue) bool {
	if a.kind != runtimeJSON || b.kind != runtimeJSON {
		return false
	}
	if a.value == nil || b.value == nil {
		return false
	}

	if order, ok := compareJSONNumbers(a.value, b.value); ok {
		return order < 0
	}

	sa, aIsString := a.value.(string)
	sb, bIsString := b.value.(string)
	return aIsString && bIsString && sa < sb
}
