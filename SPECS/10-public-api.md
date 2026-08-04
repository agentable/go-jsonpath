# Public API Contract

## Overview

The public API is optimized for one job: compile a JSONPath expression and use
it to select values from decoded JSON or JSON bytes/readers. Extension points
exist, but they stay behind parser options and typed function values.

This spec owns consumer-facing contracts. Internal AST, parser, runtime value,
and path step representation are owned by implementation packages and by
`SPECS/20-selection-runtime.md`.

## Scenarios

- A user writes `path, err := jsonpath.Parse(expr)` and immediately calls
  `path.Select(data)`.
- A user needs normalized paths and calls `Path.SelectLocated` or
  `QueryJSONLocated`.
- A user stores or serializes a compiled path using text marshaling.
- An extension author adds a deterministic filter function with typed runtime
  arguments and results.

## Concept Model

### `Path`

- **Definition**: A compiled JSONPath query value.
- **Lifecycle**: Created by `Parse`, `MustParse`, `Parser.Parse`,
  `Parser.MustParse`, or text unmarshaling.
- **Validity**: The zero `Path` value is invalid. It is not root `$`.
- **Invariants**: Valid paths select without returning runtime errors.
- **Owner**: `jsonpath.go`.

### `NormalizedPath`

- **Definition**: An immutable RFC 9535 normalized path identifying a located
  result.
- **Lifecycle**: Created as root by the zero value, by `NewNormalizedPath`, by
  located selection, or by `Append`.
- **Invariants**: `String`, `Pointer`, `Compare`, `Equal`, `Sort`, and
  `Deduplicate` semantics are stable. Public inspection returns value copies.
  Public construction and append reject nil elements, negative indexes, and
  names outside the valid UTF-8 JSON string domain with `ErrInvalidPath`;
  located selection creates already-valid private steps from decoded JSON.
- **Owner**: `types.go`.

### Function Runtime Values

- **Definition**: Public typed values used only by filter extension functions:
  `Value`, `Logical`, `Nodes`, and `NoValue`.
- **Lifecycle**: Produced by parser/runtime argument conversion or by callbacks
  created with `NewValueFunction`, `NewLogicalFunction`, or `NewNodesFunction`.
- **Invariants**: `NoValue` means absence, not JSON null. JSON null is
  represented as a present `Value` whose `Any()` is nil.
- **Owner**: `options.go` and `internal/ast/function.go`.

## Contracts

### Compiled Path Validity

- `Parse(expr)` returns `(Path, error)`.
- Top-level expressions start with `$`; `@` is valid only inside filter
  expressions. A top-level `@` failure retains positioned `ParseError` details.
- `Valid(expr)` reports whether `Parse(expr)` succeeds with the default parser
  and built-in functions. It is not custom-parser validity.
- On parse failure, the returned error satisfies `ErrPathParse`.
- `MustParse(expr)` panics with an error satisfying `ErrPathParse` on failure.
- A zero `Path` selects nothing and has an empty string representation.
- `Path.MarshalText` on a zero `Path` returns an error satisfying
  `ErrInvalidPath`.
- JSON helper functions receiving an invalid `Path` return `ErrInvalidPath`
  before consuming reader input.
- `QueryJSON*` helpers are convenience decoders over JSON bytes or readers plus
  `Path.Select` / `Path.SelectLocated`. They expose JSON numbers as
  `encoding/json.Number`. Callers needing another number representation, custom
  decoder policy, or streaming behavior decode outside this package and pass
  the decoded value to `Path` methods.

**Decision**

- **Decision**: Use a value-returning `Path` API and make the zero value
  invalid.
- **Why**: Ordinary users should not carry nil checks for a compiled query
  handle. Root `$` must be explicit, not hidden inside an uninitialized value.
- **Rejected**: `*Path` with nil receiver behavior, and zero `Path` as root.
  The first leaks nullable handles into the common path; the second hides a
  programmer mistake behind a valid query.
- **Basis**: README examples, public API tests, and Go value semantics all read
  more directly with `Path` as a small immutable handle.

### Located Results

- `Path.SelectLocated(input).Values()` must return the same values as
  `Path.Select(input)` for the same valid path and input.
- Located paths are normalized paths beginning at `$`.
- `NormalizedPath.Compare` orders element-by-element: index elements sort before
  name elements, indexes compare numerically, names compare lexically, and the
  shorter common-prefix path sorts first.
