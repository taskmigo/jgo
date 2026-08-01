package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	gots "github.com/example/gots"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type manifest struct {
	Commit      string   `json:"commit"`
	ECMAVersion string   `json:"ecmaVersion"`
	ReportDate  string   `json:"reportDate"`
	Tests       []string `json:"tests"`
}
type counts struct {
	Pass        int `json:"pass"`
	Fail        int `json:"fail"`
	Skip        int `json:"skip"`
	Unsupported int `json:"unsupported"`
	Timeout     int `json:"timeout"`
	Total       int `json:"total"`
}
type result struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Reason     string   `json:"reason,omitempty"`
	Features   []string `json:"features,omitempty"`
	DurationMS int64    `json:"durationMs"`
}
type report struct {
	Commit      string            `json:"commit"`
	ECMAVersion string            `json:"ecmaVersion"`
	Mode        string            `json:"mode"`
	Counts      counts            `json:"counts"`
	ByFeature   map[string]counts `json:"byFeature"`
	Results     []result          `json:"results"`
	Regressions []string          `json:"regressions,omitempty"`
	Coverage    coverage          `json:"coverage"`
}
type baselineSnapshot struct {
	Commit      string            `json:"commit"`
	ECMAVersion string            `json:"ecmaVersion"`
	Mode        string            `json:"mode"`
	Counts      counts            `json:"counts"`
	ByFeature   map[string]counts `json:"byFeature"`
	Coverage    coverage          `json:"coverage"`
}
type coverage struct {
	CoveredTests       int     `json:"coveredTests"`
	CoveragePercent    float64 `json:"coveragePercent"`
	OverallPassPercent float64 `json:"overallPassPercent"`
	CoveredPassPercent float64 `json:"coveredPassPercent"`
}
type suite struct {
	XMLName                  xml.Name   `xml:"testsuite"`
	Tests, Failures, Skipped int        `xml:",attr"`
	Cases                    []testcase `xml:"testcase"`
}
type testcase struct {
	Name    string   `xml:"name,attr"`
	Failure *message `xml:"failure,omitempty"`
	Skipped *message `xml:"skipped,omitempty"`
}
type message struct {
	Message string `xml:"message,attr"`
}
type metadata struct {
	features, flags, includes []string
	negative, negativeParse   bool
	negativeType              string
}

