package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func executeSuite(config runnerConfig) (report, suite, []timedResult, *baselineSnapshot, string, error) {
	selection, err := readManifest(config.selection)
	if err != nil {
		return report{}, suite{}, nil, nil, "", err
	}
	capabilities, err := readCapabilityManifest(config.selection, selection)
	if err != nil {
		return report{}, suite{}, nil, nil, "", err
	}
	effectiveDate, err := coverageReportDate(selection.ReportDate, config.reportDate)
	if err != nil {
		return report{}, suite{}, nil, nil, "", err
	}
	testNames, mode, err := selectedTests(config, selection)
	if err != nil {
		return report{}, suite{}, nil, nil, "", err
	}

	testCases, bodies, err := expandTestCases(config.root, testNames)
	if err != nil {
		return report{}, suite{}, nil, nil, "", err
	}
	if config.feature != "" {
		testCases, testNames = filterCasesByFeature(testCases, config.feature)
		if len(testCases) == 0 {
			return report{}, suite{}, nil, nil, "", fmt.Errorf("no tests tagged with feature %q", config.feature)
		}
	}
	results := report{
		SchemaVersion: baselineSchemaVersion, ECMA262Commit: selection.ECMA262Commit,
		Test262Commit: selection.Commit, ECMAVersion: selection.ECMAVersion,
		CapabilityVersion: selection.CapabilityVersion, Mode: mode,
		SourceFileDenominator: len(testNames), ExpandedCaseDenominator: len(testCases),
		ByFeature: map[string]counts{},
	}
	junit := suite{}
	timings := make([]timedResult, 0, len(testCases))
	for _, testCase := range testCases {
		started := time.Now()
		addResult(&results, &junit, runCase(config.root, testCase, bodies[testCase.SourceName], capabilities, config.maxSteps, config.timeout))
		timings = append(timings, timedResult{CaseID: testCase.ID, DurationMS: time.Since(started).Milliseconds()})
	}
	finalizeReport(&results, &junit)
	previous := applyBaseline(config, &results)
	return results, junit, timings, previous, effectiveDate, nil
}

func filterCasesByFeature(cases []executionCase, feature string) ([]executionCase, []string) {
	filtered := make([]executionCase, 0)
	sources := make([]string, 0)
	lastSource := ""
	for _, testCase := range cases {
		if !contains(testCase.Metadata.features, feature) {
			continue
		}
		filtered = append(filtered, testCase)
		if testCase.SourceName != lastSource {
			sources = append(sources, testCase.SourceName)
			lastSource = testCase.SourceName
		}
	}
	return filtered, sources
}

func expandTestCases(root string, names []string) ([]executionCase, map[string]string, error) {
	testCases := make([]executionCase, 0, len(names)*2)
	bodies := make(map[string]string, len(names))
	for _, name := range names {
		metadata, body, err := loadTestFile(root, name)
		if err != nil {
			testCases = append(testCases, executionCase{
				SourceName: name,
				ID:         name + "#raw",
				Variant:    "raw",
				LoadError:  err.Error(),
			})
			continue
		}
		bodies[name] = body
		for _, variant := range executionVariants(metadata) {
			testCases = append(testCases, executionCase{
				SourceName: name,
				ID:         name + "#" + variant,
				Variant:    variant,
				Metadata:   metadata,
			})
		}
	}
	return testCases, bodies, nil
}

func executionVariants(metadata metadata) []string {
	switch {
	case contains(metadata.flags, "raw"):
		return []string{"raw"}
	case contains(metadata.flags, "module"):
		return []string{"module"}
	case contains(metadata.flags, "onlyStrict"):
		return []string{"strict"}
	case contains(metadata.flags, "noStrict"):
		return []string{"sloppy"}
	default:
		return []string{"sloppy", "strict"}
	}
}

func readManifest(path string) (manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	var selection manifest
	if err := json.Unmarshal(data, &selection); err != nil {
		return manifest{}, err
	}
	if err := validateManifest(selection); err != nil {
		return manifest{}, err
	}
	return selection, nil
}

func selectedTests(config runnerConfig, selection manifest) ([]string, string, error) {
	if config.all {
		names, err := discover(filepath.Join(config.root, "test"))
		if err != nil {
			return nil, "", err
		}
		if len(names) == 0 {
			return nil, "", errors.New("no tests selected")
		}
		return names, "full", nil
	}
	if len(selection.Tests) == 0 {
		return nil, "", errors.New("no tests selected")
	}
	return selection.Tests, "selection", nil
}

func addResult(report *report, junit *suite, result result) {
	report.Results = append(report.Results, result)
	add(&report.Counts, result.Status)
	features := result.Features
	if len(features) == 0 {
		features = []string{inferredFeature(result.Name)}
	}
	for _, feature := range features {
		featureCounts := report.ByFeature[feature]
		add(&featureCounts, result.Status)
		report.ByFeature[feature] = featureCounts
	}

	testCase := testcase{Name: result.Name}
	if result.Status == statusFail || result.Status == statusTimeout {
		testCase.Failure = &message{result.Reason}
	} else if result.Status != statusPass {
		testCase.Skipped = &message{result.Reason}
	}
	junit.Cases = append(junit.Cases, testCase)
}

func finalizeReport(report *report, junit *suite) {
	junit.Tests = report.Counts.Total
	junit.Failures = report.Counts.Fail + report.Counts.Timeout
	junit.Skipped = report.Counts.Skip + report.Counts.Unsupported
	report.Coverage = calculateCoverage(report.Counts)
}

func validateManifest(manifest manifest) error {
	if manifest.Commit == "" || manifest.ECMA262Commit == "" || manifest.ECMAVersion == "" || manifest.ReportDate == "" || manifest.CapabilityVersion == "" {
		return errors.New("invalid selection manifest: missing commit, ecma262Commit, ecmaVersion, reportDate, or capabilityVersion")
	}
	if !isCommitSHA(manifest.Commit) || !isCommitSHA(manifest.ECMA262Commit) {
		return errors.New("invalid selection manifest: commits must be 40-character hexadecimal SHAs")
	}
	if _, err := time.Parse("2006-01-02", manifest.ReportDate); err != nil {
		return fmt.Errorf("invalid selection manifest reportDate: %w", err)
	}
	return nil
}

func isCommitSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
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
	if base, _, found := strings.Cut(name, "#"); found {
		name = base
	}
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

func discover(root string) ([]string, error) {
	var names []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".js" {
			return nil
		}
		relativePath, err := filepath.Rel(filepath.Dir(root), path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(relativePath))
		return nil
	})
	sort.Strings(names)
	return names, err
}
