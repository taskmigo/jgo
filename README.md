# Gots

Gots is an **experimental, embeddable JavaScript subset runtime** written using
only the Go standard library. It is not a full ECMAScript implementation.

The current MVP supports primitive values, arrays and objects, lexical bindings,
blocks, conditionals, loops, functions/closures/recursion, property access,
basic operators, cooperative context/step/call-depth limits, typed and reflected
Go functions, and GC-aware `WeakMap` keys built with Go's `weak` package.

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

## Roadmap

Grow conformance feature-by-feature, add complete Test262 harness include and
strict-mode handling, and keep unsupported syntax explicit rather than silently
approximating it.
