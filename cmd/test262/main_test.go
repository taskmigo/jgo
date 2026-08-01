package main

import (
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
