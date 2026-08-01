package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunCLISmokeSuite(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	temporary := t.TempDir()
	jsonOutput := filepath.Join(temporary, "report.json")
	junitOutput := filepath.Join(temporary, "report.xml")
	summaryOutput := filepath.Join(temporary, "summary.md")
	args := []string{
		"-test262", "../../test262",
		"-selection", "../../test262/selection.json",
		"-json", jsonOutput,
		"-junit", junitOutput,
		"-summary", summaryOutput,
	}
	var stdout, stderr bytes.Buffer
	if code := runCLI(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	data, err := os.ReadFile(jsonOutput)
	if err != nil {
		t.Fatal(err)
	}
	var report report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Counts != (counts{Pass: 6, Total: 6}) {
		t.Fatalf("unexpected counts: %+v", report.Counts)
	}

	data, err = os.ReadFile(junitOutput)
	if err != nil {
		t.Fatal(err)
	}
	var junit suite
	if err := xml.Unmarshal(data, &junit); err != nil {
		t.Fatal(err)
	}
	if junit.Tests != 6 || junit.Failures != 0 || junit.Skipped != 0 || len(junit.Cases) != 6 {
		t.Fatalf("unexpected JUnit report: %+v", junit)
	}
	if summary, err := os.ReadFile(summaryOutput); err != nil || !strings.Contains(string(summary), "execution coverage:** 100.00% (6/6") {
		t.Fatalf("summary=%q error=%v", summary, err)
	}
}

func TestRunCLIExitCodes(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{"-refresh-baseline"}, &stdout, &stderr); code != 2 {
		t.Fatalf("operational error exit code = %d, want 2", code)
	}
	if got := stderr.String(); got != "-refresh-baseline requires -baseline\n" {
		t.Fatalf("stderr = %q", got)
	}

	temporary := t.TempDir()
	baselinePath := filepath.Join(temporary, "baseline.json")
	baseline := baselineSnapshot{
		SchemaVersion: baselineSchemaVersion,
		Test262Commit: "b363f29d3c43c626dc852744ad64a0b48a003693",
		ECMAVersion:   "ECMAScript 2027 draft snapshot 2026-08-01",
		Mode:          "selection",
		Counts:        counts{Pass: 4, Total: 3},
		ByFeature:     map[string]counts{},
	}
	if err := writeJSON(baselinePath, baseline); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	args := []string{
		"-test262", "../../test262",
		"-selection", "../../test262/selection.json",
		"-baseline", baselinePath,
		"-json", filepath.Join(temporary, "report.json"),
		"-junit", filepath.Join(temporary, "report.xml"),
		"-summary", filepath.Join(temporary, "summary.md"),
	}
	if code := runCLI(args, &stdout, &stderr); code != 1 {
		t.Fatalf("regression exit code = %d, want 1; stderr=%q", code, stderr.String())
	}
}

func TestRunnerDefaultStepBudgetCoversLongFiniteCohorts(t *testing.T) {
	config, err := parseRunnerConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if config.maxSteps != defaultMaxSteps {
		t.Fatalf("max steps = %d, want %d", config.maxSteps, defaultMaxSteps)
	}
}

func TestSupportedVariantsAreNotBlanketExcluded(t *testing.T) {
	capabilities := capabilityManifest{Supported: []string{"strict-mode", "source-text-modules"}}
	for _, testCase := range []executionCase{{Variant: "strict"}, {Variant: "module"}} {
		if code, detail := unsupportedTestReason(testCase, capabilities); code != "" || detail != "" {
			t.Fatalf("variant %q excluded: code=%q detail=%q", testCase.Variant, code, detail)
		}
	}
}

func TestSupportedVariantStillHonorsUnsupportedDependencies(t *testing.T) {
	capabilities := capabilityManifest{
		Supported:   []string{"strict-mode"},
		Unsupported: []unsupportedCapability{{Feature: "bigint", Reason: "unsupported-bigint"}},
	}
	testCase := executionCase{Variant: "strict", Metadata: metadata{features: []string{"BigInt"}}}
	code, _ := unsupportedTestReason(testCase, capabilities)
	if code != "unsupported-bigint" {
		t.Fatalf("unsupported reason = %q, want unsupported-bigint", code)
	}
}

