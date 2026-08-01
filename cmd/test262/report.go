package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func add(counts *counts, status string) {
	counts.Total++
	switch status {
	case statusPass:
		counts.Pass++
	case statusFail:
		counts.Fail++
	case statusSkip:
		counts.Skip++
	case statusUnsupported:
		counts.Unsupported++
	case statusTimeout:
		counts.Timeout++
	}
}

func calculateCoverage(counts counts) coverage {
	covered := counts.Pass + counts.Fail + counts.Timeout
	result := coverage{CoveredTests: covered}
	if counts.Total > 0 {
		result.CoveragePercent = 100 * float64(covered) / float64(counts.Total)
		result.OverallPassPercent = 100 * float64(counts.Pass) / float64(counts.Total)
	}
	if covered > 0 {
		result.CoveredPassPercent = 100 * float64(counts.Pass) / float64(covered)
	}
	return result
}

func renderSummary(report report) string {
	counts := report.Counts
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "## Test262 %s (%s)\n\n**ECMA target:** %s<br>\n**Test262 execution coverage:** %.2f%% (%d/%d tests reached execution)<br>\n**Test262 overall pass rate:** %.2f%% (%d/%d)<br>\n**Pass rate among executed tests:** %.2f%% (%d/%d)\n\n| pass | fail | skip | unsupported | timeout | total |\n|---:|---:|---:|---:|---:|---:|\n| %d | %d | %d | %d | %d | %d |\n\n### By feature\n\n| feature | pass | fail | unsupported | timeout | total |\n|---|---:|---:|---:|---:|---:|\n", report.Commit, report.Mode, report.ECMAVersion, report.Coverage.CoveragePercent, report.Coverage.CoveredTests, counts.Total, report.Coverage.OverallPassPercent, counts.Pass, counts.Total, report.Coverage.CoveredPassPercent, counts.Pass, report.Coverage.CoveredTests, counts.Pass, counts.Fail, counts.Skip, counts.Unsupported, counts.Timeout, counts.Total)
	features := make([]string, 0, len(report.ByFeature))
	for feature := range report.ByFeature {
		features = append(features, feature)
	}
	sort.Strings(features)
	for _, feature := range features {
		counts := report.ByFeature[feature]
		fmt.Fprintf(builder, "| %s | %d | %d | %d | %d | %d |\n", feature, counts.Pass, counts.Fail, counts.Unsupported, counts.Timeout, counts.Total)
	}
	return builder.String()
}

type countMetric struct {
	name               string
	lowerIsBetter      bool
	regressionOnChange bool
	value              func(counts) int
}

var summaryCountMetrics = []countMetric{
	{name: "Pass", value: func(c counts) int { return c.Pass }},
	{name: "Fail", lowerIsBetter: true, value: func(c counts) int { return c.Fail }},
	{name: "Skip", lowerIsBetter: true, value: func(c counts) int { return c.Skip }},
	{name: "Timeout", lowerIsBetter: true, value: func(c counts) int { return c.Timeout }},
	{name: "Unsupported", lowerIsBetter: true, value: func(c counts) int { return c.Unsupported }},
	{name: "Total", regressionOnChange: true, value: func(c counts) int { return c.Total }},
}

