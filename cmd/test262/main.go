package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	gots "github.com/example/gots"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type manifest struct {
	Commit              string   `json:"commit"`
	Tests               []string `json:"tests"`
	Features            []string `json:"features"`
	ExpectedUnsupported []string `json:"expectedUnsupported"`
}
type result struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Reason     string   `json:"reason,omitempty"`
	Features   []string `json:"features,omitempty"`
	DurationMS int64    `json:"durationMs"`
}
type report struct {
	Commit      string   `json:"commit"`
	Total       int      `json:"total"`
	Pass        int      `json:"pass"`
	Fail        int      `json:"fail"`
	Skip        int      `json:"skip"`
	Unsupported int      `json:"unsupported"`
	Timeout     int      `json:"timeout"`
	Results     []result `json:"results"`
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

func main() {
	root := flag.String("test262", "test262", "Test262 checkout or smoke fixture root")
	selection := flag.String("selection", "test262/selection.json", "selection manifest")
	jsonOut := flag.String("json", "test262-report.json", "JSON report")
	junitOut := flag.String("junit", "test262-report.xml", "JUnit report")
	summary := flag.String("summary", "", "summary output (defaults to GITHUB_STEP_SUMMARY)")
	steps := flag.Uint64("steps", 100000, "steps per test")
	flag.Parse()
	b, e := os.ReadFile(*selection)
	fatal(e)
	var m manifest
	fatal(json.Unmarshal(b, &m))
	if m.Commit == "" || len(m.Tests) == 0 {
		fatal(fmt.Errorf("invalid selection manifest"))
	}
	rep := report{Commit: m.Commit}
	js := suite{}
	for _, name := range m.Tests {
		started := time.Now()
		path := filepath.Join(*root, name)
		src, e := os.ReadFile(path)
		res := result{Name: name}
		tc := testcase{Name: name}
		if e != nil {
			res.Status = "fail"
			res.Reason = e.Error()
		} else {
			meta, body, e := frontmatter(string(src))
			if e != nil {
				res.Status = "fail"
				res.Reason = e.Error()
			} else {
				res.Features = meta.features
				r := gots.New(gots.WithMaxSteps(*steps))
				_, e = r.RunString(body)
				negative := meta.negative
				if (e == nil && !negative) || (e != nil && negative) {
					res.Status = "pass"
				} else if e != nil && strings.Contains(e.Error(), "unsupported feature") {
					res.Status = "unsupported"
					res.Reason = e.Error()
				} else {
					res.Status = "fail"
					if e != nil {
						res.Reason = e.Error()
					} else {
						res.Reason = "expected negative test to fail"
					}
				}
			}
		}
		res.DurationMS = time.Since(started).Milliseconds()
		switch res.Status {
		case "pass":
			rep.Pass++
		case "unsupported":
			rep.Unsupported++
			tc.Skipped = &message{res.Reason}
		default:
			rep.Fail++
			tc.Failure = &message{res.Reason}
		}
		rep.Results = append(rep.Results, res)
		js.Cases = append(js.Cases, tc)
	}
	rep.Total = len(rep.Results)
	js.Tests = rep.Total
	js.Failures = rep.Fail
	js.Skipped = rep.Skip + rep.Unsupported
	writeJSON(*jsonOut, rep)
	xb, e := xml.MarshalIndent(js, "", "  ")
	fatal(e)
	fatal(os.WriteFile(*junitOut, append([]byte(xml.Header), xb...), 0644))
	out := fmt.Sprintf("## Test262 %s\n\n| pass | fail | skip | unsupported | total |\n|---:|---:|---:|---:|---:|\n| %d | %d | %d | %d | %d |\n", rep.Commit, rep.Pass, rep.Fail, rep.Skip, rep.Unsupported, rep.Total)
	dest := *summary
	if dest == "" {
		dest = os.Getenv("GITHUB_STEP_SUMMARY")
	}
	if dest != "" {
		fatal(os.WriteFile(dest, []byte(out), 0644))
	} else {
		fmt.Print(out)
	}
	if rep.Fail > 0 {
		os.Exit(1)
	}
}

type metadata struct {
	features []string
	negative bool
}

func frontmatter(s string) (metadata, string, error) {
	if !strings.HasPrefix(s, "/*---") {
		return metadata{}, "", fmt.Errorf("malformed frontmatter")
	}
	end := strings.Index(s, "---*/")
	if end < 0 {
		return metadata{}, "", fmt.Errorf("malformed frontmatter")
	}
	head := s[5:end]
	m := metadata{}
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "features:") {
			x := strings.Trim(strings.TrimPrefix(line, "features:"), " []")
			if x != "" {
				m.features = strings.Split(x, ",")
			}
		}
		if strings.HasPrefix(line, "negative:") {
			m.negative = true
		}
	}
	return m, s[end+5:], nil
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
