package importer_test

import (
	"testing"

	"github.com/docToolchain/Bausteinsicht/internal/importer"
	"github.com/docToolchain/Bausteinsicht/internal/model"
)

// TestImportResult_ZeroValue verifies that the zero value of ImportResult is
// usable: a nil model and no warnings.
func TestImportResult_ZeroValue(t *testing.T) {
	var r importer.ImportResult

	if r.Model != nil {
		t.Errorf("expected nil Model, got %v", r.Model)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(r.Warnings))
	}
}

// TestImportResult_Populated verifies that ImportResult correctly carries a
// model together with a set of non-fatal warnings.
func TestImportResult_Populated(t *testing.T) {
	m := &model.BausteinsichtModel{
		Model: map[string]model.Element{
			"system": {Kind: "system", Title: "System"},
		},
	}
	warnings := []string{"unresolved relationship", "unknown kind"}

	r := importer.ImportResult{
		Model:    m,
		Warnings: warnings,
	}

	if r.Model != m {
		t.Errorf("expected Model to be the same pointer")
	}
	if got, ok := r.Model.Model["system"]; !ok || got.Title != "System" {
		t.Errorf("expected element 'system' with title 'System', got %+v", got)
	}
	if len(r.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(r.Warnings))
	}
	if r.Warnings[0] != "unresolved relationship" {
		t.Errorf("unexpected first warning: %q", r.Warnings[0])
	}
}
