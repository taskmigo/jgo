# Gots

Gots is an **experimental, embeddable JavaScript subset runtime** written using
only the Go standard library. It is not a full ECMAScript implementation.

The current MVP supports primitive values, arrays and objects, lexical bindings,
blocks, conditionals, loops, functions/closures/recursion, property access,
basic operators, cooperative context/step/call-depth limits, typed and reflected
Go functions, and GC-aware `WeakMap` keys built with Go's `weak` package.
WeakMap also accepts non-registered Symbols as weak keys and implements the
proposal methods `getOrInsert` and `getOrInsertComputed`; registered
`Symbol.for` values are rejected as weak keys.
Lexical declarations support `let`/`const` declaration lists, reject duplicate
names within a lexical declaration, and require `const` initializers. Classic
`for (init; test; update)` loops create per-iteration lexical bindings, and
comma expressions evaluate from left to right. Full
Test262 `let` coverage additionally depends on out-of-MVP modules, generators,
destructuring, `for`/`for-in`/`for-of`, `switch`, `try`, and `eval` semantics.
Initial standard-library coverage includes `Array.from`, `Array.of`, array/string
`at` and `includes`, common string search/padding/trimming/replacement methods,
`Object.hasOwn`, and `globalThis`. The full report remains authoritative for
their incomplete edge-case and dependency coverage.

```go
r := gots.New(gots.WithMaxSteps(10_000))
p, _ := r.Compile(`function twice(x) { return x * 2; } twice(21)`)
value, err := r.Run(p)
```

Use `go run ./cmd/gots -e '1 + 2'`, pass a file, or pipe source on stdin.
Host functions that block cannot be preempted; cancellation is checked between
interpreter operations. Exposing privileged host functions is not a sandbox.

## Conformance

The deliberately small Test262 selection is pinned in `test262/selection.json`
at commit `b363f29d3c43c626dc852744ad64a0b48a003693`. Reproduce the bundled smoke
report with:

```sh
go run ./cmd/test262 -test262 test262
```

Reports contain the selection denominator and separate pass, fail, skip, and
unsupported counts. No aggregate result should be interpreted as full
ECMAScript compatibility. Go 1.25 or later is required.

Every report identifies the ECMA target and pinned Test262 commit, and shows
three distinct percentages: tests covered (not unsupported), passes over the
full denominator, and passes among covered tests. This avoids presenting the
covered-test pass rate as overall ECMAScript conformance.
Tests whose Test262 metadata has no `features` entry are grouped under stable
`path:*` categories derived from their suite path (for example,
`path:built-ins/WeakMap` or `path:language/expressions`), so no results disappear
into an ambiguous `(unclassified)` bucket.

To measure the entire pinned Test262 checkout, use `-all`; the command returns a
non-zero status when it finds failures:

```sh
go run ./cmd/test262 -all -test262 /path/to/test262 \
  -baseline test262/full-baseline.json
```

The official checked-in full-run coverage report is [`COVERAGE.md`](/COVERAGE.md);
detailed JSON and JUnit reports are generated
as local or CI artifacts because they contain tens of thousands of test cases.
The Test262 workflow always executes this complete pinned suite. Its baseline
allows known failures while failing CI if pass coverage decreases, failure or
unsupported counts increase, a feature regresses, or the test denominator/pin
changes unexpectedly.

## Roadmap

Grow conformance feature-by-feature, add complete Test262 harness include and
strict-mode handling, and keep unsupported syntax explicit rather than silently
approximating it.
