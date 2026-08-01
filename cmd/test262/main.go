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
	Commit string   `json:"commit"`
	Tests  []string `json:"tests"`
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
	Mode        string            `json:"mode"`
	Counts      counts            `json:"counts"`
	ByFeature   map[string]counts `json:"byFeature"`
	Results     []result          `json:"results"`
	Regressions []string          `json:"regressions,omitempty"`
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
	steps := flag.Uint64("steps", 100000, "steps per test")
	timeout := flag.Duration("timeout", 2*time.Second, "timeout per test")
	baseline := flag.String("baseline", "", "full-run baseline used to reject coverage regressions")
	flag.Parse()
	b, err := os.ReadFile(*selection)
	fatal(err)
	var m manifest
	fatal(json.Unmarshal(b, &m))
	if m.Commit == "" {
		fatal(errors.New("invalid selection manifest: missing commit"))
	}
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
	rep := report{Commit: m.Commit, Mode: mode, ByFeature: map[string]counts{}}
	js := suite{}
	for _, name := range names {
		res := runOne(*root, name, *steps, *timeout)
		rep.Results = append(rep.Results, res)
		add(&rep.Counts, res.Status)
		features := res.Features
		if len(features) == 0 {
			features = []string{"(unclassified)"}
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
	if *baseline != "" {
		rep.Regressions = compareBaseline(*baseline, rep)
	}
	writeJSON(*jsonOut, rep)
	xb, err := xml.MarshalIndent(js, "", "  ")
	fatal(err)
	fatal(os.WriteFile(*junitOut, append([]byte(xml.Header), xb...), 0644))
	out := renderSummary(rep)
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

func compareBaseline(path string, actual report) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return []string{"baseline: " + err.Error()}
	}
	var expected report
	if err := json.Unmarshal(b, &expected); err != nil {
		return []string{"baseline: " + err.Error()}
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
	return problems
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
		// Test262 requires these harness files for every non-raw test, before
		// any test-specific includes. If Gots cannot compile the harness, the
		// result is unsupported rather than a false pass.
		includes := append([]string{"sta.js", "assert.js"}, meta.includes...)
		for _, inc := range includes {
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
	} else {
		res.Status = "fail"
		res.Reason = err.Error()
	}
	return res
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
func renderSummary(r report) string {
	c := r.Counts
	b := &strings.Builder{}
	fmt.Fprintf(b, "## Test262 %s (%s)\n\n| pass | fail | skip | unsupported | timeout | total |\n|---:|---:|---:|---:|---:|---:|\n| %d | %d | %d | %d | %d | %d |\n\n### By feature\n\n| feature | pass | fail | unsupported | timeout | total |\n|---|---:|---:|---:|---:|---:|\n", r.Commit, r.Mode, c.Pass, c.Fail, c.Skip, c.Unsupported, c.Timeout, c.Total)
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
func writeJSON(path string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	fatal(e)
	fatal(os.WriteFile(path, b, 0644))
}
func fatal(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
}
