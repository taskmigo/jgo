package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

func applyBaseline(config runnerConfig, report *report) *baselineSnapshot {
	if config.baseline == "" {
		return nil
	}
	expected, err := readBaseline(config.baseline)
	if err != nil {
		report.Regressions = []string{"baseline: " + err.Error()}
		return nil
	}
	if expected.SchemaVersion == 0 {
		if !config.allowV1Reset || !config.refreshBaseline {
			report.Regressions = []string{"baseline: schema v1 requires -refresh-baseline and -allow-v1-reset"}
			return &expected
		}
		if expected.Test262Commit != report.Test262Commit {
			report.Regressions = []string{"baseline: v1 reset requires an unchanged Test262 commit"}
			return &expected
		}
		if expected.Counts.Total != report.SourceFileDenominator {
			report.Regressions = []string{fmt.Sprintf("baseline: v1 reset requires source denominator %d, got %d", expected.Counts.Total, report.SourceFileDenominator)}
			return &expected
		}
		report.Regressions = nil
	} else {
		if config.allowV1Reset {
			report.Regressions = []string{"baseline: -allow-v1-reset is valid only for a schema v1 baseline"}
			return &expected
		}
		report.Regressions = compareBaselineSnapshot(expected, *report)
	}
	if config.refreshBaseline {
		if err := writeCanonicalJSON(config.baseline, snapshot(*report)); err != nil {
			report.Regressions = []string{"baseline: " + err.Error()}
		}
	}
	return &expected
}

func compareBaseline(path string, actual report) ([]string, error) {
	expected, err := readBaseline(path)
	if err != nil {
		return nil, err
	}
	if expected.SchemaVersion != baselineSchemaVersion {
		return nil, errors.New("exact comparison requires a schema v2 baseline")
	}
	return compareBaselineSnapshot(expected, actual), nil
}

func readBaseline(path string) (baselineSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return baselineSnapshot{}, err
	}
	var expected baselineSnapshot
	if err := json.Unmarshal(data, &expected); err != nil {
		return baselineSnapshot{}, err
	}
	if expected.SchemaVersion == 0 {
		var legacy struct {
			Commit      string            `json:"commit"`
			ECMAVersion string            `json:"ecmaVersion"`
			Mode        string            `json:"mode"`
			Counts      counts            `json:"counts"`
			ByFeature   map[string]counts `json:"byFeature"`
			Coverage    coverage          `json:"coverage"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return baselineSnapshot{}, err
		}
		expected.Test262Commit = legacy.Commit
		expected.ECMAVersion = legacy.ECMAVersion
		expected.Mode = legacy.Mode
		expected.Counts = legacy.Counts
		expected.ByFeature = legacy.ByFeature
		expected.Coverage = legacy.Coverage
		return expected, nil
	}
	if expected.SchemaVersion != baselineSchemaVersion {
		return baselineSnapshot{}, fmt.Errorf("unsupported baseline schema: %d", expected.SchemaVersion)
	}
	return expected, nil
}

func compareBaselineSnapshot(expected baselineSnapshot, actual report) []string {
	var problems []string
	if expected.SchemaVersion != baselineSchemaVersion {
		return []string{"baseline schema is not v2"}
	}
	if expected.Test262Commit != actual.Test262Commit {
		problems = append(problems, fmt.Sprintf("Test262 commit changed: expected %s, got %s", expected.Test262Commit, actual.Test262Commit))
	}
	if expected.ECMA262Commit != actual.ECMA262Commit {
		problems = append(problems, fmt.Sprintf("ECMA-262 commit changed: expected %s, got %s", expected.ECMA262Commit, actual.ECMA262Commit))
	}
	if actual.SourceFileDenominator < expected.SourceFileDenominator {
		problems = append(problems, fmt.Sprintf("source-file denominator decreased: expected at least %d, got %d", expected.SourceFileDenominator, actual.SourceFileDenominator))
	}
	if actual.ExpandedCaseDenominator < expected.ExpandedCaseDenominator {
		problems = append(problems, fmt.Sprintf("expanded-case denominator decreased: expected at least %d, got %d", expected.ExpandedCaseDenominator, actual.ExpandedCaseDenominator))
	}

	actualByID := make(map[string]result, len(actual.Results))
	for _, result := range actual.Results {
		actualByID[result.Name] = result
	}
	for _, caseID := range expected.PassingCaseIDs {
		current, found := actualByID[caseID]
		if !found {
			problems = append(problems, "previous pass disappeared: "+caseID)
		} else if current.Status != statusPass {
			problems = append(problems, fmt.Sprintf("previous pass became %s: %s", current.Status, caseID))
		}
	}
	if expected.CapabilityVersion == actual.CapabilityVersion {
		expectedUnsupported := make(map[string]string, len(expected.UnsupportedCases))
		for _, item := range expected.UnsupportedCases {
			expectedUnsupported[item.CaseID] = item.Reason
		}
		for _, current := range actual.Results {
			if current.Status != statusUnsupported {
				continue
			}
			previousReason, existed := expectedUnsupported[current.Name]
			if !existed {
				problems = append(problems, "unsupported case added without capability-manifest change: "+current.Name)
			} else if previousReason != current.UnsupportedReason {
				problems = append(problems, fmt.Sprintf("unsupported reason changed without capability-manifest change: %s (%s -> %s)", current.Name, previousReason, current.UnsupportedReason))
			}
		}
	}
	return problems
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

func snapshot(report report) baselineSnapshot {
	result := baselineSnapshot{
		SchemaVersion: baselineSchemaVersion, ECMA262Commit: report.ECMA262Commit,
		Test262Commit: report.Test262Commit, ECMAVersion: report.ECMAVersion,
		CapabilityVersion: report.CapabilityVersion, Mode: report.Mode,
		SourceFileDenominator:   report.SourceFileDenominator,
		ExpandedCaseDenominator: report.ExpandedCaseDenominator,
		Counts:                  report.Counts, ByFeature: report.ByFeature, Coverage: report.Coverage,
	}
	for _, testResult := range report.Results {
		switch testResult.Status {
		case statusPass:
			result.PassingCaseIDs = append(result.PassingCaseIDs, testResult.Name)
		case statusFail:
			result.FailingCaseIDs = append(result.FailingCaseIDs, testResult.Name)
		case statusTimeout:
			result.TimeoutCaseIDs = append(result.TimeoutCaseIDs, testResult.Name)
		case statusUnsupported:
			result.UnsupportedCases = append(result.UnsupportedCases, unsupportedCase{CaseID: testResult.Name, Reason: testResult.UnsupportedReason})
		}
	}
	sort.Strings(result.PassingCaseIDs)
	sort.Strings(result.FailingCaseIDs)
	sort.Strings(result.TimeoutCaseIDs)
	sort.Slice(result.UnsupportedCases, func(left, right int) bool {
		return result.UnsupportedCases[left].CaseID < result.UnsupportedCases[right].CaseID
	})
	return result
}