// renderChangeSummary keeps the GitHub Step Summary focused on differences
// between the generated PR baseline and the baseline loaded from main.
func renderChangeSummary(previous baselineSnapshot, current report) string {
	features := make(map[string]struct{}, len(previous.ByFeature)+len(current.ByFeature))
	for feature := range previous.ByFeature {
		features[feature] = struct{}{}
	}
	for feature := range current.ByFeature {
		features[feature] = struct{}{}
	}
	changedFeatures := make([]string, 0, len(features))
	for feature := range features {
		if previous.ByFeature[feature] != current.ByFeature[feature] {
			changedFeatures = append(changedFeatures, feature)
		}
	}
	sort.Strings(changedFeatures)

	regressions := compareBaselineSnapshot(previous, current)
	changed := previous.Commit != current.Commit || previous.Counts != current.Counts || len(changedFeatures) > 0
	builder := &strings.Builder{}
	fmt.Fprintln(builder, "## Test262 PR baseline diff")
	fmt.Fprintln(builder)
	if len(regressions) > 0 {
		fmt.Fprintln(builder, "> [!CAUTION]")
		fmt.Fprintln(builder, "> **Regression detected compared with main.**")
	} else if !changed {
		fmt.Fprintln(builder, "> [!TIP]")
		fmt.Fprintln(builder, "> **Test262 check passed. No result changes compared with main.**")
	} else {
		fmt.Fprintln(builder, "> [!TIP]")
		fmt.Fprintln(builder, "> **Test262 check passed. No regression compared with main.**")
	}
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "🟢 improvement · 🔴 regression · unchanged values are shown as `0`")
	fmt.Fprintln(builder)
	if previous.Commit != current.Commit {
		fmt.Fprintln(builder, "### Metadata change")
		fmt.Fprintln(builder)
		fmt.Fprintf(builder, "Pinned Test262 commit: `%s` → `%s` 🔴\n\n", previous.Commit, current.Commit)
	}

	type rateMetric struct {
		name          string
		before, after float64
	}
	rates := []rateMetric{
		{name: "Test262 execution coverage", before: previous.Coverage.CoveragePercent, after: current.Coverage.CoveragePercent},
		{name: "Test262 overall pass rate", before: previous.Coverage.OverallPassPercent, after: current.Coverage.OverallPassPercent},
		{name: "Pass rate among executed tests", before: previous.Coverage.CoveredPassPercent, after: current.Coverage.CoveredPassPercent},
	}
	rateRows := &strings.Builder{}
	for _, metric := range rates {
		if metric.before != metric.after {
			fmt.Fprintf(rateRows, "| %s | %.2f%% | %.2f%% | %s |\n", metric.name, metric.before, metric.after, formatFloatChange(metric.after-metric.before, false))
		}
	}
	if rateRows.Len() > 0 {
		fmt.Fprintln(builder, "### Test262 rate changes")
		fmt.Fprintln(builder)
		fmt.Fprintln(builder, "| Metric | main | PR | Diff |")
		fmt.Fprintln(builder, "|---|---:|---:|---:|")
		fmt.Fprint(builder, rateRows.String())
		fmt.Fprintln(builder)
	}

	fmt.Fprintln(builder, "### Overall status diff")
	fmt.Fprintln(builder)
	fmt.Fprintln(builder, "| Status | main | PR | Diff |")
	fmt.Fprintln(builder, "|---|---:|---:|---:|")
	for _, metric := range summaryCountMetrics {
		before, after := metric.value(previous.Counts), metric.value(current.Counts)
		fmt.Fprintf(builder, "| %s | %d | %d | %s |\n", strings.ToLower(metric.name), before, after, formatIntChange(after-before, metric.lowerIsBetter, metric.regressionOnChange))
	}

	if len(changedFeatures) > 0 {
		fmt.Fprintln(builder)
		fmt.Fprintln(builder, "### Changed features")
		fmt.Fprintln(builder)
		fmt.Fprintln(builder, "Only features with changed results are included.")
		fmt.Fprintln(builder)
		fmt.Fprintln(builder, "| Feature | pass | fail | skip | timeout | unsupported | total |")
		fmt.Fprintln(builder, "|---|---:|---:|---:|---:|---:|---:|")
		for _, feature := range changedFeatures {
			before, after := previous.ByFeature[feature], current.ByFeature[feature]
			fmt.Fprintf(builder, "| %s", strings.ReplaceAll(feature, "|", "\\|"))
			for _, metric := range summaryCountMetrics {
				delta := metric.value(after) - metric.value(before)
				fmt.Fprintf(builder, " | %s", formatIntChange(delta, metric.lowerIsBetter, metric.regressionOnChange))
			}
			fmt.Fprintln(builder, " |")
		}
	}
	return builder.String()
}

func formatIntChange(delta int, lowerIsBetter, regressionOnChange bool) string {
	if delta == 0 {
		return "`0`"
	}
	marker := changeMarker(float64(delta), lowerIsBetter, regressionOnChange)
	return fmt.Sprintf("%s %+.0f", marker, float64(delta))
}

func formatFloatChange(delta float64, lowerIsBetter bool) string {
	return fmt.Sprintf("%s %+.2f pp", changeMarker(delta, lowerIsBetter, false), delta)
}

func changeMarker(delta float64, lowerIsBetter, regressionOnChange bool) string {
	improved := delta > 0
	if lowerIsBetter {
		improved = delta < 0
	}
	arrow := "▲"
	if delta < 0 {
		arrow = "▼"
	}
	if improved && !regressionOnChange {
		return "🟢 " + arrow
	}
	return "🔴 " + arrow
}

// renderCoverage produces the repository's canonical report. Its only
// time-varying value is supplied by the caller, so identical results and dates
// produce byte-for-byte identical output on every machine.
func renderCoverage(report report, reportDate string) string {
	builder := &strings.Builder{}
	fmt.Fprintln(builder, "# Test262 coverage report")
	fmt.Fprintf(builder, "\n**Report date:** %s  \n", reportDate)
	fmt.Fprintf(builder, "**Pinned Test262 commit:** `%s`  \n", report.Commit)
	fmt.Fprintf(builder, "**ECMA target:** %s\n\n", report.ECMAVersion)
	fmt.Fprintln(builder, "This report is generated from the complete pinned Test262 suite. **Test262")
	fmt.Fprintln(builder, "execution coverage** is the percentage of tests that reached execution (pass,")
	fmt.Fprintln(builder, "fail, or timeout); unsupported tests are excluded. **Test262 overall pass rate**")
	fmt.Fprintln(builder, "is passes divided by every test in the suite, including unsupported tests. These")
	fmt.Fprintln(builder, "runner metrics do not by themselves claim complete ECMAScript conformance.")
	fmt.Fprintln(builder)
	fmt.Fprint(builder, renderSummary(report))
	return builder.String()
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func writeCanonicalJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var canonical any
	if err := json.Unmarshal(data, &canonical); err != nil {
		return err
	}
	return writeJSON(path, canonical)
}
