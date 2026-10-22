package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
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
	// Assert positively that the permission failure is surfaced: it must not be
	// mistaken for (or masked as) a collision, and the real cause must survive the
	// %w wrapping so callers can inspect it.
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected a permission error to be reported, got: %v", err)
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

// TestSaveSurfacesSnapshotDirError verifies Save reports a failure to create the
// snapshot directory instead of proceeding. Pointing the manager at a path whose
// parent is a regular file makes MkdirAll fail cross-platform.
func TestSaveSurfacesSnapshotDirError(t *testing.T) {
	base := t.TempDir()
	notADir := filepath.Join(base, "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), fileMode0644); err != nil {
		t.Fatalf("seeding file in place of base dir: %v", err)
	}

	mgr := NewManager(notADir) // snapshot dir would be a child of a regular file
	err := mgr.Save(&Snapshot{
		ID:        "snapshot-nodir",
		Timestamp: time.Now().UTC(),
		Model:     &model.BausteinsichtModel{},
	})
	if err == nil {
		t.Fatal("expected Save to fail when the snapshot directory cannot be created")
	}
}

// TestSaveSurfacesIndexLockError verifies the index-lock acquisition is itself
// fail-closed: if the lock file cannot be opened, Save reports the error instead
// of updating the index unsynchronized, and still cleans up the snapshot file it
// created. Making the lock path a directory forces the open to fail
// cross-platform without permission tricks.
func TestSaveSurfacesIndexLockError(t *testing.T) {
	base := t.TempDir()
	mgr := NewManager(base)

	// Pre-create the lock path as a directory so opening it as a file fails.
	lockAsDir := filepath.Join(base, snapshotDir, "index.json.lock")
	if err := os.MkdirAll(lockAsDir, 0o755); err != nil {
		t.Fatalf("seeding lock-as-directory: %v", err)
	}

	snap := &Snapshot{
		ID:        "snapshot-lockfail",
		Timestamp: time.Now().UTC(),
		Model:     &model.BausteinsichtModel{},
	}
	if err := mgr.Save(snap); err == nil {
		t.Fatal("expected Save to fail when the index lock cannot be opened")
	}

	// The snapshot file must not linger after the failed Save.
	if mgr.Exists(snap.ID) {
		t.Errorf("snapshot file %q was left behind after a failed index lock", snap.ID)
	}
}

// TestSaveConcurrentDistinctIDsAllIndexed guards against lost index updates.
// O_EXCL only protects the per-snapshot file; the index is a read-modify-write.
// When several saves with distinct IDs run concurrently they all create their
// files, but without serialization their index updates interleave and the last
// writer drops the others' entries, so List silently loses snapshots. Each
// snapshot file exists on disk, yet it is missing from the index. Run with -race
// to also catch the data race on index.json. The index lock makes every entry
// survive.
func TestSaveConcurrentDistinctIDsAllIndexed(t *testing.T) {
	mgr := NewManager(t.TempDir())

	const n = 24
	var wg sync.WaitGroup
	errs := make(chan error, n)
	start := make(chan struct{})

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release all goroutines together to maximise contention
			snap := &Snapshot{
				ID:        fmt.Sprintf("snapshot-concurrent-%03d", i),
				Timestamp: time.Now().UTC(),
				Message:   fmt.Sprintf("snapshot %d", i),
				Model:     &model.BausteinsichtModel{},
			}
			if err := mgr.Save(snap); err != nil {
				errs <- err
			}
		}(i)
	}

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Save failed: %v", err)
	}

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("listing snapshots: %v", err)
	}
	if len(list) != n {
		t.Errorf("List returned %d snapshots, want %d; concurrent index updates lost entries", len(list), n)
	}
}