- `NormalizedPath.Equal` reports whether two paths contain the same elements.
- `LocatedNodeList` is a nodelist in query order, not a set.
- `LocatedNodeList.Sort` orders by normalized path comparison and is caller
  opt-in.
- `LocatedNodeList.Deduplicate` is caller opt-in. It deduplicates by
  `NormalizedPath`, ignores `Value`, keeps the first occurrence of each path in
  current order, preserves the relative order of kept nodes, clears trimmed
  backing slots, and returns the clipped list.

### Normalized Path Boundary

- `PathElement` is a public construction and observation boundary implemented
  by `NameElement` and `IndexElement`.
- `NameElement` values must be valid UTF-8 so canonical strings, JSON Pointers,
  comparison, and hashing identify the same member.
- Internal storage must not use public interface dispatch as its source of
  truth.
- `Elements()` returns a copy. `Append()` returns a new `NormalizedPath`.

**Decision**

- **Decision**: Keep `NormalizedPath` concrete internally and value-like
  externally.
- **Why**: Located paths are hot and visible. Callers need stable inspection;
  internal code needs compact compare/hash/format behavior.
- **Rejected**: Turning `NormalizedPath` into a JSON Pointer manipulation
  library. Pointer formatting is useful; pointer mutation/lookup is adjacent.
- **Basis**: Existing located-result APIs, path tests, and
  `BenchmarkNormalizedPath*` / `BenchmarkSelectLocated*` cover the real
  contract.

### Error Sentinels

- Parse failures wrap `ErrPathParse`.
- Function lookup, registration, and signature-validation failures wrap
  `ErrFunction`.
- JSON decode failures from `QueryJSON*` helpers wrap `ErrUnmarshal`.
- Invalid compiled-path use wraps `ErrInvalidPath`.
- Structured parse diagnostics remain available through `ParseError`.

### Extension Functions

- `WithFunctions` registers non-built-in functions.
- Invalid parser options fail at `NewParser` construction time and satisfy
  `ErrFunction`; this includes a nil `Option`, which returns a nil parser rather
  than panicking. Expression parse failures remain `ErrPathParse`.
- Function names must follow RFC function-name grammar: a lowercase ASCII letter
  followed by lowercase ASCII letters, digits, or underscores. They cannot be
  `true`, `false`, or `null`.
- `NewValueFunction`, `NewLogicalFunction`, and `NewNodesFunction` create
  immutable definitions with fixed semantic parameter types. `NewParser`
  rejects invalid definitions, and expression parsing rejects incompatible
  arguments with `ErrFunction`.
- Extension callbacks are total: runtime absence is expressed as `NoValue`,
  `Logical(false)`, or an empty `Nodes` value, not as a runtime error channel.

**Decision**

- **Decision**: Keep extension runtime values typed and total.
- **Why**: `Select` and `SelectLocated` have no error channel; adding runtime
  function errors would infect the simplest selection API.
- **Rejected**: callbacks returning `(FunctionValue, error)`. That shape confuses data
  absence with configuration or validation failure.
- **Basis**: RFC 9535 function types, existing parser validation, README
  extension examples, and tests around `ErrFunction` and `NoValue`.

## Hidden Complexity Boundary

The 90% public path must not mention:

- `internal/ast` selectors, segments, queries, or runtime values.
- Lexer/parser token types.
- `pathStep` storage.
- Descendant traversal frames or selector growth estimates.
- Built-in regex caches.

## Forbidden

- Do not add a second invalid-path sentinel; invalid compiled-path use is
  `ErrInvalidPath`.
- Do not add compatibility shims for pointer-returning parse APIs.
- Do not add options that duplicate internal selector, traversal, cache, or
  runtime value configuration.
- Do not expose public AST, planner, cache, or JSON Pointer mutation APIs
  without a real consumer and a new owning spec.

## Acceptance Criteria

- README examples compile against the value-returning `Path` API.
- Public tests cover zero/invalid path behavior, JSON helper invalid-path
  behavior, parse sentinels, function validation sentinels, located/plain
  parity, and normalized path immutability.
- Extension function tests show both parse-time `ErrFunction` failures and
  runtime absence expressed through typed values.
- `task test` and `task lint` pass after any public API contract change.
