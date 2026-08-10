# Architecture Contract

## Overview

This library is a small Go-native implementation of RFC 9535 JSONPath. Its
durable shape is an interpreter over decoded JSON values: parse a query once,
walk a JSON tree predictably, and return either values or values paired with
normalized paths.

This spec is the durable owner for architecture decisions that were proven by
the completed implementation pass.

## Scenarios

- A caller compiles a JSONPath expression and reuses the resulting `Path` across
  many decoded JSON documents.
- A caller asks for located results and expects the returned normalized paths to
  identify exactly the same values returned by plain selection.
- An extension author registers typed filter functions without learning lexer,
  parser, AST, or selector internals.
- A maintainer changes selector, filter, path, or function semantics and needs a
  small set of invariants that prevent semantic drift.

## Part List

| Part | Owner | Contract |
|---|---|---|
| Public API | `jsonpath.go`, `parse.go`, `json.go`, `normalized.go`, `result.go`, `errors.go`, `options.go` | Exposes compiled paths, JSON helpers, located results, normalized paths, sentinels, and typed function extensions. |
| Parser pipeline | `internal/lexer`, `internal/parser` | Converts top-level `$` paths and private `$`/`@` filter queries into one AST model while preserving structured parse diagnostics. |
| AST and selection semantics | `internal/ast` | Owns the single `PathQuery` representation, selector rules, filter evaluation, child traversal, slice/index resolution, and function runtime conversion. |
| Built-in functions | `internal/functions` | Owns RFC 9535 built-ins, RFC 9485 acceptance, and RE2 mapping used by parser registries. |
| Compliance harness | `compliance/` | Runs the embedded CTS and protects RFC compatibility. |

## Exposed Boundary

The `jsonpath` package is the consumer boundary. The common path is:

1. `Parse` or `MustParse`.
2. `Path.Select` or `Path.SelectLocated`.
3. Optional `NodeList` / `LocatedNodeList` iteration and `NormalizedPath`
   formatting.

Parser options and typed function values are exposed only for extension
authors. Internal AST, parser, runtime value, path step, and traversal details
must remain hidden from ordinary consumers.

## Hidden Boundary

These details are intentionally private:

- Lexer tokens, recursive descent parser state, and AST storage.
- Selector execution loops and descendant traversal stacks.
- Private `runtimeValue` representation for Nothing, JSON values, logical
  results, and node lists.
- Concrete `pathStep` storage inside `NormalizedPath`.
- Built-in regex cache and I-Regexp implementation details.
- Benchmark and PGO mechanics.

Public APIs must not expose these names, setup phases, or storage records.

## Ten-Year Invariants

- **RFC first**: RFC 9535 semantics and embedded CTS compatibility outrank local
  neatness and micro-optimization.
- **Predictable by default**: Object child traversal has one canonical order.
  Predictability is not a mode or option.
- **One selector truth**: Value selection rules have one internal owner. Located
  selection may keep a specialized loop only to construct paths.
- **One query model**: `PathQuery` represents top-level and filter queries;
  singularity is a predicate on that model, not a second query representation.
- **Pure selection**: `Select` and `SelectLocated` do not return runtime errors.
  Parse, validation, registration, and unmarshal errors stay at their trust
  boundaries.
- **Small public surface**: Common users compile and select. Extension authors
  get typed function values. No ordinary caller learns AST, planner, cache, or
  runtime algebra concepts.
- **Immutable located paths**: `NormalizedPath` is a value at the public
  boundary. Callers can inspect and append without mutating existing paths.
- **Measured performance**: Performance claims require focused `-benchmem`
  before/after evidence on the same benchmark. `default.pgo` changes only after
  representative hot paths materially change.

## Architecture Decisions

### Interpreter First

- **Decision**: Keep an interpreter-first execution model over decoded Go JSON
  values.
- **Why**: The library's main scenario is reusable compiled queries with clear
  RFC behavior, not query planning as a product.
- **Rejected**: JIT/codegen, planner APIs, query caches, and tree projection
  platforms. They add lifecycle, invalidation, and user-visible concepts without
  a current consumer.
- **Basis**: Current source and benchmarks show direct selector loops are
  readable and measurable. `.references/theory-jsontree` shows adjacent tree
  projection complexity that this library does not need.

### Specialized Loops Are Allowed

- **Decision**: Keep separate value and located public loops when that preserves
  hot-path clarity, but share selector semantics through internal owners and
  parity tests.
- **Why**: Located selection must construct normalized paths; forcing both paths
  through a generic traversal sink would hide the important work.
- **Rejected**: A single abstract traversal sink. It optimizes for symmetry over
  reviewability.
- **Basis**: Selector changes are protected by `SelectLocated.Values()` parity
  tests, while located code remains explicit about path construction.

### References Are Evidence, Not Authority

- **Decision**: Reference projects inform but do not own local architecture.
- **Why**: This project has a deliberately smaller Go public API than the richer
  reference ecosystems.
- **Rejected**: Porting `json-joy`, copying class hierarchies, or adopting a
  registry/platform shape solely because a reference has one.
- **Basis**: The embedded CTS, current code pressure, and local benchmarks are
  the primary evidence.

### Grammar Has One Owner

- **Decision**: `internal/parser.Parser.Parse` owns the top-level `$` grammar;
  private filter-query parsing alone accepts `$` and `@`.
- **Why**: Grammar acceptance, offsets, snippets, and causes must come from one
  positioned parser error path.
- **Rejected**: Public prechecks, parser modes, and duplicate snippet builders.
- **Basis**: Public and internal error tests prove the same parser-originated
  `ErrPathParse` diagnostics while filter queries retain `$`/`@` behavior.

## Forbidden

- Do not expose AST builders, selector visitors, planners, query caches, or
  plugin runtimes without a real consumer and an owning spec.
- Do not add a deterministic-order option; object traversal order is part of
  the library policy.
- Do not create policy-only gates that restate specs, README, AGENTS, or
  CLAUDE.md. Use behavior tests, compliance tests, benchmarks, or lint only when
  they prove real runtime behavior.
- Do not add spec mirror tests when stronger public or package behavior tests
  already cover the invariant.
- Do not preserve a misleading old shape solely for backward compatibility when
  the current contract has deliberately removed it.

## Acceptance Criteria

- The spec set includes this architecture contract, a consumer API contract, and
  a selection/runtime semantics contract.
- `README.md`, `CLAUDE.md` / `AGENTS.md`, and public tests agree on the current
  public surface and error sentinels.
- Selector changes include public parity coverage between `Select` and
  `SelectLocated`.
- Parser, selector, filter, function, or located-path semantic changes run
  compliance tests plus focused behavior tests.
- Performance-sensitive claims cite focused `-benchmem` evidence; otherwise no
  performance claim is made.
