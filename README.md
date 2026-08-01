# Gots

Gots is an **experimental, embeddable JavaScript subset runtime** written using
only the Go standard library. It is not a full ECMAScript implementation.

The current MVP supports primitive values, arrays and objects, lexical bindings,
blocks, conditionals, loops, functions/closures/recursion, property access,
basic operators, cooperative context/step/call-depth limits, typed and reflected
Go functions, and GC-aware `WeakMap` keys built with Go's `weak` package.
WeakMap also accepts non-registered Symbols as weak keys and implements the
pinned-spec methods `getOrInsert` and `getOrInsertComputed`; registered
`Symbol.for` values are rejected as weak keys.
Lexical declarations support `let`/`const` declaration lists, reject duplicate
names within a lexical declaration, and require `const` initializers. Classic
`for (init; test; update)` loops create per-iteration lexical bindings, and
comma expressions evaluate from left to right. Full
Test262 `let` coverage additionally depends on out-of-MVP modules, generators,
destructuring, `for`/`for-in`/`for-of`, `switch`, `try`, and `eval` semantics.
Initial standard-library coverage includes `Array.from`, `Array.of`, array
`at`, `includes`, `join`, and `push`, common string
search/padding/trimming/replacement methods, `Object.hasOwn`, and `globalThis`.
Script and function strict mode, direct/indirect eval, sloppy `with`, and
mapped/unmapped arguments objects are supported. Strict Test262 variants whose
syntax or runtime dependencies are outside the declared subset remain
explicitly unsupported. Synchronous classes include inheritance, `super`,
`new.target`, methods/accessors, public and private fields, private brands, and
static initialization blocks. Generator and async class elements remain
unsupported with their owning execution features. Annex B and BigInt remain unsupported, and the full
report is authoritative for incomplete edge-case and dependency coverage.

```go
r := gots.New(gots.Config{MaxSteps: 10_000})
p, _ := r.Compile(`function twice(x) { return x * 2; } twice(21)`)
result, err := r.Evaluate(context.Background(), p)
fmt.Println(result.Value.Inspect(), result.Stats.Steps)
```

Use `go run ./cmd/gots -e '1 + 2'`, pass a file, or pipe source on stdin.
Host functions that block cannot be preempted; cancellation is checked between
interpreter operations. Exposing privileged host functions is not a sandbox.

## Conformance

The normative language snapshot is ECMA-262 commit
`994b48ed0c0940edaa0e4ce4d9e358fa3ba91edb` (2026-08-01). The deliberately
small Test262 selection is pinned in `test262/selection.json` at commit
`b363f29d3c43c626dc852744ad64a0b48a003693`. Reproduce the bundled smoke
report with:

```sh
go run ./cmd/test262 -test262 test262
```

Unflagged Test262 Scripts expand into stable `#sloppy` and `#strict` case IDs;
`onlyStrict`, `noStrict`, `module`, and `raw` metadata select their normative
variant. Reports contain both the source-file and expanded-case denominators
and separate pass, fail, skip, unsupported, and timeout counts. Optional timing
diagnostics use `-timings`; canonical JSON, Markdown, and JUnit output contain
no durations. No aggregate result should be interpreted as full
ECMAScript compatibility. Go 1.25 or later is required.

Every report identifies the ECMAScript 2027 draft snapshot dated 2026-08-01
and the pinned Test262 commit, and shows
three distinct Test262 percentages: execution coverage (not unsupported), the
overall pass rate over the full denominator, and the pass rate among executed
tests. These are Test262 runner metrics, not ECMAScript specification coverage.
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
uses schema v2 stable case IDs. CI rejects any previous pass that becomes a
failure, timeout, or unsupported result, denominator decreases, pin changes,
and unsupported-classification changes that are not accompanied by an explicit
capability-manifest version change. Aggregate feature counts are diagnostic
because Test262 feature tags overlap.

To update the canonical coverage report and baseline locally, run:

```sh
./scripts/update-test262-coverage.sh
```

The script checks out the pinned Test262 commit into a temporary directory,
runs the complete suite, and removes the checkout and detailed report artifacts
when it finishes. `COVERAGE.md` and `test262/full-baseline.json` are preserved
for review even when the runner detects a regression and exits non-zero.
Additional Test262 runner flags can be appended to the command; the GitHub
workflow uses this to preserve its detailed JSON and JUnit artifacts outside
the temporary directory. In CI, `--baseline-ref main` loads the comparison
baseline from the remote `main` branch before running, while still writing the
new snapshot into the checked-out branch for the consistency diff. The workflow
fails when the regenerated `COVERAGE.md` or `full-baseline.json` differs from the
PR's committed files, so manually edited or stale reports are not accepted.

## Roadmap

Grow conformance feature-by-feature and keep unsupported syntax explicit rather
than silently approximating it. TC39 proposals remain out of scope until their
algorithms are incorporated into the pinned ECMA-262 revision.