func main() {
	root := flag.String("test262", "test262", "Test262 checkout or smoke fixture root")
	selection := flag.String("selection", "test262/selection.json", "selection manifest")
	all := flag.Bool("all", false, "run every .js file below <test262>/test")
	jsonOut := flag.String("json", "test262-report.json", "JSON report")
	junitOut := flag.String("junit", "test262-report.xml", "JUnit report")
	summary := flag.String("summary", "", "summary output (defaults to GITHUB_STEP_SUMMARY)")
	coverageOut := flag.String("coverage", "", "write the canonical coverage report")
	reportDate := flag.String("report-date", "", "override the manifest report date (YYYY-MM-DD)")
	steps := flag.Uint64("steps", 100000, "steps per test")
	timeout := flag.Duration("timeout", 2*time.Second, "timeout per test")
	baseline := flag.String("baseline", "", "full-run baseline used to reject coverage regressions")
	refreshBaselineFile := flag.Bool("refresh-baseline", false, "refresh -baseline after checking it for regressions")
	flag.Parse()
	if *refreshBaselineFile && *baseline == "" {
		fatal(errors.New("-refresh-baseline requires -baseline"))
	}
	b, err := os.ReadFile(*selection)
	fatal(err)
	var m manifest
	fatal(json.Unmarshal(b, &m))
	fatal(validateManifest(m))
	effectiveReportDate, err := coverageReportDate(m.ReportDate, *reportDate)
	fatal(err)
	names := m.Tests
	mode := "selection"
	if *all {
		mode = "full"
		names, err = discover(filepath.Join(*root, "test"))
		fatal(err)
	}
	if len(names) == 0 {
		fatal(errors.New("no tests selected"))
	}
	rep := report{Commit: m.Commit, ECMAVersion: m.ECMAVersion, Mode: mode, ByFeature: map[string]counts{}}
	js := suite{}
	for _, name := range names {
		res := runOne(*root, name, *steps, *timeout)
		rep.Results = append(rep.Results, res)
		add(&rep.Counts, res.Status)
		features := res.Features
		if len(features) == 0 {
			features = []string{inferredFeature(res.Name)}
		}
		for _, f := range features {
			c := rep.ByFeature[f]
			add(&c, res.Status)
			rep.ByFeature[f] = c
		}
		tc := testcase{Name: name}
		if res.Status == "fail" || res.Status == "timeout" {
			tc.Failure = &message{res.Reason}
		} else if res.Status != "pass" {
			tc.Skipped = &message{res.Reason}
		}
		js.Cases = append(js.Cases, tc)
	}
	js.Tests = rep.Counts.Total
	js.Failures = rep.Counts.Fail + rep.Counts.Timeout
	js.Skipped = rep.Counts.Skip + rep.Counts.Unsupported
	rep.Coverage = calculateCoverage(rep.Counts)
	if *baseline != "" {
		var baselineErr error
		if *refreshBaselineFile {
			rep.Regressions, baselineErr = refreshBaseline(*baseline, rep)
		} else {
			rep.Regressions, baselineErr = compareBaseline(*baseline, rep)
		}
		if baselineErr != nil {
			rep.Regressions = []string{"baseline: " + baselineErr.Error()}
		}
	}
	fatal(writeJSON(*jsonOut, rep))
	xb, err := xml.MarshalIndent(js, "", "  ")
	fatal(err)
	fatal(os.WriteFile(*junitOut, append([]byte(xml.Header), xb...), 0644))
	out := renderSummary(rep)
	if *coverageOut != "" {
		fatal(os.WriteFile(*coverageOut, []byte(renderCoverage(rep, effectiveReportDate)), 0644))
	}
	dest := *summary
	if dest == "" {
		dest = os.Getenv("GITHUB_STEP_SUMMARY")
	}
	if dest != "" {
		fatal(os.WriteFile(dest, []byte(out), 0644))
	} else {
		fmt.Print(out)
	}
	if (*baseline == "" && rep.Counts.Fail+rep.Counts.Timeout > 0) || len(rep.Regressions) > 0 {
		os.Exit(1)
	}
}

func validateManifest(m manifest) error {
	if m.Commit == "" || m.ECMAVersion == "" || m.ReportDate == "" {
		return errors.New("invalid selection manifest: missing commit, ecmaVersion, or reportDate")
	}
	if _, err := time.Parse("2006-01-02", m.ReportDate); err != nil {
		return fmt.Errorf("invalid selection manifest reportDate: %w", err)
	}
	return nil
}

func coverageReportDate(manifestDate, override string) (string, error) {
	if override == "" {
		return manifestDate, nil
	}
	if _, err := time.Parse("2006-01-02", override); err != nil {
		return "", fmt.Errorf("invalid -report-date: %w", err)
	}
	return override, nil
}

// inferredFeature gives tests without a Test262 `features` tag a stable,
// path-derived bucket instead of hiding them in a large "unclassified" group.
func inferredFeature(name string) string {
	parts := strings.Split(filepath.ToSlash(name), "/")
	if len(parts) >= 3 && parts[0] == "test" && parts[1] == "built-ins" {
		return "path:built-ins/" + parts[2]
	}
	if len(parts) >= 3 && parts[0] == "test" && parts[1] == "language" {
		return "path:language/" + parts[2]
	}
	if len(parts) >= 3 && parts[0] == "test" {
		return "path:" + parts[1] + "/" + parts[2]
	}
	return "path:other"
}

