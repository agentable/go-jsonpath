# JSONPath Library

RFC 9535 JSONPath implementation for Go. The library prioritizes predictable selector behavior, full embedded compliance-suite coverage, precise located results, and a small public API.

## Project Overview

- Module: `github.com/agentable/go-jsonpath`
- Go: see `go.mod`; current toolchain target is Go 1.26.5
- Primary user documentation: [README.md](README.md)
- Specification source: RFC 9535, the embedded CTS in `compliance/testdata/cts.json`, and the durable contracts in `SPECS/`
- `AGENTS.md` is a symlink to this file.

## Commands

```bash
task deps        # download modules and verify test dependency graph
task fmt         # go tool golangci-lint fmt ./...
task vet         # go vet ./...
task lint        # tidy-lint + golangci-lint
task test        # go test -race ./...
task bench       # go test -run '^$' -bench=. -benchmem ./...
task pgo:generate # regenerate default.pgo from representative benchmarks
task bench-pgo   # run representative benchmarks with default.pgo
task verify      # deps + fmt + vet + lint + test + vuln
```

## Architecture

```text
jsonpath/
├── jsonpath.go          # Public API, JSON decoding, and located selection
├── types.go             # Result containers, immutable NormalizedPath, sorting, dedup
├── options.go           # Parser configuration and public function extension boundary
├── compliance/          # Embedded CTS runner and test data
└── internal/
    ├── ast/             # Query, segment, selector, filter, function AST, shared eval rules
    ├── functions/       # RFC 9535 built-in function implementations
    ├── lexer/           # Zero-copy tokenization via byte offsets
    └── parser/          # Recursive descent parser and validation
```

### Current Public Surface

- `Parse`, `MustParse`, `Valid`, `ParseError`
- `NewParser`, `WithFunctions`
- `Function`, `FuncType`, `FunctionValue`, `Value`, `Logical`, `Nodes`, `NewValue`, `NoValue`
- `NewValueFunction`, `NewLogicalFunction`, `NewNodesFunction`
- `QueryJSON`, `QueryJSONRead`, `QueryJSONLocated`, `QueryJSONReadLocated`
- `Path.Select`, `Path.SelectLocated`
- `NormalizedPath`, `NewNormalizedPath`, `PathElement`, `NameElement`, `IndexElement`
- `NodeList`, `LocatedNode`, `LocatedNodeList`

## Agent Operating Rules

- Read current code and relevant docs before editing.
- Keep edits surgical and scoped to the requested behavior.
- Preserve RFC 9535 semantics before chasing local neatness or speed.
- Prefer direct control flow over abstractions on selector hot paths.
- Respect context budgets; summarize long evidence instead of pasting it.
- Resolve conflicts by preserving user work and adapting around it.
- Test behavior and public contracts, not wording copied from docs.
- Do not create policy-only gate scripts that merely restate docs.
- Do not add spec mirror tests when stronger behavior tests already cover the invariant.
- Fail loudly with sentinel-preserving errors; do not hide rich parse context.

## Agent Workflow

### Specs First

Before changing public API, selector behavior, parser semantics, function semantics, located paths, normalized paths, or performance-sensitive code, read the relevant files in `SPECS/`.

1. Use the SPECS Index to identify the owner contract.
2. Read the owner spec completely before designing or editing.
3. Keep implementation and tests aligned with the spec; if the spec is wrong, update the spec and implementation together.
4. Do not create policy-only gates or spec mirror tests that merely restate prose.

### References First

Before changing parsing, selection, compliance behavior, function semantics, located paths, or performance-sensitive code, read at least two relevant projects in `.references/` after reading the relevant specs.

1. Pick the relevant reference category from the References Index.
2. Read the implementation or test cases that match the change.
3. Reuse proven ideas only when they fit this library's simpler public API.
4. Do not import reference-project complexity unless a test or benchmark justifies it.

### Benchmark Before Performance Claims

For parser, selector, located-result, normalized-path, or built-in-function performance work:

