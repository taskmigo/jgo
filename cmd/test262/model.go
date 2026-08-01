package main

import "encoding/xml"

const baselineSchemaVersion = 2

const (
	statusPass        = "pass"
	statusFail        = "fail"
	statusSkip        = "skip"
	statusUnsupported = "unsupported"
	statusTimeout     = "timeout"
)

type manifest struct {
	Commit             string   `json:"commit"`
	ECMA262Commit      string   `json:"ecma262Commit"`
	ECMAVersion        string   `json:"ecmaVersion"`
	ReportDate         string   `json:"reportDate"`
	CapabilityManifest string   `json:"capabilityManifest"`
	CapabilityVersion  string   `json:"capabilityVersion"`
	Tests              []string `json:"tests"`
}

type capabilityManifest struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Supported     []string                `json:"supported"`
	Unsupported   []unsupportedCapability `json:"unsupported"`
}

type unsupportedCapability struct {
	Feature string `json:"feature"`
	Reason  string `json:"reason"`
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
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	Reason            string   `json:"reason,omitempty"`
	UnsupportedReason string   `json:"unsupportedReason,omitempty"`
	Features          []string `json:"features,omitempty"`
}

type timedResult struct {
	CaseID     string `json:"caseId"`
	DurationMS int64  `json:"durationMs"`
}

type report struct {
	SchemaVersion           int               `json:"schemaVersion"`
	ECMA262Commit           string            `json:"ecma262Commit"`
	Test262Commit           string            `json:"test262Commit"`
	ECMAVersion             string            `json:"ecmaVersion"`
	CapabilityVersion       string            `json:"capabilityVersion"`
	Mode                    string            `json:"mode"`
	SourceFileDenominator   int               `json:"sourceFileDenominator"`
	ExpandedCaseDenominator int               `json:"expandedCaseDenominator"`
	Counts                  counts            `json:"counts"`
	ByFeature               map[string]counts `json:"byFeature"`
	Results                 []result          `json:"results"`
	Regressions             []string          `json:"regressions,omitempty"`
	Coverage                coverage          `json:"coverage"`
}

type baselineSnapshot struct {
	SchemaVersion           int               `json:"schemaVersion"`
	ECMA262Commit           string            `json:"ecma262Commit"`
	Test262Commit           string            `json:"test262Commit"`
	ECMAVersion             string            `json:"ecmaVersion"`
	CapabilityVersion       string            `json:"capabilityVersion"`
	Mode                    string            `json:"mode"`
	SourceFileDenominator   int               `json:"sourceFileDenominator"`
	ExpandedCaseDenominator int               `json:"expandedCaseDenominator"`
	PassingCaseIDs          []string          `json:"passingCaseIds"`
	FailingCaseIDs          []string          `json:"failingCaseIds"`
	TimeoutCaseIDs          []string          `json:"timeoutCaseIds"`
	UnsupportedCases        []unsupportedCase `json:"unsupportedCases"`
	Counts                  counts            `json:"counts"`
	ByFeature               map[string]counts `json:"byFeature"`
	Coverage                coverage          `json:"coverage"`
}

type unsupportedCase struct {
	CaseID string `json:"caseId"`
	Reason string `json:"reason"`
}

type executionCase struct {
	SourceName string
	ID         string
	Variant    string
	Metadata   metadata
	LoadError  string
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