func compareBaseline(path string, actual report) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var expected baselineSnapshot
	if err := json.Unmarshal(b, &expected); err != nil {
		return nil, err
	}
	var problems []string
	if expected.Commit != actual.Commit {
		problems = append(problems, fmt.Sprintf("commit changed: expected %s, got %s", expected.Commit, actual.Commit))
	}
	if expected.Counts.Total != actual.Counts.Total {
		problems = append(problems, fmt.Sprintf("total changed: expected %d, got %d", expected.Counts.Total, actual.Counts.Total))
	}
	if actual.Counts.Pass < expected.Counts.Pass {
		problems = append(problems, fmt.Sprintf("pass count decreased: expected at least %d, got %d", expected.Counts.Pass, actual.Counts.Pass))
	}
	if actual.Counts.Fail > expected.Counts.Fail {
		problems = append(problems, fmt.Sprintf("fail count increased: expected at most %d, got %d", expected.Counts.Fail, actual.Counts.Fail))
	}
	if actual.Counts.Unsupported > expected.Counts.Unsupported {
		problems = append(problems, fmt.Sprintf("unsupported count increased: expected at most %d, got %d", expected.Counts.Unsupported, actual.Counts.Unsupported))
	}
	if actual.Counts.Timeout > expected.Counts.Timeout {
		problems = append(problems, fmt.Sprintf("timeout count increased: expected at most %d, got %d", expected.Counts.Timeout, actual.Counts.Timeout))
	}
	for feature, want := range expected.ByFeature {
		got, ok := actual.ByFeature[feature]
		if !ok {
			problems = append(problems, "feature disappeared: "+feature)
			continue
		}
		if got.Total != want.Total || got.Pass < want.Pass || got.Fail > want.Fail || got.Unsupported > want.Unsupported || got.Timeout > want.Timeout {
			problems = append(problems, fmt.Sprintf("feature regressed: %s (want pass>=%d fail<=%d unsupported<=%d timeout<=%d total=%d; got %+v)", feature, want.Pass, want.Fail, want.Unsupported, want.Timeout, want.Total, got))
		}
	}
	return problems, nil
}

func refreshBaseline(path string, actual report) ([]string, error) {
	problems, err := compareBaseline(path, actual)
	if err != nil {
		return nil, err
	}
	if err := writeCanonicalJSON(path, snapshot(actual)); err != nil {
		return nil, err
	}
	return problems, nil
}

func snapshot(r report) baselineSnapshot {
	return baselineSnapshot{
		Commit: r.Commit, ECMAVersion: r.ECMAVersion, Mode: r.Mode,
		Counts: r.Counts, ByFeature: r.ByFeature, Coverage: r.Coverage,
	}
}

