package ast

import (
	"errors"
	"fmt"
	"strings"
)

// FuncType describes the return type of a function expression per RFC 9535 §2.4.1.
type FuncType uint8

const (
	// Logical indicates the function returns a logical (bool) value.
	Logical FuncType = iota
	// Value indicates the function returns a single JSON value.
	Value
	// Nodes indicates the function returns a node list.
	Nodes
)

// String returns the string representation of ft.
func (ft FuncType) String() string {
	switch ft {
	case Logical:
		return "Logical"
	case Value:
		return "Value"
	case Nodes:
		return "Nodes"
	default:
		return fmt.Sprintf("FuncType(%d)", ft)
	}
}

// FunctionValue is one of the RFC 9535 function runtime value types.
type FunctionValue interface {
	functionValue()
	ResultType() FuncType
}

// TypedValue is a JSON value or the absence of a value.
type TypedValue struct {
	value any
	valid bool
}

// NewValue returns a present JSON value for function execution.
func NewValue(value any) TypedValue {
	return TypedValue{value: value, valid: true}
}

// NoValue returns an absent value for function execution.
func NoValue() TypedValue {
	return TypedValue{}
}

func (TypedValue) functionValue() {}

// ResultType returns [Value].
func (TypedValue) ResultType() FuncType { return Value }

// Any returns the underlying JSON value. It returns nil for both JSON null
// and an absent value; use [TypedValue.IsNothing] to distinguish them.
func (v TypedValue) Any() any { return v.value }

// IsNothing reports whether v is absent rather than JSON null.
func (v TypedValue) IsNothing() bool { return !v.valid }

// TypedLogical is a logical function result.
type TypedLogical bool

func (TypedLogical) functionValue() {}

// ResultType returns [Logical].
func (TypedLogical) ResultType() FuncType { return Logical }

// Bool returns l as a bool.
func (l TypedLogical) Bool() bool { return bool(l) }

// TypedNodes is a function node-list value.
type TypedNodes []any

func (TypedNodes) functionValue() {}

// ResultType returns [Nodes].
func (TypedNodes) ResultType() FuncType { return Nodes }

func typedValueFromAny(v any) TypedValue {
	return typedValueFromRuntimeValue(runtimeValueFromAny(v))
}

// Function defines a function that can be called in filter expressions.
// Implementations must be safe for concurrent use.
type Function interface {
	// Name returns the function name as used in JSONPath expressions.
	Name() string
	// ResultType returns the FuncType of the function's return value.
	ResultType() FuncType
	// ParameterCount returns the function's fixed arity.
	ParameterCount() int
	// ParameterType returns the semantic type of parameter index.
	ParameterType(index int) FuncType
	// Call evaluates the function at query time and returns the result.
	Call(args []FunctionValue) FunctionValue
}

// FuncExpr represents a function call in a filter expression per RFC 9535 §2.4.
type FuncExpr struct {
	name string   // function name
	fn   Function // resolved function definition
	args []any    // argument expressions
}

// NewFuncExpr creates a [FuncExpr] for the given function and arguments.
func NewFuncExpr(fn Function, args ...any) *FuncExpr {
	return &FuncExpr{name: fn.Name(), fn: fn, args: args}
}

// Name returns the function name.
func (fe *FuncExpr) Name() string { return fe.name }

// Func returns the resolved [Function].
func (fe *FuncExpr) Func() Function { return fe.fn }

// Args returns the argument expressions.
func (fe *FuncExpr) Args() []any { return fe.args }

// ResultType returns the return type of the underlying function.
func (fe *FuncExpr) ResultType() FuncType { return fe.fn.ResultType() }

// Call evaluates the function with the given current and root nodes.
// It evaluates argument expressions and passes the results to the underlying function.
func (fe *FuncExpr) Call(current, root any) FunctionValue {
	evalArgs := make([]FunctionValue, len(fe.args))
	for i, arg := range fe.args {
		evalArgs[i] = fe.evalArg(i, arg, current, root)
	}
	return fe.fn.Call(evalArgs)
}

func (fe *FuncExpr) evalArg(index int, arg, current, root any) FunctionValue {
	switch a := arg.(type) {
	case *PathQuery:
		return fe.evalPathQueryArg(index, a, current, root)
	case *FuncExpr:
		return a.Call(current, root)
	case LogicalOr:
		return TypedLogical(a.Eval(current, root))
	case CompValue:
		return typedValueFromRuntimeValue(a.Value(current, root))
	default:
		return typedValueFromAny(arg)
	}
}

func (fe *FuncExpr) evalPathQueryArg(index int, query *PathQuery, current, root any) FunctionValue {
	nodes := query.Select(current, root)
	if index < fe.fn.ParameterCount() && fe.fn.ParameterType(index) == Nodes {
		return TypedNodes(nodes)
	}
	if !query.IsSingular() {
		return TypedNodes(nodes)
	}
	if len(nodes) == 1 {
		return NewValue(nodes[0])
	}
	return NoValue()
}

// Eval implements BasicExpr for logical functions.
// Returns false if the function is not a logical function.
func (fe *FuncExpr) Eval(current, root any) bool {
	result := runtimeValueFromFunctionValue(fe.Call(current, root))
	switch fe.fn.ResultType() {
	case Logical:
		return result.kind == runtimeLogical && result.logical
	case Nodes:
		return result.kind == runtimeNodes && len(result.nodes) > 0
	default:
		return false
	}
}

// writeTo writes the canonical string representation of fe to buf.
func (fe *FuncExpr) writeTo(buf *strings.Builder) {
	buf.WriteString(fe.name)
	buf.WriteByte('(')
	for i, arg := range fe.args {
		if i > 0 {
			buf.WriteString(", ")
		}
		writeFunctionArg(buf, arg)
	}
	buf.WriteByte(')')
}

func writeFunctionArg(buf *strings.Builder, arg any) {
	switch a := arg.(type) {
	case *PathQuery:
		a.writeTo(buf)
	case *FuncExpr:
		a.writeTo(buf)
	case CompValue:
		writeCompValue(buf, a)
	case LogicalOr:
		a.writeTo(buf)
	case LogicalAnd:
		a.writeTo(buf)
	case BasicExpr:
		writeBasicExpr(buf, a)
	default:
		writeLiteral(buf, a)
	}
}

// String returns the canonical string representation of fe.
func (fe *FuncExpr) String() string {
	var buf strings.Builder
	fe.writeTo(&buf)
	return buf.String()
}

// ErrArgCount indicates a function received the wrong number of arguments.
var ErrArgCount = errors.New("wrong number of arguments")

// ErrArgType indicates an incompatible function argument type.
var ErrArgType = errors.New("incompatible argument type")
