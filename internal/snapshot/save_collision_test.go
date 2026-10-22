package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

// TestSaveSurfacesFilesystemError verifies Save reports a filesystem failure
// (here: an unwritable snapshot directory) rather than swallowing it. This also
// exercises the non-collision branch of the O_EXCL open error, so the
// fail-closed write path is fully covered, not just the duplicate-ID case.
func TestSaveSurfacesFilesystemError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory write permissions are not enforced the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	base := t.TempDir()
	mgr := NewManager(base)

	// Seed one snapshot so the snapshot directory exists.
	if err := mgr.Save(&Snapshot{
		ID:        "snapshot-a",
		Timestamp: time.Now().UTC(),
		Model:     &model.BausteinsichtModel{},
	}); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	// Make the snapshot directory read-only so creating a new file fails with a
	// permission error (not os.ErrExist).
	snapDir := filepath.Join(base, snapshotDir)
	if err := os.Chmod(snapDir, 0o500); err != nil {
		t.Fatalf("chmod snapshot dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(snapDir, 0o700) }) // restore so t.TempDir can clean up

	err := mgr.Save(&Snapshot{
		ID:        "snapshot-b",
		Timestamp: time.Now().UTC(),
		Model:     &model.BausteinsichtModel{},
	})
	if err == nil {
		t.Fatal("expected Save to fail when the snapshot directory is not writable")
	}
	if errors.Is(err, os.ErrExist) {
		t.Fatalf("error should report a filesystem failure, not a collision: %v", err)
	}
}

// TestSaveCleansUpAfterPostCreateFailure verifies that when Save fails after it
// has already created the snapshot file (here: the index write fails), it
// removes that file again. Otherwise the orphaned file would make O_EXCL reject
// every later retry of the same ID, so the snapshot could never be saved.
func TestSaveCleansUpAfterPostCreateFailure(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base)

	// Make the index path a directory so updateIndex's file write fails *after*
	// the snapshot file has been created. This is cross-platform (writing a file
	// over a directory fails everywhere) and needs no permission tricks.
	indexAsDir := filepath.Join(base, snapshotDir, "index.json")
	if err := os.MkdirAll(indexAsDir, 0o755); err != nil {
		t.Fatalf("seeding index-as-directory: %v", err)
	}

	snap := &Snapshot{
		ID:        "snapshot-cleanup",
		Timestamp: time.Now().UTC(),
		Model:     &model.BausteinsichtModel{},
	}
	if err := mgr.Save(snap); err == nil {
		t.Fatal("expected Save to fail when the index cannot be written")
	}

	// The snapshot file this call created must be gone, so O_EXCL does not block
	// a retry on the same ID.
	if mgr.Exists(snap.ID) {
		t.Errorf("snapshot file %q was left behind after a failed Save", snap.ID)
	}
}
