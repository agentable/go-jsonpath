package jsonpath

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/agentable/go-jsonpath/internal/ast"
	"github.com/agentable/go-jsonpath/internal/functions"
	"github.com/agentable/go-jsonpath/internal/parser"
)

var builtinRegistry = sync.OnceValue(func() map[string]ast.Function {
	builtins := functions.Builtins()
	registry := make(map[string]ast.Function, len(builtins))
	for _, fn := range builtins {
		registry[fn.Name()] = fn
	}
	return registry
})

// FuncType describes a function extension's parameter or result type as defined
// by RFC 9535 §2.4.1.
type FuncType uint8

const (
	// FuncLogical indicates a logical (bool) parameter or result.
	FuncLogical FuncType = iota
	// FuncValue indicates a single-JSON-value parameter or result.
	FuncValue
	// FuncNodes indicates a node-list parameter or result.
	FuncNodes
)

// FunctionValue is one of the RFC 9535 function runtime value types.
type FunctionValue = ast.FunctionValue

// Value is a JSON value passed to or returned from a JSONPath function.
type Value = ast.TypedValue

// Logical is a logical value passed to or returned from a JSONPath function.
type Logical = ast.TypedLogical

// Nodes is a node-list value passed to or returned from a JSONPath function.
type Nodes = ast.TypedNodes

// NewValue returns a present JSON value for function execution.
func NewValue(value any) Value {
	return ast.NewValue(value)
}

// NoValue returns an absent value for function execution.
func NoValue() Value {
	return ast.NoValue()
}

// Function is an immutable extension function definition created by
// [NewValueFunction], [NewLogicalFunction], or [NewNodesFunction]. Callbacks
// must be safe for concurrent use when the configured [Parser] is used
// concurrently.
type Function struct {
	name       string
	params     []FuncType
	resultType FuncType
	call       func([]FunctionValue) FunctionValue
}

// NewValueFunction defines a function that returns a JSON value.
func NewValueFunction(name string, params []FuncType, call func([]FunctionValue) Value) Function {
	var invoke func([]FunctionValue) FunctionValue
	if call != nil {
		invoke = func(args []FunctionValue) FunctionValue { return call(args) }
	}
	return newFunction(name, params, FuncValue, invoke)
}

// NewLogicalFunction defines a function that returns a logical value.
func NewLogicalFunction(name string, params []FuncType, call func([]FunctionValue) Logical) Function {
	var invoke func([]FunctionValue) FunctionValue
	if call != nil {
		invoke = func(args []FunctionValue) FunctionValue { return call(args) }
	}
	return newFunction(name, params, FuncLogical, invoke)
}

// NewNodesFunction defines a function that returns a node list.
func NewNodesFunction(name string, params []FuncType, call func([]FunctionValue) Nodes) Function {
	var invoke func([]FunctionValue) FunctionValue
	if call != nil {
		invoke = func(args []FunctionValue) FunctionValue { return call(args) }
	}
	return newFunction(name, params, FuncNodes, invoke)
}

func newFunction(name string, params []FuncType, resultType FuncType, call func([]FunctionValue) FunctionValue) Function {
	return Function{
		name:       name,
		params:     slices.Clone(params),
		resultType: resultType,
		call:       call,
	}
}

// Option configures a [Parser].
type Option func(*parserOptions)

type parserOptions struct {
	functions map[string]ast.Function
	err       error
}

// WithFunctions registers additional filter functions beyond the RFC 9535
// built-ins. Function names must follow RFC function-name grammar: a lowercase
// ASCII letter followed by lowercase ASCII letters, digits, or underscores.
// Names must not duplicate another registered function or override a built-in.
func WithFunctions(fns ...Function) Option {
	fns = slices.Clone(fns)
	return func(o *parserOptions) {
		o.registerFunctions(fns...)
	}
}

// Parser parses JSONPath expressions into [Path] values, optionally
// configured with extension functions. The zero value uses the RFC 9535
// built-in functions and is safe for concurrent use.
type Parser struct {
	registry map[string]ast.Function
}

type functionAdapter struct {
	name       string
	params     []ast.FuncType
	resultType ast.FuncType
	call       func([]FunctionValue) FunctionValue
}

func (a functionAdapter) Name() string { return a.name }

func (a functionAdapter) ResultType() ast.FuncType {
	return a.resultType
}

