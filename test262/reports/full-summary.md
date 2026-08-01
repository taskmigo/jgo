# Full Test262 coverage report

Generated on 2026-08-01 with Go 1.25.1 against pinned Test262 commit
`b363f29d3c43c626dc852744ad64a0b48a003693`:

```sh
go run ./cmd/test262 -all -test262 /tmp/test262 -timeout 100ms \
  -json test262-full.json -junit test262-full.xml \
  -summary test262/reports/full-summary.md
```

The command exits non-zero because unexpected failures exist. `unsupported` means
Gots could not compile a required Test262 harness/test feature. A `pass` includes
valid negative tests, so this is runner coverage—not a claim of full ECMAScript
feature support. The machine-readable JSON/JUnit files are intentionally emitted
as CI/local artifacts rather than committed.


| pass | fail | skip | unsupported | timeout | total |
|---:|---:|---:|---:|---:|---:|
| 4438 | 317 | 0 | 49114 | 0 | 53869 |

### By feature

| feature | pass | fail | unsupported | timeout | total |
|---|---:|---:|---:|---:|---:|
| (unclassified) | 1362 | 299 | 17282 | 0 | 18943 |
| AggregateError | 0 | 0 | 31 | 0 | 31 |
| Array.fromAsync | 0 | 0 | 95 | 0 | 95 |
| Array.prototype.at | 0 | 0 | 12 | 0 | 12 |
| Array.prototype.flat | 0 | 0 | 15 | 0 | 15 |
| Array.prototype.flatMap | 0 | 0 | 21 | 0 | 21 |
| Array.prototype.includes | 0 | 0 | 69 | 0 | 69 |
| Array.prototype.values | 0 | 0 | 4 | 0 | 4 |
| ArrayBuffer | 0 | 0 | 278 | 0 | 278 |
| Atomics | 0 | 0 | 386 | 0 | 386 |
| Atomics.pause | 0 | 0 | 5 | 0 | 5 |
| Atomics.waitAsync | 0 | 0 | 101 | 0 | 101 |
| BigInt | 39 | 0 | 1462 | 0 | 1501 |
| DataView | 0 | 0 | 193 | 0 | 193 |
| DataView.prototype.getFloat32 | 0 | 0 | 7 | 0 | 7 |
| DataView.prototype.getFloat64 | 0 | 0 | 5 | 0 | 5 |
| DataView.prototype.getInt16 | 0 | 0 | 7 | 0 | 7 |
| DataView.prototype.getInt32 | 0 | 0 | 7 | 0 | 7 |
| DataView.prototype.getInt8 | 0 | 0 | 5 | 0 | 5 |
| DataView.prototype.getUint16 | 0 | 0 | 7 | 0 | 7 |
| DataView.prototype.getUint32 | 0 | 0 | 7 | 0 | 7 |
| DataView.prototype.setUint8 | 0 | 0 | 56 | 0 | 56 |
| Error.isError | 0 | 0 | 13 | 0 | 13 |
| FinalizationRegistry | 0 | 0 | 49 | 0 | 49 |
| Float16Array | 0 | 0 | 51 | 0 | 51 |
| Float32Array | 0 | 0 | 7 | 0 | 7 |
| Float64Array | 0 | 0 | 7 | 0 | 7 |
| Int16Array | 0 | 0 | 2 | 0 | 2 |
| Int32Array | 0 | 0 | 4 | 0 | 4 |
| Int8Array | 0 | 0 | 38 | 0 | 38 |
| Intl-enumeration | 0 | 0 | 35 | 0 | 35 |
| Intl.DateTimeFormat-datetimestyle | 0 | 0 | 16 | 0 | 16 |
| Intl.DateTimeFormat-dayPeriod | 0 | 0 | 12 | 0 | 12 |
| Intl.DateTimeFormat-extend-timezonename | 0 | 0 | 2 | 0 | 2 |
| Intl.DateTimeFormat-formatRange | 0 | 0 | 37 | 0 | 37 |
| Intl.DateTimeFormat-fractionalSecondDigits | 0 | 0 | 10 | 0 | 10 |
| Intl.DisplayNames | 0 | 0 | 47 | 0 | 47 |
| Intl.DisplayNames-v2 | 0 | 0 | 12 | 0 | 12 |
| Intl.DurationFormat | 0 | 0 | 111 | 0 | 111 |
| Intl.Era-monthcode | 0 | 0 | 1566 | 0 | 1566 |
| Intl.ListFormat | 0 | 0 | 81 | 0 | 81 |
| Intl.Locale | 0 | 0 | 172 | 0 | 172 |
| Intl.Locale-info | 0 | 0 | 60 | 0 | 60 |
| Intl.NumberFormat-unified | 0 | 0 | 68 | 0 | 68 |
| Intl.NumberFormat-v3 | 0 | 0 | 99 | 0 | 99 |
| Intl.RelativeTimeFormat | 0 | 0 | 79 | 0 | 79 |
| Intl.Segmenter | 0 | 0 | 79 | 0 | 79 |
| IsHTMLDDA | 0 | 0 | 42 | 0 | 42 |
| Iterator.prototype.join | 0 | 0 | 18 | 0 | 18 |
| Map | 0 | 0 | 40 | 0 | 40 |
| Math.sumPrecise | 0 | 0 | 10 | 0 | 10 |
| Object.fromEntries | 0 | 0 | 25 | 0 | 25 |
| Object.hasOwn | 0 | 0 | 62 | 0 | 62 |
| Object.is | 0 | 0 | 2 | 0 | 2 |
| Promise | 0 | 0 | 4 | 0 | 4 |
| Promise.allSettled | 0 | 0 | 102 | 0 | 102 |
| Promise.any | 0 | 0 | 92 | 0 | 92 |
| Promise.prototype.finally | 0 | 0 | 29 | 0 | 29 |
| Proxy | 0 | 0 | 479 | 0 | 479 |
| Reflect | 0 | 0 | 478 | 0 | 478 |
| Reflect.construct | 0 | 0 | 704 | 0 | 704 |
| Reflect.set | 0 | 0 | 46 | 0 | 46 |
| Reflect.setPrototypeOf | 0 | 0 | 23 | 0 | 23 |
| RegExp.escape | 0 | 0 | 21 | 0 | 21 |
| Set | 0 | 0 | 38 | 0 | 38 |
| ShadowRealm | 0 | 0 | 64 | 0 | 64 |
| SharedArrayBuffer | 0 | 0 | 467 | 0 | 467 |
| String.fromCodePoint | 0 | 0 | 22 | 0 | 22 |
| String.prototype.at | 0 | 0 | 11 | 0 | 11 |
| String.prototype.endsWith | 0 | 0 | 27 | 0 | 27 |
| String.prototype.includes | 0 | 0 | 26 | 0 | 26 |
| String.prototype.isWellFormed | 0 | 0 | 8 | 0 | 8 |
| String.prototype.matchAll | 0 | 0 | 16 | 0 | 16 |
| String.prototype.replaceAll | 0 | 0 | 41 | 0 | 41 |
| String.prototype.toWellFormed | 0 | 0 | 8 | 0 | 8 |
| String.prototype.trimEnd | 0 | 0 | 24 | 0 | 24 |
| String.prototype.trimStart | 0 | 0 | 23 | 0 | 23 |
| Symbol | 0 | 0 | 1494 | 0 | 1494 |
| Symbol.asyncIterator | 0 | 0 | 538 | 0 | 538 |
| Symbol.hasInstance | 0 | 0 | 17 | 0 | 17 |
| Symbol.isConcatSpreadable | 0 | 0 | 34 | 0 | 34 |
| Symbol.iterator | 0 | 0 | 1867 | 0 | 1867 |
| Symbol.match | 0 | 0 | 88 | 0 | 88 |
| Symbol.matchAll | 0 | 0 | 63 | 0 | 63 |
| Symbol.prototype.description | 0 | 0 | 8 | 0 | 8 |
| Symbol.replace | 0 | 0 | 98 | 0 | 98 |
| Symbol.search | 0 | 0 | 37 | 0 | 37 |
| Symbol.species | 0 | 0 | 284 | 0 | 284 |
| Symbol.split | 0 | 0 | 58 | 0 | 58 |
| Symbol.toPrimitive | 0 | 0 | 233 | 0 | 233 |
| Symbol.toStringTag | 0 | 0 | 131 | 0 | 131 |
| Symbol.unscopables | 0 | 0 | 45 | 0 | 45 |
| Temporal | 0 | 0 | 6714 | 0 | 6714 |
| TypedArray | 0 | 0 | 2524 | 0 | 2524 |
| TypedArray.prototype.at | 0 | 0 | 13 | 0 | 13 |
| Uint16Array | 0 | 0 | 6 | 0 | 6 |
| Uint32Array | 0 | 0 | 2 | 0 | 2 |
| Uint8Array | 0 | 0 | 13 | 0 | 13 |
| Uint8ClampedArray | 0 | 0 | 6 | 0 | 6 |
| WeakMap | 0 | 0 | 79 | 0 | 79 |
| WeakRef | 0 | 0 | 37 | 0 | 37 |
| WeakSet | 0 | 0 | 34 | 0 | 34 |
| __getter__ | 0 | 0 | 27 | 0 | 27 |
| __proto__ | 0 | 0 | 20 | 0 | 20 |
| __setter__ | 0 | 0 | 27 | 0 | 27 |
| align-detached-buffer-semantics-with-web-reality | 0 | 0 | 158 | 0 | 158 |
| arbitrary-module-namespace-names | 0 | 0 | 16 | 0 | 16 |
| array-find-from-last | 0 | 0 | 109 | 0 | 109 |
| array-grouping | 0 | 0 | 28 | 0 | 28 |
| arraybuffer-transfer | 0 | 0 | 59 | 0 | 59 |
| arrow-function | 64 | 0 | 885 | 0 | 949 |
| async-functions | 203 | 0 | 502 | 0 | 705 |
| async-iteration | 593 | 0 | 4378 | 0 | 4971 |
| await-dictionary | 0 | 0 | 89 | 0 | 89 |
| caller | 0 | 0 | 23 | 0 | 23 |
| canonical-tz | 0 | 0 | 19 | 0 | 19 |
| change-array-by-copy | 0 | 0 | 132 | 0 | 132 |
| class | 744 | 0 | 4048 | 0 | 4792 |
| class-fields-private | 370 | 0 | 764 | 0 | 1134 |
| class-fields-private-in | 8 | 0 | 11 | 0 | 19 |
| class-fields-public | 289 | 0 | 1769 | 0 | 2058 |
| class-methods-private | 344 | 0 | 1365 | 0 | 1709 |
| class-static-block | 28 | 0 | 37 | 0 | 65 |
| class-static-fields-private | 16 | 0 | 329 | 0 | 345 |
| class-static-fields-public | 28 | 0 | 185 | 0 | 213 |
| class-static-methods-private | 162 | 0 | 1351 | 0 | 1513 |
| coalesce-expression | 4 | 0 | 22 | 0 | 26 |
| computed-property-names | 12 | 0 | 466 | 0 | 478 |
| const | 0 | 0 | 15 | 0 | 15 |
| cross-realm | 0 | 0 | 203 | 0 | 203 |
| decorators | 0 | 0 | 27 | 0 | 27 |
| default-parameters | 219 | 0 | 2050 | 0 | 2269 |
| destructuring-assignment | 90 | 0 | 51 | 0 | 141 |
| destructuring-binding | 516 | 0 | 6121 | 0 | 6637 |
| dynamic-import | 376 | 0 | 635 | 0 | 1011 |
| error-cause | 0 | 0 | 5 | 0 | 5 |
| error-stack-accessor | 0 | 0 | 35 | 0 | 35 |
| explicit-resource-management | 60 | 0 | 423 | 0 | 483 |
| exponentiation | 14 | 0 | 90 | 0 | 104 |
| export-star-as-namespace-from-module | 0 | 0 | 19 | 0 | 19 |
| for-in-order | 0 | 0 | 9 | 0 | 9 |
| for-of | 0 | 0 | 5 | 0 | 5 |
| generators | 418 | 0 | 3701 | 0 | 4119 |
| globalThis | 0 | 0 | 148 | 0 | 148 |
| hashbang | 6 | 18 | 5 | 0 | 29 |
| host-gc-required | 0 | 0 | 15 | 0 | 15 |
| immutable-arraybuffer | 0 | 0 | 66 | 0 | 66 |
| import-attributes | 1 | 0 | 100 | 0 | 101 |
| import-bytes | 0 | 0 | 5 | 0 | 5 |
| import-defer | 90 | 0 | 162 | 0 | 252 |
| import-text | 0 | 0 | 6 | 0 | 6 |
| import.meta | 1 | 0 | 22 | 0 | 23 |
| intl-normative-optional | 0 | 0 | 8 | 0 | 8 |
| iterator-chunking | 0 | 0 | 78 | 0 | 78 |
| iterator-helpers | 0 | 0 | 567 | 0 | 567 |
| iterator-includes | 0 | 0 | 44 | 0 | 44 |
| iterator-sequencing | 0 | 0 | 32 | 0 | 32 |
| joint-iteration | 0 | 0 | 82 | 0 | 82 |
| json-modules | 0 | 0 | 14 | 0 | 14 |
| json-parse-with-source | 0 | 0 | 22 | 0 | 22 |
| json-superset | 0 | 0 | 4 | 0 | 4 |
| legacy-regexp | 0 | 0 | 26 | 0 | 26 |
| let | 1 | 0 | 76 | 0 | 77 |
| logical-assignment-operators | 12 | 0 | 96 | 0 | 108 |
| new.target | 12 | 0 | 51 | 0 | 63 |
| nonextensible-applies-to-private | 0 | 0 | 4 | 0 | 4 |
| numeric-separator-literal | 64 | 0 | 95 | 0 | 159 |
| object-rest | 3 | 0 | 352 | 0 | 355 |
| object-spread | 24 | 0 | 111 | 0 | 135 |
| optional-catch-binding | 1 | 0 | 4 | 0 | 5 |
| optional-chaining | 26 | 0 | 30 | 0 | 56 |
| promise-try | 0 | 0 | 12 | 0 | 12 |
| promise-with-resolvers | 0 | 0 | 10 | 0 | 10 |
| proxy-missing-checks | 0 | 0 | 3 | 0 | 3 |
| regexp-dotall | 0 | 0 | 17 | 0 | 17 |
| regexp-duplicate-named-groups | 0 | 0 | 19 | 0 | 19 |
| regexp-lookbehind | 0 | 0 | 19 | 0 | 19 |
| regexp-match-indices | 0 | 0 | 31 | 0 | 31 |
| regexp-modifiers | 83 | 0 | 147 | 0 | 230 |
| regexp-named-groups | 54 | 0 | 46 | 0 | 100 |
| regexp-unicode-property-escapes | 163 | 0 | 518 | 0 | 681 |
| regexp-v-flag | 50 | 0 | 137 | 0 | 187 |
| resizable-arraybuffer | 0 | 0 | 465 | 0 | 465 |
| rest-parameters | 96 | 0 | 0 | 0 | 96 |
| set-methods | 0 | 0 | 193 | 0 | 193 |
| source-phase-imports | 147 | 0 | 106 | 0 | 253 |
| source-phase-imports-module-source | 63 | 0 | 46 | 0 | 109 |
| stable-array-sort | 0 | 0 | 4 | 0 | 4 |
| stable-typedarray-sort | 0 | 0 | 1 | 0 | 1 |
| string-trimming | 0 | 0 | 54 | 0 | 54 |
| super | 4 | 0 | 15 | 0 | 19 |
| symbols-as-weakmap-keys | 0 | 0 | 29 | 0 | 29 |
| tail-call-optimization | 0 | 0 | 35 | 0 | 35 |
| template | 0 | 0 | 1 | 0 | 1 |
| top-level-await | 0 | 0 | 277 | 0 | 277 |
| u180e | 1 | 0 | 24 | 0 | 25 |
| uint8array-base64 | 0 | 0 | 71 | 0 | 71 |
| upsert | 0 | 0 | 72 | 0 | 72 |
| well-formed-json-stringify | 0 | 0 | 1 | 0 | 1 |
