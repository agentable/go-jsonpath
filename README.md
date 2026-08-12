# JSONPath

[![Go Version](https://img.shields.io/badge/go-1.26.5%2B-blue.svg)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/agentable/go-jsonpath.svg)](https://pkg.go.dev/github.com/agentable/go-jsonpath)
[![License](https://img.shields.io/badge/license-Agentable%20Commercial-purple.svg)](LICENSE)
[![RFC 9535](https://img.shields.io/badge/RFC-9535-green)](https://www.rfc-editor.org/rfc/rfc9535)

A Go implementation of RFC 9535 JSONPath with compliance-suite coverage, located results, and typed filter function extensions

## Features

- **RFC 9535 JSONPath**: Parses and evaluates standard JSONPath expressions against Go JSON values.
- **Compiled paths**: Reuse `Path` values safely across documents and goroutines.
- **Go-native selection**: Query decoded `any`, `[]any`, `map[string]any`, strings, numbers, booleans, and nil.
- **JSON helpers**: Query `[]byte` and `io.Reader` inputs with `QueryJSON*` convenience functions.
- **Located results**: Return immutable `NormalizedPath` values alongside matched nodes.
- **Typed function extensions**: Register custom filter functions with explicit `Value`, `Logical`, and `Nodes` runtime values.
- **Checked regular expressions**: Built-in `match` and `search` accept RFC 9485 I-Regexp patterns and fail closed on invalid syntax.
- **Structured diagnostics**: Inspect parse failures with `errors.Is` and `errors.As`.

## Installation

```bash
go get github.com/agentable/go-jsonpath
```

Requires **Go 1.26.5+**.

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	"github.com/agentable/go-jsonpath"
)

func main() {
	path, err := jsonpath.Parse("$.store.book[*].author")
	if err != nil {
		log.Fatal(err)
	}

	data := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"author": "Nigel Rees"},
				map[string]any{"author": "Evelyn Waugh"},
			},
		},
	}

	for author := range path.Select(data).All() {
		fmt.Println(author)
	}
}
```

## API Overview

| Task | API |
|---|---|
| Compile expressions | `Parse`, `MustParse`, `Valid`, `NewParser` |
| Inspect errors | `ParseError`, `ErrPathParse`, `ErrFunction`, `ErrUnmarshal`, `ErrInvalidPath`, `ErrIndexOutOfBounds` |
| Query decoded values | `Path.Select`, `Path.SelectLocated` |
| Query JSON bytes/readers | `QueryJSON`, `QueryJSONRead`, `QueryJSONLocated`, `QueryJSONReadLocated` |
| Iterate values | `NodeList.All`, `LocatedNodeList.All`, `LocatedNodeList.Values`, `LocatedNodeList.Paths` |
| Work with paths | `NormalizedPath.String`, `NormalizedPath.Pointer`, `NormalizedPath.ElementChecked`, `NormalizedPath.Elements`, `NormalizedPath.Append` |
| Extend filters | `WithFunctions`, `NewValueFunction`, `NewLogicalFunction`, `NewNodesFunction` |

See [pkg.go.dev/github.com/agentable/go-jsonpath](https://pkg.go.dev/github.com/agentable/go-jsonpath) for complete package documentation.

`Valid(expr)` checks whether `Parse(expr)` succeeds with the default built-in functions; use `NewParser` and `Parser.Parse` for expressions that rely on custom functions. `QueryJSON*` helpers preserve JSON numbers as `encoding/json.Number`; invalid `Path` values fail with `ErrInvalidPath` before reading or decoding. Callers that need another number representation, decoder policy, or streaming behavior should decode outside this package and then call `Path.Select` or `Path.SelectLocated`.

The zero `Parser` value uses the same built-in functions as `Parse` and a
no-option `NewParser`. Use `NewParser` only when configuring extension
functions. Non-nil parsers are immutable after construction and safe for
concurrent parsing.

Top-level paths start with `$`. The current-node identifier `@` is available only
inside filter expressions; using it as the root returns a positioned
`ErrPathParse` diagnostic.

Filter comparison preserves the exact magnitude of Go integer types and the
exact decimal value of `encoding/json.Number`. Finite `float32` and `float64`
values supplied to `Select` compare by their exact binary value. Invalid
`json.Number` text, NaN, and infinities are outside the decoded-JSON numeric
domain and are not comparable.

## Core Concepts

### Compiled Paths

Compile once, select many times:

```go
path := jsonpath.MustParse("$['store']['book'][*]['title']")
titles := path.Select(data)
```

`Path.String()` and `Path.MarshalText()` return a stable, canonical JSONPath string for the compiled query.
The zero `Path` value is invalid; `String()` returns an empty string and `MarshalText()` returns `ErrInvalidPath`.

### Located Results

Use located queries when you need both the value and its RFC 9535 normalized path.

```go
src := []byte(`{"store":{"book":[{"title":"Moby Dick"},{"title":"1984"}]}}`)
path := jsonpath.MustParse("$.store.book[*].title")

located, err := jsonpath.QueryJSONLocated(src, path)
if err != nil {
	log.Fatal(err)
}

for node := range located.All() {
	fmt.Printf("%s -> %s = %v\n", node.Path, node.Path.Pointer(), node.Value)
}
```

`NormalizedPath` is immutable. `NewNormalizedPath` and `Append` reject nil
elements, negative indexes, and name elements that are not valid UTF-8 with
`ErrInvalidPath`. Normalized names belong to the JSON string domain. Read
runtime indexes through `ElementChecked`, which returns `ErrIndexOutOfBounds`,
or copy all elements with `Elements`. `Element` is the compatibility wrapper
for statically known indexes and panics on an out-of-bounds index.
`LocatedNodeList` is a nodelist in query order, not a set. Use `Values` and `Paths` to iterate just values or paths. `Sort` and `Deduplicate` are caller opt-in operations; `Deduplicate` is path-based, keeps the first node for each path, and ignores value equality.

```go
path, err := jsonpath.NewNormalizedPath(
	jsonpath.NameElement("store"),
	jsonpath.NameElement("book"),
	jsonpath.IndexElement(0),
)
if err != nil {
	log.Fatal(err)
}

fmt.Println(path.String())  // $['store']['book'][0]
fmt.Println(path.Pointer()) // /store/book/0
```

### Parse Diagnostics

Parse failures satisfy `ErrPathParse`. Function lookup, validation, and registration failures also satisfy `ErrFunction`. JSON decode failures from `QueryJSON*` wrap `ErrUnmarshal`, and invalid compiled paths return `ErrInvalidPath`.

```go
_, err := jsonpath.Parse(" $.store")

var parseErr *jsonpath.ParseError
if errors.As(err, &parseErr) {
	fmt.Println(errors.Is(err, jsonpath.ErrPathParse))
	fmt.Println(parseErr.Offset)
	fmt.Println(parseErr.Reason)
}
```

### Function Extensions

Register custom filter functions with `NewParser(jsonpath.WithFunctions(...))`. Invalid function options fail at parser construction with `ErrFunction`. Runtime arguments and return values are typed:

- `jsonpath.Value`: one JSON value or `NoValue`
- `jsonpath.Logical`: boolean test result
- `jsonpath.Nodes`: node list result

Functions have one immutable name, fixed parameter list, result category, and
callback. The parser validates argument conversions from that signature;
callbacks receive only typed runtime values. Return `NoValue`, `Logical(false)`,
or an empty `Nodes` value for runtime absence.
`FuncLogical` parameters accept the same comparisons, test expressions,
negation, conjunction, disjunction, and parentheses used by filters.

```go
hasPrefix := jsonpath.NewLogicalFunction(
	"has_prefix",
	[]jsonpath.FuncType{jsonpath.FuncValue, jsonpath.FuncValue},
	func(args []jsonpath.FunctionValue) jsonpath.Logical {
		value, ok := args[0].(jsonpath.Value)
		if !ok || value.IsNothing() {
			return jsonpath.Logical(false)
		}
		prefix, ok := args[1].(jsonpath.Value)
		if !ok || prefix.IsNothing() {
			return jsonpath.Logical(false)
		}

		s, ok := value.Any().(string)
		p, ok2 := prefix.Any().(string)
		return jsonpath.Logical(ok && ok2 && strings.HasPrefix(s, p))
	},
)

parser, err := jsonpath.NewParser(jsonpath.WithFunctions(hasPrefix))
if err != nil {
	log.Fatal(err)
}
path, err := parser.Parse(`$[?has_prefix(@.sku, "book-")]`)
if err != nil {
	log.Fatal(err)
}
```

Use `NewValueFunction`, `NewLogicalFunction`, or `NewNodesFunction` according to
the result category. `WithFunctions` rejects zero-value definitions, nil callbacks,
invalid names or parameter types, duplicates, and built-in collisions when
`NewParser` is called. A nil `Option` also returns `ErrFunction` instead of
panicking. Function names use RFC function-name grammar: a lowercase ASCII
letter followed by lowercase ASCII letters, digits, or underscores.

### Built-in Regular Expressions

`match(value, pattern)` matches the entire string; `search(value, pattern)`
looks for a matching substring. The pattern may be a literal or a singular-query
value and must conform to RFC 9485 I-Regexp syntax. Invalid UTF-8, Go-specific
flags, lookarounds, backreferences, unsupported escapes, and malformed classes
evaluate to `Logical(false)`.

Unescaped `^` and `$` follow the RFC 9485 RE2 mapping and the embedded compliance
suite, where they act as anchors.

## Performance

The package includes benchmarks for selector execution, located queries, filter evaluation, normalized-path construction, JSON decoding, and PGO workflows.

```bash
task bench
go test -run '^$' -bench 'Benchmark(Select_|SelectLocated_|QueryJSON|ExtendPath|ParserParse)' -benchmem -count=10 .
task pgo:generate
task bench-pgo
```

Compare results only when the machine, Go toolchain, PGO mode, benchmark regex,
and repetition count are identical.

Regenerate `default.pgo` only when representative query hot paths change materially.

## Development

```bash
task fmt        # format code
task lint       # tidy-lint + golangci-lint
task test       # go test -race ./...
task vet        # go vet ./...
task verify     # deps + fmt + vet + lint + test + vuln
```

For development conventions and repository workflow, see [AGENTS.md](AGENTS.md).

## Contributing

Run `task fmt`, `task lint`, and `task test` before submitting changes.

## License

This software is licensed under the **Agentable Commercial License**,
exclusively for use with Agentable platform services and their direct
integrations. See the [LICENSE](LICENSE) file for full terms.
