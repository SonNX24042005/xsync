package pathutils

import (
	"reflect"
	"testing"
)

func TestProcessIncludePaths(t *testing.T) {
	raw := []string{
		"# Comment line",
		"  ",
		"\"data/models/\"",
		"'src/main.py'",
		"utils/helpers",
	}

	got := ProcessIncludePaths(raw, "push", "", "/remote/path")
	expected := []string{
		"data/models/",
		"src/main.py",
		"utils/helpers",
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestBuildIncludeFilter(t *testing.T) {
	paths := []string{
		"mlops/datasets/",
		"scripts/run.py",
	}

	rules := BuildIncludeFilter(paths)
	if len(rules) == 0 {
		t.Fatalf("expected non-empty rules")
	}

	// Check if ends with - *
	if rules[len(rules)-1] != "- *" {
		t.Errorf("expected last rule to be '- *', got '%s'", rules[len(rules)-1])
	}

	// Check parent directory and file rules
	hasMlopsParent := false
	hasMlopsDir := false
	hasMlopsRec := false
	hasScriptsParent := false
	hasRunPy := false

	for _, r := range rules {
		if r == "+ mlops/" {
			hasMlopsParent = true
		}
		if r == "+ mlops/datasets/" {
			hasMlopsDir = true
		}
		if r == "+ mlops/datasets/**" {
			hasMlopsRec = true
		}
		if r == "+ scripts/" {
			hasScriptsParent = true
		}
		if r == "+ scripts/run.py" {
			hasRunPy = true
		}
	}

	if !hasMlopsParent || !hasMlopsDir || !hasMlopsRec || !hasScriptsParent || !hasRunPy {
		t.Errorf("rules missing expected entries: %v", rules)
	}
}
