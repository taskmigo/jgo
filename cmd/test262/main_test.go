package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRenderCoverageIsDeterministic(t *testing.T) {
	r := report{
		Commit:      "abc",
		ECMAVersion: "ECMAScript 2025",
		Mode:        "full",
		Counts:      counts{Pass: 2, Fail: 1, Skip: 1, Unsupported: 1, Timeout: 1, Total: 6},
		ByFeature: map[string]counts{
			"z": {Unsupported: 1, Total: 1},
			"a": {Pass: 1, Fail: 1, Skip: 1, Timeout: 1, Total: 4},
		},
	}
	r.Coverage = calculateCoverage(r.Counts)
	got := renderCoverage(r, "2026-08-01")
	want := `# Test262 coverage report

` + "**Report date:** 2026-08-01  \n" + "**Pinned Test262 commit:** `abc`  \n" + `**ECMA target:** ECMAScript 2025

This report is generated from the complete pinned Test262 suite. **Test262
execution coverage** is the percentage of tests that reached execution (pass,
fail, or timeout); unsupported tests are excluded. **Test262 overall pass rate**
is passes divided by every test in the suite, including unsupported tests. These
runner metrics do not by themselves claim complete ECMAScript conformance.

## Test262 abc (full)

**ECMA target:** ECMAScript 2025<br>
**Test262 execution coverage:** 66.67% (4/6 tests reached execution)<br>
**Test262 overall pass rate:** 33.33% (2/6)<br>
**Pass rate among executed tests:** 50.00% (2/4)

| pass | fail | skip | unsupported | timeout | total |
|---:|---:|---:|---:|---:|---:|
| 2 | 1 | 1 | 1 | 1 | 6 |

### By feature

| feature | pass | fail | unsupported | timeout | total |
|---|---:|---:|---:|---:|---:|
| a | 1 | 1 | 0 | 1 | 4 |
| z | 0 | 0 | 1 | 0 | 1 |
`
	if got != want {
		t.Fatalf("renderCoverage() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if again := renderCoverage(r, "2026-08-01"); again != got {
		t.Fatal("renderCoverage() changed for identical input")
	}
}

func TestRenderChangeSummaryShowsOnlyImprovements(t *testing.T) {
	previous := baselineSnapshot{
		Counts:    counts{Pass: 10, Fail: 2, Unsupported: 8, Total: 20},
		ByFeature: map[string]counts{"a": {Pass: 5, Unsupported: 1, Total: 6}, "unchanged": {Pass: 1, Total: 1}},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{
		Counts:    counts{Pass: 11, Fail: 2, Unsupported: 7, Total: 20},
		ByFeature: map[string]counts{"a": {Pass: 6, Total: 6}, "unchanged": {Pass: 1, Total: 1}},
	}
	current.Coverage = calculateCoverage(current.Counts)

	want := `## Test262 PR baseline diff

> [!TIP]
> **Test262 check passed. No regression compared with main.**

🟢 improvement · 🔴 regression · unchanged values are shown as ` + "`0`" + `

### Test262 rate changes

| Metric | main | PR | Diff |
|---|---:|---:|---:|
| Test262 execution coverage | 60.00% | 65.00% | 🟢 ▲ +5.00 pp |
| Test262 overall pass rate | 50.00% | 55.00% | 🟢 ▲ +5.00 pp |
| Pass rate among executed tests | 83.33% | 84.62% | 🟢 ▲ +1.28 pp |

### Overall status diff

| Status | main | PR | Diff |
|---|---:|---:|---:|
| pass | 10 | 11 | 🟢 ▲ +1 |
| fail | 2 | 2 | ` + "`0`" + ` |
| skip | 0 | 0 | ` + "`0`" + ` |
| timeout | 0 | 0 | ` + "`0`" + ` |
| unsupported | 8 | 7 | 🟢 ▼ -1 |
| total | 20 | 20 | ` + "`0`" + ` |

### Changed features

Only features with changed results are included.

| Feature | pass | fail | skip | timeout | unsupported | total |
|---|---:|---:|---:|---:|---:|---:|
| a | 🟢 ▲ +1 | ` + "`0`" + ` | ` + "`0`" + ` | ` + "`0`" + ` | 🟢 ▼ -1 | ` + "`0`" + ` |
`
	if got := renderChangeSummary(previous, current); got != want {
		t.Fatalf("renderChangeSummary() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderChangeSummaryShowsRegression(t *testing.T) {
	previous := baselineSnapshot{
		Counts:    counts{Pass: 10, Fail: 2, Unsupported: 8, Total: 20},
		ByFeature: map[string]counts{"b": {Pass: 3, Total: 3}, "unchanged": {Pass: 1, Total: 1}},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{
		Counts:    counts{Pass: 9, Fail: 3, Unsupported: 8, Total: 20},
		ByFeature: map[string]counts{"b": {Pass: 2, Fail: 1, Total: 3}, "unchanged": {Pass: 1, Total: 1}},
	}
	current.Coverage = calculateCoverage(current.Counts)

	want := `## Test262 PR baseline diff

> [!CAUTION]
> **Regression detected compared with main.**

🟢 improvement · 🔴 regression · unchanged values are shown as ` + "`0`" + `

### Test262 rate changes

| Metric | main | PR | Diff |
|---|---:|---:|---:|
| Test262 overall pass rate | 50.00% | 45.00% | 🔴 ▼ -5.00 pp |
| Pass rate among executed tests | 83.33% | 75.00% | 🔴 ▼ -8.33 pp |

### Overall status diff

| Status | main | PR | Diff |
|---|---:|---:|---:|
| pass | 10 | 9 | 🔴 ▼ -1 |
| fail | 2 | 3 | 🔴 ▲ +1 |
| skip | 0 | 0 | ` + "`0`" + ` |
| timeout | 0 | 0 | ` + "`0`" + ` |
| unsupported | 8 | 8 | ` + "`0`" + ` |
| total | 20 | 20 | ` + "`0`" + ` |

### Changed features

Only features with changed results are included.

| Feature | pass | fail | skip | timeout | unsupported | total |
|---|---:|---:|---:|---:|---:|---:|
| b | 🔴 ▼ -1 | 🔴 ▲ +1 | ` + "`0`" + ` | ` + "`0`" + ` | ` + "`0`" + ` | ` + "`0`" + ` |
`
	if got := renderChangeSummary(previous, current); got != want {
		t.Fatalf("renderChangeSummary() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderChangeSummaryTreatsPinnedCommitChangeAsRegression(t *testing.T) {
	previous := baselineSnapshot{
		Commit:    "main-commit",
		Counts:    counts{Pass: 1, Total: 1},
		ByFeature: map[string]counts{"a": {Pass: 1, Total: 1}},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{
		Commit:    "pr-commit",
		Counts:    previous.Counts,
		ByFeature: previous.ByFeature,
		Coverage:  previous.Coverage,
	}
	want := `## Test262 PR baseline diff

> [!CAUTION]
> **Regression detected compared with main.**

🟢 improvement · 🔴 regression · unchanged values are shown as ` + "`0`" + `

### Metadata change

Pinned Test262 commit: ` + "`main-commit`" + ` → ` + "`pr-commit`" + ` 🔴

### Overall status diff

| Status | main | PR | Diff |
|---|---:|---:|---:|
| pass | 1 | 1 | ` + "`0`" + ` |
| fail | 0 | 0 | ` + "`0`" + ` |
| skip | 0 | 0 | ` + "`0`" + ` |
| timeout | 0 | 0 | ` + "`0`" + ` |
| unsupported | 0 | 0 | ` + "`0`" + ` |
| total | 1 | 1 | ` + "`0`" + ` |
`
	if got := renderChangeSummary(previous, current); got != want {
		t.Fatalf("renderChangeSummary() mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderChangeSummaryWhenNothingChanged(t *testing.T) {
	previous := baselineSnapshot{
		Counts:    counts{Pass: 1, Total: 1},
		ByFeature: map[string]counts{"a": {Pass: 1, Total: 1}},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{Counts: previous.Counts, ByFeature: previous.ByFeature, Coverage: previous.Coverage}
	want := `## Test262 PR baseline diff

> [!TIP]
> **Test262 check passed. No result changes compared with main.**

🟢 improvement · 🔴 regression · unchanged values are shown as ` + "`0`" + `

### Overall status diff

| Status | main | PR | Diff |
|---|---:|---:|---:|
| pass | 1 | 1 | ` + "`0`" + ` |
| fail | 0 | 0 | ` + "`0`" + ` |
| skip | 0 | 0 | ` + "`0`" + ` |
| timeout | 0 | 0 | ` + "`0`" + ` |
| unsupported | 0 | 0 | ` + "`0`" + ` |
| total | 1 | 1 | ` + "`0`" + ` |
`
	if got := renderChangeSummary(previous, current); got != want {
		t.Fatalf("renderChangeSummary() = %q, want %q", got, want)
	}
}

func TestValidateManifestReportDate(t *testing.T) {
	valid := manifest{Commit: "abc", ECMAVersion: "ECMAScript 2025", ReportDate: "2026-08-01"}
	if err := validateManifest(valid); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	for name, date := range map[string]string{"missing": "", "invalid": "2026-02-30"} {
		t.Run(name, func(t *testing.T) {
			m := valid
			m.ReportDate = date
			if err := validateManifest(m); err == nil {
				t.Fatal("invalid reportDate accepted")
			}
		})
	}
}

func TestCoverageReportDateOverride(t *testing.T) {
	if got, err := coverageReportDate("2026-08-01", ""); err != nil || got != "2026-08-01" {
		t.Fatalf("manifest date: got %q, %v", got, err)
	}
	if got, err := coverageReportDate("2026-08-01", "2026-08-02"); err != nil || got != "2026-08-02" {
		t.Fatalf("override date: got %q, %v", got, err)
	}
	if _, err := coverageReportDate("2026-08-01", "not-a-date"); err == nil {
		t.Fatal("invalid override accepted")
	}
}

func TestFrontmatter(t *testing.T) {
	m, body, err := frontmatter("// copyright\n/*---\nfeatures:\n - let\nflags: [onlyStrict]\nincludes: [compareArray.js]\nnegative:\n  phase: parse\n  type: SyntaxError\n---*/\nlet = 1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.features, []string{"let"}) || !m.negativeParse || m.negativeType != "SyntaxError" || body != "\nlet = 1" {
		t.Fatalf("%+v %q", m, body)
	}
}

func TestCompareBaselineRejectsCoverageRegression(t *testing.T) {
	want := report{Commit: "abc", Counts: counts{Pass: 2, Total: 2}, ByFeature: map[string]counts{"let": {Pass: 2, Total: 2}}}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := compareBaseline(path, want); err != nil || len(got) != 0 {
		t.Fatalf("unchanged coverage regressed: %v, %v", got, err)
	}
	actual := want
	actual.Counts.Pass = 1
	actual.ByFeature = map[string]counts{"let": {Pass: 1, Fail: 1, Total: 2}}
	if got, err := compareBaseline(path, actual); err != nil || len(got) == 0 {
		t.Fatalf("coverage regression accepted: %v, %v", got, err)
	}
}

func TestRefreshBaselineComparesBeforeWritingSnapshot(t *testing.T) {
	old := report{
		Commit:      "abc",
		ECMAVersion: "ECMAScript 2025",
		Mode:        "full",
		Counts:      counts{Pass: 2, Total: 2},
		ByFeature:   map[string]counts{"let": {Pass: 2, Total: 2}},
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeCanonicalJSON(path, snapshot(old)); err != nil {
		t.Fatal(err)
	}
	actual := old
	actual.Counts = counts{Pass: 1, Fail: 1, Total: 2}
	actual.ByFeature = map[string]counts{"let": {Pass: 1, Fail: 1, Total: 2}}
	actual.Results = []result{{Name: "test/language/let.js", Status: "fail"}}
	actual.Regressions = []string{"not part of the snapshot"}

	problems, err := refreshBaseline(path, actual)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) == 0 {
		t.Fatal("old baseline was not used to detect the regression")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatal("canonical baseline does not end with a newline")
	}
	var stored map[string]any
	if err := json.Unmarshal(b, &stored); err != nil {
		t.Fatal(err)
	}
	if _, ok := stored["results"]; ok {
		t.Fatal("baseline snapshot contains per-test results")
	}
	if _, ok := stored["regressions"]; ok {
		t.Fatal("baseline snapshot contains regressions")
	}
	if got, err := compareBaseline(path, actual); err != nil || len(got) != 0 {
		t.Fatalf("refreshed baseline does not match actual report: %v, %v", got, err)
	}
}

func TestCalculateCoverage(t *testing.T) {
	got := calculateCoverage(counts{Pass: 75, Fail: 5, Unsupported: 20, Total: 100})
	if got.CoveredTests != 80 || got.CoveragePercent != 80 || got.OverallPassPercent != 75 || got.CoveredPassPercent != 93.75 {
		t.Fatalf("unexpected coverage: %+v", got)
	}
}

func TestDiscover(t *testing.T) {
	root := filepath.Join(t.TempDir(), "test")
	if err := os.MkdirAll(filepath.Join(root, "language"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "language", "x.js"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	names, err := discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"test/language/x.js"}) {
		t.Fatal(names)
	}
}

func TestInferredFeature(t *testing.T) {
	tests := map[string]string{
		"test/built-ins/WeakMap/prototype/get.js": "path:built-ins/WeakMap",
		"test/language/expressions/addition/x.js": "path:language/expressions",
		"test/annexB/built-ins/x.js":              "path:annexB/built-ins",
		"smoke/x.js":                              "path:other",
	}
	for name, want := range tests {
		if got := inferredFeature(name); got != want {
			t.Errorf("inferredFeature(%q)=%q, want %q", name, got, want)
		}
	}
}
