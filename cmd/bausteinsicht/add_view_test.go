package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestAddView_NewViewPreservesComments guards the comment-preserving save
// (#122): adding a NEW view must not re-serialise the model and drop comments.
func TestAddView_NewViewPreservesComments(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "architecture.jsonc")
	content := `{
  // PREAMBLE-MARKER: keep me
  "specification": { "elements": { "system": {"notation": "box"} }, "relationships": {} },
  "model": { "a": { "kind": "system", "title": "A" } },
  "relationships": [],
  "views": {
    // VIEW-MARKER: keep me
    "ctx": { "title": "Context", "include": ["a"] }
  }
}`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "view", "overview", "--model", p, "--title", "Overview", "--include", "a"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("add view: %v", err)
	}

	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, marker := range []string{"PREAMBLE-MARKER: keep me", "VIEW-MARKER: keep me"} {
		if !strings.Contains(s, marker) {
			t.Errorf("comment %q was dropped by add view:\n%s", marker, s)
		}
	}
	if !strings.Contains(s, `"overview"`) {
		t.Errorf("expected new view %q to be inserted, got:\n%s", "overview", s)
	}
}

func TestAddViewCmd_MissingTitleForNewView(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeViewTestModel(t, dir)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "view", "newview",
		"--model", modelPath,
	})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for missing --title on new view")
	}
}

func TestAddViewCmd_WithTitle(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeViewTestModel(t, dir)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "view", "myview",
		"--model", modelPath,
		"--title", "My View",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddViewCmd_MissingViewKey(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(newAddCmd())

	cmd.SetArgs([]string{"add", "view"})
	err := cmd.Execute()

	if err == nil {
		t.Error("expected error for missing view key, got nil")
	}
}

func TestAddViewCmd_MergeViewFields(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeViewTestModel(t, dir)

	// Create first view
	cmd1 := NewRootCmd()
	cmd1.SetArgs([]string{"add", "view", "containers",
		"--model", modelPath,
		"--title", "System Containers",
		"--include", "webshop",
	})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("unexpected error on first add: %v", err)
	}

	// Merge: update title
	cmd2 := NewRootCmd()
	cmd2.SetArgs([]string{"add", "view", "containers",
		"--model", modelPath,
		"--title", "Updated Containers",
	})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("unexpected error on merge title: %v", err)
	}

	// Merge: add to include list
	cmd3 := NewRootCmd()
	cmd3.SetArgs([]string{"add", "view", "containers",
		"--model", modelPath,
		"--include", "webshop",
		"--include", "payments",
	})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("unexpected error on merge include: %v", err)
	}
}

// TestAddViewCmd_InvalidKeys covers the key rejection branch of "add view" at
// the command level. Shares invalidKeys with the specification tests so all
// three identifier commands are held to the same contract (#582).
func TestAddViewCmd_InvalidKeys(t *testing.T) {
	for _, key := range invalidKeys {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			modelPath := writeViewTestModel(t, dir)

			cmd := NewRootCmd()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"add", "view", key,
				"--model", modelPath,
				"--title", "Test",
			})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for invalid view key %q", key)
			}
			if e, ok := err.(*exitError); ok && e.code != 1 {
				t.Errorf("expected exit code 1, got %d", e.code)
			}
		})
	}
}

// TestAddViewCmd_CamelCaseKey is the "add view" half of the #582 regression:
// the view validator was lowercase-only.
func TestAddViewCmd_CamelCaseKey(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeViewTestModel(t, dir)

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"add", "view", "systemLandscape",
		"--model", modelPath,
		"--title", "System Landscape",
	})
	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error for camelCase view key: %v", err)
	}
}

func writeViewTestModel(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "architecture.jsonc")
	content := `{
  "specification": {
    "elements": {
      "system": {"notation": "box", "container": true}
    }
  },
  "model": {
    "webshop": {
      "kind": "system",
      "title": "Webshop"
    },
    "payments": {
      "kind": "system",
      "title": "Payment Service"
    }
  },
  "relationships": [],
  "views": {}
}`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}
