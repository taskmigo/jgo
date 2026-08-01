package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	report.Regressions = compareBaselineSnapshot(expected, *report)
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
	return expected, nil
}

func compareBaselineSnapshot(expected baselineSnapshot, actual report) []string {
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
	if actual.Counts.Skip > expected.Counts.Skip {
		problems = append(problems, fmt.Sprintf("skip count increased: expected at most %d, got %d", expected.Counts.Skip, actual.Counts.Skip))
	}
	if actual.Counts.Unsupported > expected.Counts.Unsupported {
		problems = append(problems, fmt.Sprintf("unsupported count increased: expected at most %d, got %d", expected.Counts.Unsupported, actual.Counts.Unsupported))
	}
	if actual.Counts.Timeout > expected.Counts.Timeout {
		problems = append(problems, fmt.Sprintf("timeout count increased: expected at most %d, got %d", expected.Counts.Timeout, actual.Counts.Timeout))
	}
	for feature, expectedCounts := range expected.ByFeature {
		actualCounts, found := actual.ByFeature[feature]
		if !found {
			problems = append(problems, "feature disappeared: "+feature)
			continue
		}
		if featureRegressed(expectedCounts, actualCounts) {
			problems = append(problems, fmt.Sprintf(
				"feature regressed: %s (want pass>=%d fail<=%d skip<=%d unsupported<=%d timeout<=%d total=%d; got %+v)",
				feature, expectedCounts.Pass, expectedCounts.Fail, expectedCounts.Skip,
				expectedCounts.Unsupported, expectedCounts.Timeout, expectedCounts.Total, actualCounts,
			))
		}
	}
	return problems
}

func featureRegressed(expected, actual counts) bool {
	return actual.Total != expected.Total ||
		actual.Pass < expected.Pass ||
		actual.Fail > expected.Fail ||
		actual.Skip > expected.Skip ||
		actual.Unsupported > expected.Unsupported ||
		actual.Timeout > expected.Timeout
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
	return baselineSnapshot{
		Commit: report.Commit, ECMAVersion: report.ECMAVersion, Mode: report.Mode,
		Counts: report.Counts, ByFeature: report.ByFeature, Coverage: report.Coverage,
	}
}