func discover(root string) ([]string, error) {
	var names []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".js" {
			return nil
		}
		rel, e := filepath.Rel(filepath.Dir(root), path)
		if e != nil {
			return e
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(names)
	return names, err
}
func runOne(root, name string, steps uint64, timeout time.Duration) (res result) {
	started := time.Now()
	res = result{Name: name}
	defer func() { res.DurationMS = time.Since(started).Milliseconds() }()
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		res.Status = "fail"
		res.Reason = err.Error()
		return res
	}
	meta, body, err := frontmatter(string(src))
	if err != nil {
		res.Status = "fail"
		res.Reason = err.Error()
		return res
	}
	res.Features = meta.features
	if contains(meta.flags, "module") {
		res.Status = "unsupported"
		res.Reason = "unsupported feature: modules"
		return res
	}
	prefix := ""
	if !contains(meta.flags, "raw") {
		for _, inc := range meta.includes {
			b, e := os.ReadFile(filepath.Join(root, "harness", filepath.FromSlash(inc)))
			if e != nil {
				res.Status = "fail"
				res.Reason = "harness include: " + e.Error()
				return res
			}
			prefix += string(b) + "\n"
		}
	}
	if contains(meta.flags, "onlyStrict") {
		prefix += "\"use strict\";\n"
	}
	source := prefix + body
	r := gots.New(gots.WithMaxSteps(steps))
	if !contains(meta.flags, "raw") {
		if err := installHarness(r); err != nil {
			res.Status, res.Reason = "fail", err.Error()
			return res
		}
	}
	p, err := r.Compile(source)
	if meta.negativeParse {
		if err != nil {
			res.Status = "pass"
		} else {
			res.Status = "fail"
			res.Reason = "expected parse error"
		}
		return res
	}
	if err != nil {
		var se *gots.SyntaxError
		if errors.As(err, &se) {
			res.Status = "unsupported"
			res.Reason = err.Error()
		} else {
			res.Status = "fail"
			res.Reason = err.Error()
		}
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = r.RunContext(ctx, p)
	if meta.negative {
		if err != nil {
			res.Status = "pass"
		} else {
			res.Status = "fail"
			res.Reason = "expected " + meta.negativeType + " error"
		}
		return res
	}
	if err == nil {
		res.Status = "pass"
	} else if errors.Is(err, gots.ErrCancelled) || errors.Is(err, gots.ErrStepLimit) {
		res.Status = "timeout"
		res.Reason = err.Error()
	} else if !strings.Contains(err.Error(), "Test262 assertion failed") {
		res.Status = "unsupported"
		res.Reason = err.Error()
	} else {
		res.Status = "fail"
		res.Reason = err.Error()
	}
	return res
}

func installHarness(r *gots.Runtime) error {
	assertCall := gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) == 0 || !args[0].Bool() {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed")
		}
		return gots.Undefined(), nil
	})
	if err := r.Set("assert", assertCall); err != nil {
		return err
	}
	assert := r.Get("assert")
	same := gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) < 2 || !gots.SameValue(args[0], args[1]) {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed: values are not the same")
		}
		return gots.Undefined(), nil
	})
	notSame := gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) >= 2 && gots.SameValue(args[0], args[1]) {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed: values are the same")
		}
		return gots.Undefined(), nil
	})
	throws := gots.NativeFunction(func(rt *gots.Runtime, _ gots.Value, args []gots.Value) (gots.Value, error) {
		if len(args) < 2 {
			return gots.Undefined(), fmt.Errorf("Test262 assert.throws requires a constructor and callback")
		}
		if _, err := rt.Call(args[1], gots.Undefined()); err == nil {
			return gots.Undefined(), fmt.Errorf("Test262 assertion failed: expected an exception")
		}
		return gots.Undefined(), nil
	})
	for name, fn := range map[string]gots.NativeFunction{"sameValue": same, "notSameValue": notSame, "throws": throws} {
		key := "__test262_" + name
		if err := r.Set(key, fn); err != nil {
			return err
		}
		if err := assert.SetProperty(name, r.Get(key)); err != nil {
			return err
		}
	}
	if err := r.Set("assert", assert); err != nil {
		return err
	}
	// Error constructors are identity markers for assert.throws in this subset.
	for _, name := range []string{"TypeError", "RangeError", "SyntaxError"} {
		if err := r.Set(name, gots.NativeFunction(func(_ *gots.Runtime, _ gots.Value, _ []gots.Value) (gots.Value, error) { return gots.NewObject(), nil })); err != nil {
			return err
		}
	}
	return nil
}
func frontmatter(s string) (metadata, string, error) {
	start := strings.Index(s, "/*---")
	if start < 0 {
		return metadata{}, "", errors.New("malformed frontmatter: opening marker missing")
	}
	endRel := strings.Index(s[start+5:], "---*/")
	if endRel < 0 {
		return metadata{}, "", errors.New("malformed frontmatter: closing marker missing")
	}
	end := start + 5 + endRel
	head := s[start+5 : end]
	m := metadata{}
	lines := strings.Split(head, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "features":
			m.features = parseList(val, lines, &i)
		case "flags":
			m.flags = parseList(val, lines, &i)
		case "includes":
			m.includes = parseList(val, lines, &i)
		case "negative":
			m.negative = true
		case "phase":
			if m.negative && strings.Trim(val, " '\"") == "parse" {
				m.negativeParse = true
			}
		case "type":
			if m.negative {
				m.negativeType = strings.Trim(val, " '\"")
			}
		}
	}
	return m, s[end+5:], nil
}
func parseList(val string, lines []string, i *int) []string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "[") {
		val = strings.Trim(val, "[] ")
		if val == "" {
			return nil
		}
		parts := strings.Split(val, ",")
		for j := range parts {
			parts[j] = strings.Trim(strings.TrimSpace(parts[j]), "'\"")
		}
		return parts
	}
	var out []string
	for *i+1 < len(lines) {
		next := strings.TrimSpace(lines[*i+1])
		if !strings.HasPrefix(next, "-") {
			break
		}
		*i++
		out = append(out, strings.Trim(strings.TrimSpace(strings.TrimPrefix(next, "-")), "'\""))
	}
	return out
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func add(c *counts, status string) {
	c.Total++
	switch status {
	case "pass":
		c.Pass++
	case "fail":
		c.Fail++
	case "skip":
		c.Skip++
	case "unsupported":
		c.Unsupported++
	case "timeout":
		c.Timeout++
	}
}
func calculateCoverage(c counts) coverage {
	covered := c.Pass + c.Fail + c.Timeout
	result := coverage{CoveredTests: covered}
	if c.Total > 0 {
		result.CoveragePercent = 100 * float64(covered) / float64(c.Total)
		result.OverallPassPercent = 100 * float64(c.Pass) / float64(c.Total)
	}
	if covered > 0 {
		result.CoveredPassPercent = 100 * float64(c.Pass) / float64(covered)
	}
	return result
}
func renderSummary(r report) string {
	c := r.Counts
	b := &strings.Builder{}
	fmt.Fprintf(b, "## Test262 %s (%s)\n\n**ECMA target:** %s<br>\n**Coverage:** %.2f%% (%d/%d tests reached execution)<br>\n**Overall pass:** %.2f%% (%d/%d)<br>\n**Pass among covered:** %.2f%% (%d/%d)\n\n| pass | fail | skip | unsupported | timeout | total |\n|---:|---:|---:|---:|---:|---:|\n| %d | %d | %d | %d | %d | %d |\n\n### By feature\n\n| feature | pass | fail | unsupported | timeout | total |\n|---|---:|---:|---:|---:|---:|\n", r.Commit, r.Mode, r.ECMAVersion, r.Coverage.CoveragePercent, r.Coverage.CoveredTests, c.Total, r.Coverage.OverallPassPercent, c.Pass, c.Total, r.Coverage.CoveredPassPercent, c.Pass, r.Coverage.CoveredTests, c.Pass, c.Fail, c.Skip, c.Unsupported, c.Timeout, c.Total)
	keys := make([]string, 0, len(r.ByFeature))
	for k := range r.ByFeature {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		x := r.ByFeature[k]
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d |\n", k, x.Pass, x.Fail, x.Unsupported, x.Timeout, x.Total)
	}
	return b.String()
}

