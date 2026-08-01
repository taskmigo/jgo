package main

import "encoding/xml"

const (
	statusPass        = "pass"
	statusFail        = "fail"
	statusSkip        = "skip"
	statusUnsupported = "unsupported"
	statusTimeout     = "timeout"
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
