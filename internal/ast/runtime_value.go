package ast

type runtimeValueKind uint8

const (
	runtimeNothing runtimeValueKind = iota
	runtimeJSON
	runtimeLogical
	runtimeNodes
)

type runtimeValue struct {
	kind    runtimeValueKind
	value   any
	logical bool
	nodes   []any
}

func nothingRuntimeValue() runtimeValue {
	return runtimeValue{kind: runtimeNothing}
}

func jsonRuntimeValue(value any) runtimeValue {
	if _, ok := value.(jsonNull); ok {
		value = nil
	}
	return runtimeValue{kind: runtimeJSON, value: value}
}

func logicalRuntimeValue(value bool) runtimeValue {
	return runtimeValue{kind: runtimeLogical, logical: value}
}

func nodesRuntimeValue(nodes []any) runtimeValue {
	return runtimeValue{kind: runtimeNodes, nodes: nodes}
}

func runtimeValueFromAny(value any) runtimeValue {
	if value, ok := value.(runtimeValue); ok {
		return value
	}
	return jsonRuntimeValue(value)
}

func runtimeValueFromFunctionValue(value FunctionValue) runtimeValue {
	switch value := value.(type) {
	case TypedValue:
		if value.IsNothing() {
			return nothingRuntimeValue()
		}
		return jsonRuntimeValue(value.Any())
	case TypedLogical:
		return logicalRuntimeValue(value.Bool())
	case TypedNodes:
		return nodesRuntimeValue([]any(value))
	default:
		return nothingRuntimeValue()
	}
}

func typedValueFromRuntimeValue(value runtimeValue) TypedValue {
	if value.kind != runtimeJSON {
		return NoValue()
	}
	return NewValue(value.value)
}