func mapFuncType(ft FuncType) (ast.FuncType, bool) {
	switch ft {
	case FuncLogical:
		return ast.Logical, true
	case FuncValue:
		return ast.Value, true
	case FuncNodes:
		return ast.Nodes, true
	default:
		return ast.Value, false
	}
}

func (a functionAdapter) ParameterCount() int { return len(a.params) }

func (a functionAdapter) ParameterType(index int) ast.FuncType {
	return a.params[index]
}

func (a functionAdapter) Call(args []ast.FunctionValue) ast.FunctionValue {
	return a.call(args)
}

func (o *parserOptions) registerFunctions(fns ...Function) {
	for _, fn := range fns {
		name := fn.name
		if !validFunctionName(name) {
			o.addFunctionError("invalid function name %q", name)
			continue
		}
		if _, exists := o.functions[name]; exists {
			o.addFunctionError("duplicate function %q", name)
			continue
		}
		if _, builtin := builtinRegistry()[name]; builtin {
			o.addFunctionError("function %q overrides a built-in", name)
			continue
		}

		resultType, ok := mapFuncType(fn.resultType)
		if !ok {
			o.addFunctionError("function %q has invalid result type %d", name, fn.resultType)
			continue
		}
		if fn.call == nil {
			o.addFunctionError("function %q has nil callback", name)
			continue
		}

		params := make([]ast.FuncType, len(fn.params))
		valid := true
		for i, param := range fn.params {
			params[i], ok = mapFuncType(param)
			if !ok {
				o.addFunctionError("function %q has invalid parameter type %d at index %d", name, param, i)
				valid = false
				break
			}
		}
		if !valid {
			continue
		}

		o.functions[name] = functionAdapter{
			name:       name,
			params:     params,
			resultType: resultType,
			call:       fn.call,
		}
	}
}

func (o *parserOptions) addFunctionError(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	o.err = errors.Join(o.err, err)
}

func validFunctionName(name string) bool {
	switch name {
	case "", "true", "false", "null":
		return false
	}

	if name[0] < 'a' || name[0] > 'z' {
		return false
	}

	for i := 1; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

// NewParser creates a new [Parser] configured by opts. Invalid options return
// an error satisfying [ErrFunction].
func NewParser(opts ...Option) (*Parser, error) {
	if len(opts) == 0 {
		return &Parser{}, nil
	}

	cfg := parserOptions{
		functions: make(map[string]ast.Function),
	}
	for _, option := range opts {
		if option == nil {
			return nil, fmt.Errorf("%w: nil parser option", ErrFunction)
		}
		option(&cfg)
	}
	if cfg.err != nil {
		return nil, errors.Join(ErrFunction, cfg.err)
	}

	registry := maps.Clone(builtinRegistry())
	maps.Copy(registry, cfg.functions)

	return &Parser{registry: registry}, nil
}

// Parse compiles a JSONPath expression. Returns [ErrPathParse] on failure.
func (p *Parser) Parse(expr string) (Path, error) {
	registry := p.registry
	if registry == nil {
		registry = builtinRegistry()
	}
	return parse(expr, registry)
}

func parse(expr string, registry map[string]ast.Function) (Path, error) {
	internalParser, err := parser.New(expr, registry)
	if err != nil {
		return Path{}, wrapPathParseError(err)
	}

	query, err := internalParser.Parse()
	if err != nil {
		return Path{}, wrapPathParseError(err)
	}

	return Path{query: query}, nil
}

func wrapPathParseError(err error) error {
	if err == nil {
		return nil
	}
	parseErr := newParseError(err)
	if errors.Is(err, parser.ErrFunction) {
		return errors.Join(ErrPathParse, ErrFunction, parseErr)
	}
	return errors.Join(ErrPathParse, parseErr)
}

func newParseError(err error) *ParseError {
	var parserErr *parser.Error
	if errors.As(err, &parserErr) {
		return &ParseError{
			Offset:  parserErr.Offset,
			Reason:  parserErr.Reason,
			Snippet: parserErr.Snippet,
			Cause:   parserErr.Cause,
		}
	}
	return &ParseError{
		Offset: -1,
		Reason: err.Error(),
		Cause:  err,
	}
}

// MustParse is like [Parser.Parse] but panics if expr is invalid.
func (p *Parser) MustParse(expr string) Path {
	path, err := p.Parse(expr)
	if err != nil {
		panic(err)
	}
	return path
}
