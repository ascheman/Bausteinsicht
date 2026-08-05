package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAddSpecificationElementCmd_MissingNotationFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(newAddCmd())

	cmd.SetArgs([]string{"add", "specification", "element", "custom_type"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for missing --notation flag, got nil")
	}
	if err != nil && err.Error() != "required flag(s) \"notation\" not set" {
		t.Logf("got error: %v", err)
	}
}

func TestAddSpecificationRelationshipCmd_MissingNotationFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(newAddCmd())

	cmd.SetArgs([]string{"add", "specification", "relationship", "custom_rel"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for missing --notation flag, got nil")
	}
	if err != nil && err.Error() != "required flag(s) \"notation\" not set" {
		t.Logf("got error: %v", err)
	}
}

func TestAddSpecificationElementCmd_WithNotation(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeSpecTestModel(t, dir)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "specification", "element", "custom_type",
		"--model", modelPath,
		"--notation", "Custom Component",
		"--description", "A custom type",
		"--container",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddSpecificationRelationshipCmd_WithNotation(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeSpecTestModel(t, dir)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "specification", "relationship", "custom_rel",
		"--model", modelPath,
		"--notation", "custom calls",
		"--dashed",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// invalidKeys are rejected by isValidKey for every command that names an
// identifier. Kept in one place so element, specification, and view keys are
// asserted against the same set -- they drifted apart once already (#582).
var invalidKeys = []string{
	"foo.bar",    // dot (hierarchy separator)
	"foo bar",    // space
	"foo@bar",    // special char
	"foo/bar",    // slash
	"",           // empty
	"123invalid", // starts with digit
	"_leading",   // starts with underscore
}

// TestAddSpecificationElementCmd_InvalidKeys covers the key rejection branch of
// "add specification element" at the command level, not just the validator.
func TestAddSpecificationElementCmd_InvalidKeys(t *testing.T) {
	for _, key := range invalidKeys {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			modelPath := writeSpecTestModel(t, dir)

			cmd := NewRootCmd()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"add", "specification", "element", key,
				"--model", modelPath,
				"--notation", "Box",
			})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for invalid specification key %q", key)
			}
			if e, ok := err.(*exitError); ok && e.code != 1 {
				t.Errorf("expected exit code 1, got %d", e.code)
			}
		})
	}
}

// TestAddSpecificationRelationshipCmd_InvalidKeys is the relationship
// counterpart of TestAddSpecificationElementCmd_InvalidKeys.
func TestAddSpecificationRelationshipCmd_InvalidKeys(t *testing.T) {
	for _, key := range invalidKeys {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			modelPath := writeSpecTestModel(t, dir)

			cmd := NewRootCmd()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"add", "specification", "relationship", key,
				"--model", modelPath,
				"--notation", "->",
			})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for invalid specification key %q", key)
			}
			if e, ok := err.(*exitError); ok && e.code != 1 {
				t.Errorf("expected exit code 1, got %d", e.code)
			}
		})
	}
}

// TestAddSpecificationCmd_CamelCaseKeys is the regression test for #582: the
// specification commands used a lowercase-only validator while the model,
// schema, validate, and "add element" all accepted camelCase.
func TestAddSpecificationCmd_CamelCaseKeys(t *testing.T) {
	for _, key := range []string{"boundaryObject", "applicationService"} {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			modelPath := writeSpecTestModel(t, dir)

			cmd := NewRootCmd()
			cmd.SetArgs([]string{"add", "specification", "element", key,
				"--model", modelPath,
				"--notation", "Box",
			})
			if err := cmd.Execute(); err != nil {
				t.Errorf("unexpected error for camelCase key %q: %v", key, err)
			}
		})
	}
}

// TestAddSpecificationElementCmd_AutoDetectFails covers
// loadModelForSpecificationCmd's auto-detect error branch: no --model flag,
// and no .jsonc file in the working directory for AutoDetect to find.
func TestAddSpecificationElementCmd_AutoDetectFails(t *testing.T) {
	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "specification", "element", "custom_type", "--notation", "Box"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no .jsonc file exists for auto-detection")
	}
}

// TestAddSpecificationRelationshipCmd_LoadModelFails covers
// loadModelForSpecificationCmd's model.Load error branch: --model points at
// a file that isn't valid JSONC.
func TestAddSpecificationRelationshipCmd_LoadModelFails(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "architecture.jsonc")
	if err := os.WriteFile(modelPath, []byte("not valid jsonc {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "specification", "relationship", "custom_rel",
		"--model", modelPath,
		"--notation", "->",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error loading a malformed model file")
	}
}

// chdir changes the working directory for the duration of a test and
// returns a func to restore it — used to exercise AutoDetect's cwd-scanning
// behavior without depending on the ambient test-runner working directory.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}
}

// TestAddSpecification_PreservesComments guards the comment-preserving save
// (#122): adding an element/relationship type must NOT re-serialise the whole
// model and drop JSONC comments.
func TestAddSpecification_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "architecture.jsonc")
	content := `{
  // PREAMBLE-MARKER: keep me
  "specification": {
    "elements": {
      // ELEMENT-MARKER: keep me
      "system": {"notation": "box"}
    },
    "relationships": {
      "uses": {"notation": "->"}
    }
  },
  "model": {},
  "relationships": [],
  "views": {}
}`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "specification", "element", "custom_type", "--model", p, "--notation", "Custom"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add element: %v", err)
	}
	cmd = NewRootCmd()
	cmd.SetArgs([]string{"add", "specification", "relationship", "custom_rel", "--model", p, "--notation", "calls", "--dashed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add relationship: %v", err)
	}

	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, marker := range []string{"PREAMBLE-MARKER: keep me", "ELEMENT-MARKER: keep me"} {
		if !strings.Contains(s, marker) {
			t.Errorf("comment %q was dropped by add specification:\n%s", marker, s)
		}
	}
	// The new entries must have been inserted, too.
	for _, key := range []string{`"custom_type"`, `"custom_rel"`} {
		if !strings.Contains(s, key) {
			t.Errorf("expected %s to be inserted, got:\n%s", key, s)
		}
	}
}

func writeSpecTestModel(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "architecture.jsonc")
	content := `{
  "specification": {
    "elements": {
      "system": {"notation": "box"}
    },
    "relationships": {
      "uses": {"notation": "->"}
    }
  },
  "model": {},
  "relationships": [],
  "views": {}
}`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}