1. Run focused benchmarks first.
2. Make the change.
3. Re-run the same benchmarks with `-benchmem`.
4. Claim a performance win only when post-change numbers improve.

### Documentation Layers

- README: user-facing installation, examples, API overview.
- CLAUDE.md / AGENTS.md: development rules, workflow, indexes.
- SPECS: durable architecture, public API, and runtime semantics contracts.
- Do not move tutorial content into CLAUDE.md.

## SPECS Index

| Path | Owns |
|---|---|
| `SPECS/00-architecture.md` | Architecture boundaries, exposed/hidden surfaces, 10-year invariants, forbidden architectural scope |
| `SPECS/10-public-api.md` | Consumer API, `Path` validity, sentinels, normalized path boundary, extension function public contract |
| `SPECS/20-selection-runtime.md` | Deterministic traversal, selector ownership, filter comparison, private runtime values, function conversion, performance discipline |

## References Index

| Path | Use For |
|---|---|
| `.references/jsonpath-compliance-test-suite` | RFC 9535 compliance cases, result ordering, expected behavior |
| `.references/speakeasy-jsonpath` | Modern Go JSONPath API and project layout comparisons |
| `.references/theory-jsonpath` | Parser, registry, normalized path, and function model patterns |
| `.references/oliveagle-jsonpath` | Historical Goessner-style tradeoffs and compatibility pitfalls |
| `.references/theory-jsontree` | Adjacent tree/path traversal ideas for located-path work |

## Design Philosophy

- **KISS** — Keep one parser model, one selector semantics source, one normalized path representation.
- **DRY** — Share selector boundary rules through small helpers, not parallel reimplementations.
- **YAGNI** — Do not add guard APIs, compatibility shims, caches, or alternate query APIs without a concrete consumer.
- **Progressive disclosure** — Keep `Parse` / `Select` direct; expose parser options and typed function values only for extension authors.
- **Errors as teachers** — Preserve sentinel categories while keeping parse offset, reason, snippet, and cause available.
- **Never:** accidental complexity, feature gravity, abstraction theater, configurability cope.

## Design Constraints

- Keep `Select` and `SelectLocated` behavior aligned whenever selector semantics change.
- Keep plain selection composition owned by `internal/ast`; `Path.Select` remains a thin validity and delegation boundary.
- Keep `internal/ast.Selector` as the hot-path tagged union; do not replace it with interface dispatch.
- Treat `jsonpath.go`, `types.go`, and `internal/functions/builtins.go` as performance-sensitive.
- Keep `NormalizedPath` immutable at the public boundary; preserve `String`, `Pointer`, `Compare`, `Equal`, `Sort`, and `Deduplicate` semantics.
- Reject invalid UTF-8 normalized names at public construction boundaries; do not normalize invalid bytes to U+FFFD.
- Keep top-level `$` grammar and positioned errors in `internal/parser`; accept `@` only through private filter-query parsing.
- Keep `PathQuery` as the only query representation; use `IsSingular` instead of adding a singular-query mirror.
- Preserve JSON number lexemes in `QueryJSON*` and exact numeric comparison across integer, `json.Number`, and finite float inputs; never normalize the numeric domain through `float64`.
- Keep function extension runtime values typed: `Value`, `Logical`, `Nodes`; use `NoValue` for absence, not JSON null.
- Validate raw I-Regexp syntax before RE2 mapping and cache insertion; include match/search mode in cache identity, and preserve CTS anchor semantics for unescaped `^`/`$`.
- Regenerate `default.pgo` only when representative query hot paths materially change.
- Track the Go version declared in `go.mod`; do not add compatibility paths for older toolchains.

## Coding Rules

### Performance Rules

- Pre-size slices when cardinality is knowable: wildcard, slice, filter scans, dedup maps.
- Check `len(...) == 0` before allocating result slices.
- Avoid interface churn, reflection, hidden allocations, and callback-heavy dispatch on selector execution paths.
- Keep capacity growth and selector dispatch visible in code review.
- If a refactor makes hot-path code more abstract, prove it benchmark-neutral or better.

