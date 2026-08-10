# Selection And Runtime Semantics

## Overview

Selection semantics are the heart of the library. A compiled query walks a
decoded JSON value and returns matching nodes in a predictable order. Filter
comparison and function execution use private runtime values so RFC concepts
remain clear without becoming ordinary public API.

This spec owns the completed implementation decisions for deterministic
traversal, selector ownership, runtime value semantics, total function
execution, and measured performance.

## Scenarios

- A wildcard query over an object returns stable results across process runs.
- A descendant query over objects and arrays returns the same value order for
  plain and located selection.
- A filter compares missing values, JSON null, strings, numbers, booleans,
  arrays, and objects without collapsing distinct RFC concepts.
- A built-in or extension function receives singular-query and filter-query
  arguments with the correct runtime type.
- A maintainer wants to optimize a selector hot path without changing semantics
  or claiming unmeasured performance.

## Concept Model

### Child Traversal

- **Definition**: The ordered enumeration of direct children of an array or
  object.
- **Array order**: Natural index order.
- **Object order**: Ascending key byte order.
- **Reverse order**: The exact reverse needed by stack-based descendant
  traversal so the popped result order remains canonical.
- **Owner**: `internal/ast/child.go`.

### Selector Semantics

- **Definition**: Name, index, slice, wildcard, and filter rules for producing
  selected values from one input node.
- **Plain composition owner**: `internal/ast.PathQuery.Select` and
  `internal/ast.Segment.Apply`; `Path.Select` is only the public validity and
  delegation boundary.
- **Owner**: `internal/ast.Selector.Apply` for value selection.
- **Located specialization**: `jsonpath.go` may keep specialized located loops
  only to construct `NormalizedPath` values.

### Runtime Value

- **Definition**: Private representation of RFC function/filter runtime
  categories: Nothing, JSON value, Logical, and Nodes.
- **Owner**: `internal/ast/runtime_value.go`.
- **Public boundary**: Only typed extension values (`Value`, `Logical`,
  `Nodes`, `NoValue`) are public.

### Query Representation

- **Definition**: `PathQuery` is the only query representation for top-level
  and filter queries.
- **Singularity**: `PathQuery.IsSingular` recognizes name/index child chains;
  wildcard, filter, slice, multi-selector, and descendant queries are not
  singular.
- **Owner**: `internal/ast/query.go`.

## Contracts

### Deterministic Child Traversal

- Arrays enumerate children from index `0` to `len(arr)-1`.
- Objects enumerate children by ascending key byte order.
- Primitives enumerate no children.
- Wildcard, filter, descendant, and located traversal must share the same child
  order.
- There is no option to use Go map iteration order.

**Decision**

- **Decision**: Object traversal is deterministic by library policy.
- **Why**: Predictable selector behavior makes tests, debugging, located paths,
  and cache keys trustworthy even where RFC 9535 permits multiple object result
  orders.
- **Rejected**: Leaving object order unspecified, or making deterministic order
  configurable. Both push avoidable uncertainty onto callers.
- **Basis**: The embedded CTS allows object order flexibility, but this
  library's tests and README promise predictable selector behavior.

### Selector Ownership

- Value-only selection uses the selector owner directly.
- Public located selection must preserve selector semantics and add only path
  construction.
- Internal filter query evaluation must reuse the same selector and child
  traversal rules.
- `SelectLocated(input).Values()` must stay equal to `Select(input)`.

**Decision**

- **Decision**: Share semantic facts, not necessarily one traversal function.
- **Why**: Located traversal has extra path work. A single sink abstraction would
  hide hot-path behavior and make review harder.
- **Rejected**: Interface-heavy traversal sinks and visitor platforms.
- **Basis**: Current code keeps direct loops while tests prove plain/located
  parity.

### Filter Comparison

- Nothing equals only Nothing.
- Nothing does not equal JSON null.
- JSON null equals document nil.
- Numeric JSON values compare with exact numeric coercion. Signed and unsigned
  integers preserve magnitude, `encoding/json.Number` preserves its decimal
  value, and finite floats preserve their binary value. Invalid number text,
  NaN, and infinities are not comparable JSON numbers.
- `QueryJSON*` decodes JSON text numbers as `encoding/json.Number`; caller-decoded
  input passed to `Select` or `SelectLocated` retains its supplied Go numeric
  representation.
- Strings compare lexicographically.
- Arrays and objects compare by deep JSON equality.
- Booleans and nulls can satisfy `<=` or `>=` only through equality; they have
  no less-than or greater-than order.