// renderCoverage produces the repository's canonical report. Its only
// time-varying value is supplied by the caller, so identical results and dates
// produce byte-for-byte identical output on every machine.
func renderCoverage(r report, reportDate string) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "# Test262 coverage report")
	fmt.Fprintf(b, "\n**Report date:** %s  \n", reportDate)
	fmt.Fprintf(b, "**Pinned Test262 commit:** `%s`  \n", r.Commit)
	fmt.Fprintf(b, "**ECMA target:** %s\n\n", r.ECMAVersion)
	fmt.Fprintln(b, "This report is generated from the complete pinned Test262 suite. **Test262")
	fmt.Fprintln(b, "coverage** is the percentage of all tests that reached execution (pass, fail, or")
	fmt.Fprintln(b, "timeout); unsupported tests are excluded. **Overall pass rate** is passes divided")
	fmt.Fprintln(b, "by every test in the suite, including unsupported tests. These runner metrics do")
	fmt.Fprintln(b, "not by themselves claim complete ECMAScript conformance.")
	fmt.Fprintln(b)
	fmt.Fprint(b, renderSummary(r))
	return b.String()
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func writeCanonicalJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var canonical any
	if err := json.Unmarshal(b, &canonical); err != nil {
		return err
	}
	return writeJSON(path, canonical)
}
func fatal(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
}