### Go Rules

- Use `t.Parallel()` unless a test mutates shared package state.
- Use `for b.Loop()` in benchmarks.
- Use `%w` or `errors.Join` so sentinels remain discoverable with `errors.Is`.
- Prefer stdlib helpers already used here: `maps.Clone`, `maps.Copy`, `slices.Clone`, `slices.Grow`, `slices.SortFunc`, `slices.Clip`, `sync.OnceValue`, and `iter.Seq`.
- Preserve documented zero-value behavior on public types.
- Keep new code ASCII unless the file already needs Unicode.

## Testing

- Use `require` for must-pass preconditions and `assert` for value comparisons.
- Run `task test` and `task lint` before finishing code changes.
- Run compliance tests when parser, query, filter, or function semantics change.
- Add example tests when README snippets change materially.
- Add focused benchmarks for selector, located-path, normalized-path, parser, or built-in-function performance changes.
- Keep table-driven tests direct and readable.

## Error Handling

Public sentinels:

- `ErrPathParse`
- `ErrFunction`
- `ErrUnmarshal`
- `ErrInvalidPath`

Rules:

- Wrap errors so callers can use `errors.Is`.
- Preserve structured parse diagnostics through `ParseError`.
- Do not collapse parser or function validation context into vague strings.

## Dependencies

Runtime dependency:

- `github.com/go-json-experiment/json`: JSON decoding for `QueryJSON*` helpers.

Development and tooling dependencies:

- `github.com/google/go-cmp`: structural comparisons in tests.
- `github.com/stretchr/testify`: assertions in tests.
- `github.com/golangci/golangci-lint/v2/cmd/golangci-lint`: linter entrypoint managed via `go tool`.

## Dependency Issue Reporting

When you encounter a bug, limitation, or unexpected behavior in a dependency library:

1. Do not work around it by reimplementing the dependency's functionality.
2. Do not silently replace the dependency with local code.
3. Create a report file: `reports/<dependency-name>.md`.
4. Include dependency name and version, trigger scenario, expected vs actual behavior, relevant errors, and possible workaround without implementing it.
5. Continue with unrelated work if possible.

## Forbidden

- No breaking RFC 9535 behavior to make a benchmark look better.
- No changing only `Select` or only `SelectLocated` when selector semantics should stay aligned.
- No benchmark claims without `-benchmem` evidence.
- No replacing straightforward selector switches with interface-heavy dispatch.
- No mutability leaks in `NormalizedPath`.
- No untyped `any` contract for public function extension runtime values.
- No documentation masquerading as code: do not encode prose into constants or data structures unless runtime code consumes them.
- No policy-only gates that restate README, CLAUDE.md, or SPECS.
- No working around dependency bugs by reimplementing dependency functionality. Use `reports/` instead.
- No destructive git commands such as `git reset --hard` or reverting unrelated worktree changes.

## Agent Skills

Use repository skills under `.agents/skills/` when the task clearly matches them.

| Skill | When to Use |
|---|---|
| `go-best-practices` | Idiomatic Go API, error, naming, and testing decisions |
| `modernizing` | Go 1.20-1.26 language and standard-library updates |
| `golangci-linting` | Lint configuration, running golangci-lint, and lint-driven cleanup |
| `library-test-covering` | Strengthening tests for public behavior |
| `taskfile-configuring` | Taskfile maintenance |
| `readme-writing` | User-facing README updates |
| `agent-md-writing` | Regenerating CLAUDE.md / AGENTS.md guidance |
| `library-docs-maintaining` | Refreshing CLAUDE.md, AGENTS.md, and README.md together |
| `library-code-simplifying` | Simplifying recent code without behavior changes |
| `library-legacy-pruning` | Removing deprecated or legacy public surfaces |
| `code-refactoring` | Broader structural cleanup when local simplification is not enough |
