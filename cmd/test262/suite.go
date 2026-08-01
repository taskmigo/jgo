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

func executeSuite(config runnerConfig) (report, suite, *baselineSnapshot, string, error) {
	selection, err := readManifest(config.selection)
	if err != nil {
		return report{}, suite{}, nil, "", err
	}
	effectiveDate, err := coverageReportDate(selection.ReportDate, config.reportDate)
	if err != nil {
		return report{}, suite{}, nil, "", err
	}
	testNames, mode, err := selectedTests(config, selection)
	if err != nil {
		return report{}, suite{}, nil, "", err
	}

	results := report{Commit: selection.Commit, ECMAVersion: selection.ECMAVersion, Mode: mode, ByFeature: map[string]counts{}}
	junit := suite{}
	for _, name := range testNames {
		addResult(&results, &junit, runOne(config.root, name, config.maxSteps, config.timeout))
	}
	finalizeReport(&results, &junit)
	previous := applyBaseline(config, &results)
	return results, junit, previous, effectiveDate, nil
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
	if manifest.Commit == "" || manifest.ECMAVersion == "" || manifest.ReportDate == "" {
		return errors.New("invalid selection manifest: missing commit, ecmaVersion, or reportDate")
	}
	if _, err := time.Parse("2006-01-02", manifest.ReportDate); err != nil {
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
