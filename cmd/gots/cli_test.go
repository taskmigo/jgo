package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLISourcePrecedence(t *testing.T) {
	script := filepath.Join(t.TempDir(), "script.js")
	if err := os.WriteFile(script, []byte("2 + 3"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "expression", args: []string{"-e", "1 + 2", script}, stdin: "6 + 1", want: "3\n"},
		{name: "file", args: []string{script}, stdin: "6 + 1", want: "5\n"},
		{name: "stdin", stdin: "6 + 1", want: "7\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runCLI(test.args, strings.NewReader(test.stdin), &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
			}
			if got := stdout.String(); got != test.want {
				t.Fatalf("stdout = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRunCLIErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{name: "invalid flag", args: []string{"-unknown"}, wantCode: 2, wantStderr: "flag provided but not defined"},
		{name: "missing file", args: []string{filepath.Join(t.TempDir(), "missing.js")}, wantCode: 1, wantStderr: "no such file"},
		{name: "runtime error", args: []string{"-e", "missing"}, wantCode: 1, wantStderr: "missing is not defined"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runCLI(test.args, strings.NewReader(""), &stdout, &stderr); code != test.wantCode {
				t.Fatalf("exit code = %d, want %d", code, test.wantCode)
			}
			if !strings.Contains(stderr.String(), test.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), test.wantStderr)
			}
			if stdout.Len() != 0 {
				t.Fatalf("unexpected stdout: %q", stdout.String())
			}
		})
	}
}
