package snapshot

import (
	"testing"
	"time"

	"github.com/docToolchain/Bausteinsicht/internal/model"
)

// TestSaveRejectsDuplicateID proves Save is fail-closed: once a snapshot with a
// given ID is persisted, a second Save that resolves to the same ID must fail
// instead of silently overwriting the first. The in-process counter prevents
// collisions within one process, but two CLI invocations in the same clock tick
// generate the same ID across processes; without the O_EXCL guard the second
// Save overwrites the first and the earlier snapshot is lost (#648).
func TestSaveRejectsDuplicateID(t *testing.T) {
	mgr := NewManager(t.TempDir())

	first := &Snapshot{
		ID:        "snapshot-2026-01-02T15-04-05.000000000Z-000001",
		Timestamp: time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC),
		Message:   "first",
		Model:     &model.BausteinsichtModel{},
	}
	if err := mgr.Save(first); err != nil {
		t.Fatalf("saving first snapshot: %v", err)
	}

	// Same ID, different content: simulates a same-tick collision from another
	// process, which the per-process counter cannot prevent.
	second := &Snapshot{
		ID:        first.ID,
		Timestamp: first.Timestamp,
		Message:   "second",
		Model:     &model.BausteinsichtModel{},
	}
	if err := mgr.Save(second); err == nil {
		t.Fatalf("Save overwrote existing snapshot ID %q instead of failing; the earlier snapshot would be lost", first.ID)
	}

	// The original snapshot must survive the rejected overwrite.
	got, err := mgr.Load(first.ID)
	if err != nil {
		t.Fatalf("loading snapshot after rejected overwrite: %v", err)
	}
	if got.Message != "first" {
		t.Errorf("snapshot was overwritten: message = %q, want %q", got.Message, "first")
	}
}
