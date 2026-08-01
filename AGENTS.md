# AGENTS.md

This file applies to the entire repository.

## Project intent

Gots is an experimental, embeddable ECMAScript subset implemented with the Go
standard library. Correct JavaScript semantics and maintainable feature modules
take priority over preserving historical behavior.

- The semantic target is ECMA-262 commit
  `994b48ed0c0940edaa0e4ce4d9e358fa3ba91edb`, snapshot date 2026-08-01.
- Use that pinned revision's normative algorithms as the source of truth. Do
  not infer JavaScript behavior from Go behavior or another engine.
- Never implement a TC39 proposal that is not incorporated into the pinned
  ECMA-262 commit, regardless of its stage or adoption by other runtimes.
- The project is pre-release. Public APIs may be redesigned when that produces
  a clearer model. Do not add deprecated aliases, compatibility wrappers, or
  migration layers unless explicitly requested.
- Unsupported syntax must fail explicitly. Do not silently approximate an
  out-of-scope feature or classify broad ECMAScript support from a few passing
  tests.
- Go 1.25 and the standard library are the only runtime requirements.

## Mandatory ECMA-262 compliance

Every language behavior implemented by this repository must follow the
normative ECMAScript algorithms at
<https://github.com/tc39/ecma262/tree/994b48ed0c0940edaa0e4ce4d9e358fa3ba91edb>.
This requirement is not optional and takes precedence over convenience,
compatibility with an older implementation, or a simpler Go mapping.

For every semantic change:

1. Read the relevant current specification sections before designing the code.
   Include all abstract operations referenced by the main algorithm; do not
   implement an isolated summary from memory.
2. Translate the algorithm into cohesive Go helpers while preserving observable
   ordering, abrupt completion, coercion, property access, and calls into user
   code. Specification records do not need identical names, but their semantic
   distinctions must remain visible in the design.
3. Add focused tests for the normal algorithm and its observable edge cases,
   then run the relevant pinned Test262 feature cohort.
4. Treat a mismatch between existing behavior and ECMA-262 as a bug. Correct
   the implementation and update tests; do not preserve the mismatch behind a
   legacy path.
5. If a required dependency is not implemented, return an explicit unsupported
   syntax/feature result. Never substitute an approximation that can produce an
   incorrect observable result.

When the living specification changes, update the dated target in
`test262/selection.json`, README, generated coverage, tests, and this file in a
single intentional change. Keep the pinned Test262 commit fixed during a
semantic comparison; update the pin separately so denominator changes cannot
hide regressions.

The repository remains a subset: “compliant” means every claimed and executable
feature follows ECMA-262. It does not mean unsupported chapters are implemented,
and documentation must never imply full ECMAScript conformance.

## Repository layout

Keep code organized as same-package feature modules:

- `syntax_*`: tokens, spans, lexer, AST, parser, and static semantics.
- `runtime_*`: lifecycle, completion records, references, environments,
  evaluation, calls, and construction.
- `value.go`, `value_*`: language values, UTF-16 strings, abstract operations,
  property descriptors, prototypes, and object behavior.
- `builtins.go`, `builtin_*`: intrinsic registry and domain-specific built-ins.
  Each domain owns a private installer; installation order is explicit.
- `host_bridge.go`: Go-to-JavaScript conversion and native/reflected calls.
- `cmd/gots`: thin process entry point plus testable CLI orchestration.
- `cmd/test262`: suite execution, harness, metadata, baseline policy, models,
  and deterministic report rendering.
- `test262/selection.json`: pinned suite commit and semantic target.
- `COVERAGE.md` and `test262/full-baseline.json`: generated canonical full-suite
  results; do not edit their counts manually.

Place tests beside their owning feature and use the same filename prefix where
practical. Extract a new module only when it gives a responsibility a clear
home; avoid generic plugin interfaces and one-function files without a strong
reason.

## Semantic invariants

Preserve these invariants whenever evaluator or value code changes:

- Evaluation order is observable and proceeds left to right where required.
  Member-call bases and computed property keys are evaluated exactly once.
- Statement control flow uses completion records. JavaScript exceptions remain
  distinct from cancellation, step limits, call-depth limits, and host errors.
- Identifier and property operations go through references, environment
  records, and common property operations instead of ad hoc evaluator logic.
- Lexical bindings model declaration instantiation, TDZ, mutability, closures,
  and per-iteration environments.