func TestRenderCoverageIsDeterministic(t *testing.T) {
	r := report{
		Test262Commit:           "abc",
		ECMA262Commit:           "def",
		SourceFileDenominator:   6,
		ExpandedCaseDenominator: 6,
		ECMAVersion:             "ECMAScript 2025",
		Mode:                    "full",
		Counts:                  counts{Pass: 2, Fail: 1, Skip: 1, Unsupported: 1, Timeout: 1, Total: 6},
		ByFeature: map[string]counts{
			"z": {Unsupported: 1, Total: 1},
			"a": {Pass: 1, Fail: 1, Skip: 1, Timeout: 1, Total: 4},
		},
	}
	r.Coverage = calculateCoverage(r.Counts)
	got := renderCoverage(r, "2026-08-01")
	want := `# Test262 coverage report

` + "**Report date:** 2026-08-01  \n" + "**Pinned Test262 commit:** `abc`  \n" + "**Pinned ECMA-262 commit:** `def`<br>\n" + `**ECMA target:** ECMAScript 2025

This report is generated from the complete pinned Test262 suite. **Test262
execution coverage** is the percentage of tests that reached execution (pass,
fail, or timeout); unsupported tests are excluded. **Test262 overall pass rate**
is passes divided by every test in the suite, including unsupported tests. These
runner metrics do not by themselves claim complete ECMAScript conformance.

## Test262 abc (full)

**ECMA-262 commit:** ` + "`def`" + `<br>
**ECMA target:** ECMAScript 2025<br>
**Source files:** 6<br>
**Expanded cases:** 6<br>
**Test262 execution coverage:** 66.67% (4/6 cases reached execution)<br>
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
		SchemaVersion: baselineSchemaVersion,
		Counts:        counts{Pass: 10, Fail: 2, Unsupported: 8, Total: 20},
		ByFeature:     map[string]counts{"a": {Pass: 5, Unsupported: 1, Total: 6}, "unchanged": {Pass: 1, Total: 1}},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{
		SchemaVersion: baselineSchemaVersion,
		Counts:        counts{Pass: 11, Fail: 2, Unsupported: 7, Total: 20},
		ByFeature:     map[string]counts{"a": {Pass: 6, Total: 6}, "unchanged": {Pass: 1, Total: 1}},
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
		SchemaVersion:  baselineSchemaVersion,
		Counts:         counts{Pass: 10, Fail: 2, Unsupported: 8, Total: 20},
		ByFeature:      map[string]counts{"b": {Pass: 3, Total: 3}, "unchanged": {Pass: 1, Total: 1}},
		PassingCaseIDs: []string{"test/b.js#sloppy"},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{
		SchemaVersion: baselineSchemaVersion,
		Counts:        counts{Pass: 9, Fail: 3, Unsupported: 8, Total: 20},
		ByFeature:     map[string]counts{"b": {Pass: 2, Fail: 1, Total: 3}, "unchanged": {Pass: 1, Total: 1}},
		Results:       []result{{Name: "test/b.js#sloppy", Status: statusFail}},
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
		SchemaVersion: baselineSchemaVersion,
		Test262Commit: "main-commit",
		Counts:        counts{Pass: 1, Total: 1},
		ByFeature:     map[string]counts{"a": {Pass: 1, Total: 1}},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{
		SchemaVersion: baselineSchemaVersion,
		Test262Commit: "pr-commit",
		Counts:        previous.Counts,
		ByFeature:     previous.ByFeature,
		Coverage:      previous.Coverage,
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
		SchemaVersion: baselineSchemaVersion,
		Counts:        counts{Pass: 1, Total: 1},
		ByFeature:     map[string]counts{"a": {Pass: 1, Total: 1}},
	}
	previous.Coverage = calculateCoverage(previous.Counts)
	current := report{SchemaVersion: baselineSchemaVersion, Counts: previous.Counts, ByFeature: previous.ByFeature, Coverage: previous.Coverage}
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
	valid := manifest{Commit: strings.Repeat("a", 40), ECMA262Commit: strings.Repeat("b", 40), ECMAVersion: "ECMAScript 2025", ReportDate: "2026-08-01", CapabilityVersion: "v1"}
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
	want := report{
		SchemaVersion: baselineSchemaVersion, Test262Commit: "abc",
		SourceFileDenominator: 1, ExpandedCaseDenominator: 2,
		Counts: counts{Pass: 2, Total: 2}, ByFeature: map[string]counts{"let": {Pass: 2, Total: 2}},
		Results: []result{{Name: "test/language/let.js#sloppy", Status: statusPass}, {Name: "test/language/let.js#strict", Status: statusPass}},
	}
	b, err := json.Marshal(snapshot(want))
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
	actual.Results[1].Status = statusFail
	if got, err := compareBaseline(path, actual); err != nil || len(got) == 0 {
		t.Fatalf("coverage regression accepted: %v, %v", got, err)
	}
}

func TestRefreshBaselineComparesBeforeWritingSnapshot(t *testing.T) {
	old := report{
		SchemaVersion: baselineSchemaVersion, Test262Commit: "abc",
		ECMAVersion: "ECMAScript 2025", Mode: "full",
		SourceFileDenominator: 1, ExpandedCaseDenominator: 2,
		Counts: counts{Pass: 2, Total: 2}, ByFeature: map[string]counts{"let": {Pass: 2, Total: 2}},
		Results: []result{{Name: "test/language/let.js#sloppy", Status: statusPass}, {Name: "test/language/let.js#strict", Status: statusPass}},
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeCanonicalJSON(path, snapshot(old)); err != nil {
		t.Fatal(err)
	}
	actual := old
	actual.Counts = counts{Pass: 1, Fail: 1, Total: 2}
	actual.ByFeature = map[string]counts{"let": {Pass: 1, Fail: 1, Total: 2}}
	actual.Results = []result{{Name: "test/language/let.js#sloppy", Status: statusPass}, {Name: "test/language/let.js#strict", Status: statusFail}}
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

func TestExecutionVariants(t *testing.T) {
	tests := []struct {
		name     string
		metadata metadata
		want     []string
	}{
		{name: "unflagged", want: []string{"sloppy", "strict"}},
		{name: "onlyStrict", metadata: metadata{flags: []string{"onlyStrict"}}, want: []string{"strict"}},
		{name: "noStrict", metadata: metadata{flags: []string{"noStrict"}}, want: []string{"sloppy"}},
		{name: "module", metadata: metadata{flags: []string{"module"}}, want: []string{"module"}},
		{name: "raw", metadata: metadata{flags: []string{"raw"}}, want: []string{"raw"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := executionVariants(test.metadata); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("executionVariants() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSnapshotSortsStableCaseIDsAndReasons(t *testing.T) {
	report := report{
		Results: []result{
			{Name: "test/z.js#strict", Status: statusUnsupported, UnsupportedReason: "unsupported-feature"},
			{Name: "test/b.js#sloppy", Status: statusFail},
			{Name: "test/a.js#sloppy", Status: statusPass},
			{Name: "test/c.js#sloppy", Status: statusTimeout},
			{Name: "test/a.js#strict", Status: statusUnsupported, UnsupportedReason: "unsupported-feature"},
		},
	}
	snapshot := snapshot(report)
	if !reflect.DeepEqual(snapshot.PassingCaseIDs, []string{"test/a.js#sloppy"}) ||
		!reflect.DeepEqual(snapshot.FailingCaseIDs, []string{"test/b.js#sloppy"}) ||
		!reflect.DeepEqual(snapshot.TimeoutCaseIDs, []string{"test/c.js#sloppy"}) {
		t.Fatalf("unexpected snapshot IDs: %+v", snapshot)
	}
	wantUnsupported := []unsupportedCase{
		{CaseID: "test/a.js#strict", Reason: "unsupported-feature"},
		{CaseID: "test/z.js#strict", Reason: "unsupported-feature"},
	}
	if !reflect.DeepEqual(snapshot.UnsupportedCases, wantUnsupported) {
		t.Fatalf("unsupported cases = %+v, want %+v", snapshot.UnsupportedCases, wantUnsupported)
	}
}

func TestCanonicalReportExcludesTimings(t *testing.T) {
	report := report{Results: []result{{Name: "test/a.js#sloppy", Status: statusPass}}}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := writeCanonicalJSON(path, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("duration")) {
		t.Fatalf("canonical report contains timing data: %s", data)
	}
}

func TestV1BaselineResetMustBeExplicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	legacy := map[string]any{
		"commit": "test262", "counts": counts{Pass: 1, Total: 1},
	}
	if err := writeJSON(path, legacy); err != nil {
		t.Fatal(err)
	}
	report := report{
		SchemaVersion: baselineSchemaVersion, Test262Commit: "test262",
		SourceFileDenominator: 1, ExpandedCaseDenominator: 2,
		Counts: counts{Pass: 1, Unsupported: 1, Total: 2},
		Results: []result{
			{Name: "test/a.js#sloppy", Status: statusPass},
			{Name: "test/a.js#strict", Status: statusUnsupported, UnsupportedReason: "unsupported-feature"},
		},
	}
	config := runnerConfig{baseline: path, refreshBaseline: true}
	applyBaseline(config, &report)
	if len(report.Regressions) == 0 {
		t.Fatal("implicit v1 reset was accepted")
	}
	report.Regressions = nil
	config.allowV1Reset = true
	applyBaseline(config, &report)
	if len(report.Regressions) != 0 {
		t.Fatalf("explicit v1 reset failed: %v", report.Regressions)
	}
	stored, err := readBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SchemaVersion != baselineSchemaVersion {
		t.Fatalf("stored schema = %d", stored.SchemaVersion)
	}
}

func TestUnsupportedTestReason(t *testing.T) {
	tests := []struct {
		name     string
		metadata metadata
		want     string
	}{
		{name: "test/language/module-code/source-text.js", metadata: metadata{flags: []string{"module"}}, want: "unsupported feature: modules"},
		{name: "test/language/statements/x.js", metadata: metadata{flags: []string{"onlyStrict"}}, want: "unsupported feature: strict mode"},
		{name: "test/annexB/language/x.js", want: "unsupported feature: Annex B"},
		{name: "test/language/expressions/x.js", metadata: metadata{features: []string{"BigInt"}}, want: "unsupported feature: BigInt"},
		{name: "test/language/expressions/x.js"},
	}
	for _, test := range tests {
		_, got := unsupportedTestReason(executionCase{SourceName: test.name, Variant: variantForLegacyRun(test.metadata), Metadata: test.metadata}, capabilityManifest{})
		if got != test.want {
			t.Errorf("unsupportedTestReason(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestObjectDescriptorsRunWithoutHarnessShim(t *testing.T) {
	root := t.TempDir()
	testDir := filepath.Join(root, "test", "built-ins", "Object")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTest := func(name, features, body string) string {
		t.Helper()
		path := filepath.Join(testDir, name)
		source := "/*---\nfeatures: [" + features + "]\n---*/\n" + body
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(rel)
	}

	objectIsTest := writeTest("is-descriptor.js", "Object.is", `
let descriptor = Object.getOwnPropertyDescriptor(Object, "is");
assert.sameValue(descriptor.value, Object.is);
assert.sameValue(descriptor.writable, true);
assert.sameValue(descriptor.enumerable, false);
assert.sameValue(descriptor.configurable, true);`)
	if got := runOne(root, objectIsTest, 100000, 2*time.Second); got.Status != "pass" {
		t.Fatalf("Object.is descriptor test: status=%s reason=%s", got.Status, got.Reason)
	}

	unrelatedTest := writeTest("unrelated-descriptor.js", "", `Object.getOwnPropertyDescriptor(Object, "hasOwn");`)
	if got := runOne(root, unrelatedTest, 100000, 2*time.Second); got.Status != "pass" {
		t.Fatalf("unrelated descriptor test: status=%s reason=%s", got.Status, got.Reason)
	}
}