- Logical and Nodes runtime values are not comparable as JSON values.
- Exact numeric recognition and comparison are owned by
  `internal/ast/number.go`; filter expressions consume that private result.

**Decision**

- **Decision**: Comparison operates on private `runtimeValue`, not naked `any`.
- **Why**: `any` cannot express the difference between a missing singular query
  and JSON null without scattered sentinels.
- **Rejected**: Public algebraic runtime types and ad hoc `nothing` / `jsonNull`
  checks in comparison helpers. The first leaks internals; the second makes
  later function fixes fragile.
- **Basis**: RFC 9535 ValueType / LogicalType / NodesType semantics, CTS cases
  for null and special Nothing, behavior tests for missing-vs-null, and public
  precision-boundary tests across plain and located selection.

### Function Argument Conversion

- Literal arguments convert to JSON `Value`.
- Singular query arguments convert to `Value` when exactly one node is selected,
  otherwise `NoValue`.
- Filter query and non-singular query arguments convert to `Nodes`.
- Logical-expression arguments evaluate to `Logical` using the existing filter
  expression grammar and precedence.
- Nested function calls pass through their typed runtime result.
- Every extension definition has a fixed semantic parameter list. A nested
  function argument is accepted only when its declared result type matches the
  destination parameter.
- Value functions used as comparison operands convert through the private
  runtime boundary before comparison.

### Total Function Execution

- Built-in calls and extension callbacks always return a `FunctionValue`.
- Runtime absence is data semantics: `NoValue`, `Logical(false)`, or empty
  `Nodes`.
- Parse-time validation and unknown functions are expression parse errors.
  Zero-value definitions, nil callbacks, invalid names or parameter types, and
  accidental built-in collisions fail parser construction. Typed constructors
  make invalid result categories unrepresentable. Function error semantics must
  wrap `ErrFunction`.
- Function argument validation is a deterministic comparison against the fixed
  signature. Singular-query conversion is selected from the destination
  parameter without invoking extension code.
- Function runtime code must not use panic as a control path.

### I-Regexp Acceptance And Mapping

- `match` and `search` accept only valid UTF-8 patterns conforming to the RFC
  9485 acceptance grammar. Invalid patterns evaluate to `Logical(false)`.
- One private checker validates the raw pattern before match anchoring, dot
  replacement, or RE2 compilation. Literal and singular-query patterns use the
  same compiler path.
- The checker accepts only RFC single-character escapes and Unicode general
  categories/subcategories; it rejects Go-specific flags, lookarounds,
  backreferences, multi-character escapes, Unicode blocks, malformed classes,
  invalid quantifiers, and unbalanced groups.
- `match` compiles in full-match mode; `search` compiles in substring mode.
  Cache identity includes both the raw pattern and mode, so a compiled match
  entry cannot bypass validation for a raw search pattern.
- RFC 9485's ABNF/XSD reading and its Section 5.4 RE2 mapping disagree about
  unescaped `^` and `$`. This library follows Section 5.4 and the embedded CTS,
  which give them anchor semantics.
- Go's RE2 implementation owns execution and its documented resource limits;
  the checker adds no separate arbitrary pattern, nesting, or repetition limit.

### Performance Discipline

- Pre-size slices when cardinality is knowable for wildcard, slice, filter, and
  located path construction.
- Avoid interface dispatch, reflection, hidden allocation, or callback-heavy
  abstraction on selector hot paths unless benchmark evidence justifies it.
- Run focused `-benchmem` before and after performance-sensitive changes.
- Do not claim a performance win without before/after evidence from the same
  benchmark.
- Regenerate `default.pgo` only when representative hot paths materially change.

## Forbidden

- Do not add a query planner, JIT, string code generator, or cache platform to
  solve ordinary selector execution.
- Do not expose private `runtimeValue` publicly.
- Do not make function runtime failures return errors from `Select`.
- Do not split selector rules into parallel unsynchronized implementations.
- Do not update only `Select` or only `SelectLocated` when selector semantics
  change.

## Acceptance Criteria

- Object wildcard, filter, descendant, and located traversal tests assert the
  canonical order.
- Plain and located parity tests cover name, index, slice, wildcard, filter,
  descendant, and multi-selector queries.
- Filter comparison tests cover Nothing, JSON null, bool equality-only ordering,
  numeric coercion, strings, arrays, and objects.
- Function tests cover singular query, missing singular query, filter query,
  non-singular query, nested function, and extension runtime absence.
- Compliance tests pass after parser, query, filter, or function semantic
  changes.
- Focused benchmarks are recorded for selector, located path, normalized path,
  parser, built-in function, or runtime value hot-path changes.
