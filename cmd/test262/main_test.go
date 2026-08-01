package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFrontmatter(t *testing.T) {
	m, body, err := frontmatter("// copyright\n/*---\nfeatures:\n - let\nflags: [onlyStrict]\nincludes: [compareArray.js]\nnegative:\n  phase: parse\n  type: SyntaxError\n---*/\nlet = 1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.features, []string{"let"}) || !m.negativeParse || m.negativeType != "SyntaxError" || body != "\nlet = 1" {
		t.Fatalf("%+v %q", m, body)
	}
}

func TestCompareBaselineRejectsCoverageRegression(t *testing.T) {
	want := report{Commit: "abc", Counts: counts{Pass: 2, Total: 2}, ByFeature: map[string]counts{"let": {Pass: 2, Total: 2}}}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	if got := compareBaseline(path, want); len(got) != 0 {
		t.Fatalf("unchanged coverage regressed: %v", got)
	}
	actual := want
	actual.Counts.Pass = 1
	actual.ByFeature = map[string]counts{"let": {Pass: 1, Fail: 1, Total: 2}}
	if got := compareBaseline(path, actual); len(got) == 0 {
		t.Fatal("coverage regression accepted")
	}
}

func TestCalculateCoverage(t *testing.T) {
	got := calculateCoverage(counts{Pass: 75, Fail: 5, Unsupported: 20, Total: 100})
	if got.CoveredTests != 80 || got.CoveragePercent != 80 || got.OverallPassPercent != 75 || got.CoveredPassPercent != 93.75 {
		t.Fatalf("unexpected coverage: %+v", got)
	}
}

func TestDiscover(t *testing.T) {
	root := filepath.Join(t.TempDir(), "test")
	if err := os.MkdirAll(filepath.Join(root, "language"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "language", "x.js"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	names, err := discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"test/language/x.js"}) {
		t.Fatal(names)
	}
}

func TestInferredFeature(t *testing.T) {
	tests := map[string]string{
		"test/built-ins/WeakMap/prototype/get.js": "path:built-ins/WeakMap",
		"test/language/expressions/addition/x.js": "path:language/expressions",
		"test/annexB/built-ins/x.js":              "path:annexB/built-ins",
		"smoke/x.js":                              "path:other",
	}
	for name, want := range tests {
		if got := inferredFeature(name); got != want {
			t.Errorf("inferredFeature(%q)=%q, want %q", name, got, want)
		}
	}
}