- ECMAScript strings are UTF-16 code-unit sequences. Never replace string
  indexing or length logic with byte counts or `[]rune`; preserve lone
  surrogates internally.
- Property keys are Strings or Symbols. Never stringify a Symbol key.
- Keep strict equality, SameValue, SameValueZero, and loose equality separate.
  Route coercion through the shared abstract operations.
- Ordinary property lookup follows descriptors and prototype chains. Arrays
  additionally maintain indexed-property and `length` invariants.
- Built-in methods live on the appropriate intrinsic/prototype, validate their
  receiver, and use shared argument/coercion helpers.
- `Symbol.for` identity is stable within a Runtime. Registered symbols are not
  valid weak keys; objects and non-registered symbols are.
- WeakMap storage must not keep keys alive. Keep strong identities alive only
  for the duration required to safely complete an operation.
- Runtime checkpoints and statistics must remain consistent across JavaScript,
  native, iterator, call, and construct paths.

When a spec algorithm depends on an out-of-scope feature, implement the common
abstract operation and leave the unsupported dependency explicit rather than
adding a feature-specific shortcut.

## Coding conventions

- Prefer descriptive names and straightforward control flow over terse code.
- Keep exported APIs small and cohesive. Internal spec records and algorithms
  should remain unexported unless hosts genuinely need them.
- Use `Inspect` for diagnostic/output rendering; do not substitute it for the
  ECMAScript `ToString` operation.
- Use `Runtime` coercion/property helpers when evaluation may invoke user code.
  Primitive-only helpers must not bypass `valueOf`, `toString`, or method calls
  in interpreter paths.
- Represent JavaScript failures with `Exception`; use ordinary Go errors for
  host and operational failures.
- Keep comments for compatibility constraints, specification intent, GC
  lifetime, or other non-obvious behavior. Do not narrate obvious code.
- Do not add dependencies, code generation, global initialization side effects,
  or dynamic built-in registration.
- Format every modified Go file with `gofmt`.

## Validation workflow

Run the narrowest relevant tests while iterating, then run the repository gates:

```sh
gofmt -w <modified-go-files>
go test -count=1 ./...
go vet ./...
go test -race ./...
go test -run '^$' -bench . ./...
git diff --check
```

The race build requires CGO and a C compiler. If the local environment cannot
provide them, report that limitation; do not claim the race suite passed.

For parser, evaluator, or built-in semantic changes, add focused tests covering
both the normal path and observable edge cases. Important examples include:

- source spans, invalid grammar, and early errors;
- evaluation order, abrupt completion, TDZ, closures, and `this` binding;
- NaN, infinities, signed zero, coercion failures, and equality variants;
- astral characters, surrogate pairs, and lone surrogates;
- own/inherited properties, descriptors, Symbol keys, and array length;
- iterator protocol validation and iterator-result handling;
- WeakMap receiver/key validation, callback re-entry, and garbage collection;
- host conversion, panic recovery, cancellation, limits, and statistics;
- CLI input precedence, exact output, and exit codes.

## Test262 policy

Use the pinned commit from `test262/selection.json`; do not test against an
unrecorded moving checkout.

Quick smoke run:

```sh
go run ./cmd/test262 -test262 test262
```

Canonical full-suite refresh:

```sh
./scripts/update-test262-coverage.sh
```

The refresh downloads the pinned suite into a temporary directory and updates
`COVERAGE.md` and `test262/full-baseline.json`. Review both generated diffs.
Detailed JSON and JUnit reports belong in `/tmp` or CI artifacts, not in the
repository. Extra output flags can be passed after `--`, for example:

```sh
./scripts/update-test262-coverage.sh -- \
  -json /tmp/gots-test262.json \
  -junit /tmp/gots-test262.xml
```

A conformance change is acceptable only when the claimed feature tests pass and
the full-suite policy accepts the new baseline. At minimum, total pass count
must not decrease, failures/timeouts must not increase, unsupported tests may
only decrease, and the pinned commit/denominator must remain intentional.

## Change discipline

- Inspect `git status` before editing. Preserve unrelated staged and unstaged
  changes; never reset or rewrite user work.
- Keep implementation, tests, target metadata, README examples, CLI call sites,
  and generated Test262 baseline changes in sync.
- Do not manually stage, commit, push, or publish unless explicitly requested.
- Before handing off, state which checks passed, which could not run, the final
  Test262 counts when relevant, and whether generated artifacts remain.
