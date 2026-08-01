package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

type runnerConfig struct {
	root            string
	selection       string
	all             bool
	jsonOutput      string
	junitOutput     string
	summaryOutput   string
	coverageOutput  string
	timingOutput    string
	reportDate      string
	maxSteps        uint64
	timeout         time.Duration
	baseline        string
	refreshBaseline bool
	allowV1Reset    bool
	feature         string
}

const defaultMaxSteps uint64 = 200_000

func runCLI(args []string, stdout, stderr io.Writer) int {
	config, err := parseRunnerConfig(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if config.refreshBaseline && config.baseline == "" {
		return reportOperationalError(stderr, errors.New("-refresh-baseline requires -baseline"))
	}

	report, junit, timings, previous, effectiveDate, err := executeSuite(config)
	if err != nil {
		return reportOperationalError(stderr, err)
	}
	if err := writeRunnerOutputs(config, report, junit, timings, previous, effectiveDate, stdout); err != nil {
		return reportOperationalError(stderr, err)
	}
	if (config.baseline == "" && report.Counts.Fail+report.Counts.Timeout > 0) || len(report.Regressions) > 0 {
		return 1
	}
	return 0
}

func parseRunnerConfig(args []string, stderr io.Writer) (runnerConfig, error) {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	config := runnerConfig{}
	flags.StringVar(&config.root, "test262", "test262", "Test262 checkout or smoke fixture root")
	flags.StringVar(&config.selection, "selection", "test262/selection.json", "selection manifest")
	flags.BoolVar(&config.all, "all", false, "run every .js file below <test262>/test")
	flags.StringVar(&config.jsonOutput, "json", "test262-report.json", "JSON report")
	flags.StringVar(&config.junitOutput, "junit", "test262-report.xml", "JUnit report")
	flags.StringVar(&config.summaryOutput, "summary", "", "summary output (defaults to GITHUB_STEP_SUMMARY)")
	flags.StringVar(&config.coverageOutput, "coverage", "", "write the canonical coverage report")
	flags.StringVar(&config.timingOutput, "timings", "", "write non-canonical per-case timing diagnostics")
	flags.StringVar(&config.reportDate, "report-date", "", "override the manifest report date (YYYY-MM-DD)")
	flags.Uint64Var(&config.maxSteps, "steps", defaultMaxSteps, "steps per test")
	flags.DurationVar(&config.timeout, "timeout", 2*time.Second, "timeout per test")
	flags.StringVar(&config.baseline, "baseline", "", "full-run baseline used to reject coverage regressions")
	flags.BoolVar(&config.refreshBaseline, "refresh-baseline", false, "refresh -baseline after checking it for regressions")
	flags.BoolVar(&config.allowV1Reset, "allow-v1-reset", false, "explicitly permit the one-time schema v1 to v2 baseline reset")
	flags.StringVar(&config.feature, "feature", "", "run only cases tagged with an exact Test262 feature")
	if err := flags.Parse(args); err != nil {
		return runnerConfig{}, err
	}
	return config, nil
}

func writeRunnerOutputs(config runnerConfig, report report, junit suite, timings []timedResult, previous *baselineSnapshot, reportDate string, stdout io.Writer) error {
	if err := writeJSON(config.jsonOutput, report); err != nil {
		return err
	}
	xmlReport, err := xml.MarshalIndent(junit, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(config.junitOutput, append([]byte(xml.Header), xmlReport...), 0644); err != nil {
		return err
	}
	if config.timingOutput != "" {
		if err := writeJSON(config.timingOutput, timings); err != nil {
			return err
		}
	}

	summary := renderSummary(report)
	if previous != nil {
		summary = renderChangeSummary(*previous, report)
	}
	if config.coverageOutput != "" {
		if err := os.WriteFile(config.coverageOutput, []byte(renderCoverage(report, reportDate)), 0644); err != nil {
			return err
		}
	}
	destination := config.summaryOutput
	if destination == "" {
		destination = os.Getenv("GITHUB_STEP_SUMMARY")
	}
	if destination != "" {
		return os.WriteFile(destination, []byte(summary), 0644)
	}
	fmt.Fprint(stdout, summary)
	return nil
}

func reportOperationalError(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	return 2
}
