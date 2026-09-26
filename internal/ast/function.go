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

type functionArgumentSource uint8

const (
	functionArgumentLiteral functionArgumentSource = iota
	functionArgumentQuery
	functionArgumentLogical
	functionArgumentFunction
)

type functionArgument struct {
	kind     FuncType
	source   functionArgumentSource
	literal  any
	query    *PathQuery
	logical  LogicalOr
	function *FuncExpr
}

func newFunctionArgument(kind FuncType, arg any) (functionArgument, error) {
	compiled := functionArgument{kind: kind}
	switch arg := arg.(type) {
	case *PathQuery:
		if kind != Nodes && (kind != Value || !arg.IsSingular()) {
			return functionArgument{}, ErrArgType
		}
		compiled.source = functionArgumentQuery
		compiled.query = arg
	case *FuncExpr:
		if arg.ResultType() != kind {
			return functionArgument{}, ErrArgType
		}
		compiled.source = functionArgumentFunction
		compiled.function = arg
	case LogicalOr:
		if kind != Logical {
			return functionArgument{}, ErrArgType
		}
		compiled.source = functionArgumentLogical
		compiled.logical = arg
	case LogicalAnd:
		if kind != Logical {
			return functionArgument{}, ErrArgType
		}
		compiled.source = functionArgumentLogical
		compiled.logical = LogicalOr{arg}
	case BasicExpr:
		if kind != Logical {
			return functionArgument{}, ErrArgType
		}
		compiled.source = functionArgumentLogical
		compiled.logical = LogicalOr{LogicalAnd{arg}}
	case CompValue:
		return functionArgument{}, ErrArgType
	default:
		if kind != Value {
			return functionArgument{}, ErrArgType
		}
		compiled.source = functionArgumentLiteral
		compiled.literal = arg
	}
	return compiled, nil
}

func (arg functionArgument) evaluate(current, root any) FunctionValue {
	switch arg.source {
	case functionArgumentLiteral:
		return typedValueFromAny(arg.literal)
	case functionArgumentQuery:
		nodes := arg.query.Select(current, root)
		if arg.kind == Nodes {
			return TypedNodes(nodes)
		}
		if len(nodes) == 1 {
			return NewValue(nodes[0])
		}
		return NoValue()
	case functionArgumentLogical:
		return TypedLogical(arg.logical.Eval(current, root))
	case functionArgumentFunction:
		return arg.function.Call(current, root)
	default:
		return NoValue()
	}
}

func (arg functionArgument) writeTo(buf *strings.Builder) {
	switch arg.source {
	case functionArgumentLiteral:
		writeLiteral(buf, arg.literal)
	case functionArgumentQuery:
		arg.query.writeTo(buf)
	case functionArgumentLogical:
		arg.logical.writeTo(buf)
	case functionArgumentFunction:
		arg.function.writeTo(buf)
	}
}

// FuncExpr represents a function call in a filter expression per RFC 9535 §2.4.
type FuncExpr struct {
	name       string             // function name
	resultType FuncType           // compiled function result type
	fn         Function           // resolved function definition
	args       []functionArgument // compiled argument expressions
}

// NewFuncExpr creates a [FuncExpr] for the given function and arguments.
func NewFuncExpr(fn Function, args ...any) (*FuncExpr, error) {
	if len(args) != fn.ParameterCount() {
		return nil, fmt.Errorf("expected %d, got %d: %w", fn.ParameterCount(), len(args), ErrArgCount)
	}

	compiled := make([]functionArgument, len(args))
	for i, arg := range args {
		kind := fn.ParameterType(i)
		compiledArg, err := newFunctionArgument(kind, arg)
		if err != nil {
			return nil, fmt.Errorf("argument %d cannot convert to %s: %w", i+1, kind, err)
		}
		compiled[i] = compiledArg
	}
	return &FuncExpr{name: fn.Name(), resultType: fn.ResultType(), fn: fn, args: compiled}, nil
}

// Name returns the function name.
func (fe *FuncExpr) Name() string { return fe.name }

// ResultType returns the result type captured when the expression was compiled.
func (fe *FuncExpr) ResultType() FuncType { return fe.resultType }

// Call evaluates the function with the given current and root nodes.
// It evaluates argument expressions and passes the results to the underlying function.
func (fe *FuncExpr) Call(current, root any) FunctionValue {
	evalArgs := make([]FunctionValue, len(fe.args))
	for i := range fe.args {
		evalArgs[i] = fe.args[i].evaluate(current, root)
	}
	return fe.fn.Call(evalArgs)
}

// Eval implements BasicExpr for logical functions.
// Returns false if the function is not a logical function.
func (fe *FuncExpr) Eval(current, root any) bool {
	result := runtimeValueFromFunctionValue(fe.Call(current, root))
	switch fe.resultType {
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
		arg.writeTo(buf)
	}
	buf.WriteByte(')')
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
