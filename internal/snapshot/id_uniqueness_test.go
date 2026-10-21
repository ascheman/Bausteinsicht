package snapshot

import (
	"strings"
	"sync"
	"testing"
)

// TestGenerateSnapshotIDUniqueConcurrent reproduces the collision bug: when
// many IDs are generated within the same clock tick (common under concurrency,
// and guaranteed on coarse-resolution clocks), a purely time-derived ID repeats
// and Manager.Save silently overwrites the earlier snapshot file.
func TestGenerateSnapshotIDUniqueConcurrent(t *testing.T) {
	const workers = 200

	var (
		mu  sync.Mutex
		set = make(map[string]struct{}, workers)
		wg  sync.WaitGroup
	)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			id := generateSnapshotID()
			mu.Lock()
			set[id] = struct{}{}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(set) != workers {
		t.Fatalf("snapshot IDs collided under concurrency: got %d unique IDs out of %d generated (%d collisions)",
			len(set), workers, workers-len(set))
	}
}

// TestGenerateSnapshotIDUniqueTightLoop reproduces the collision deterministically
// without relying on goroutine scheduling: a tight sequential loop generates many
// IDs faster than a coarse clock can advance, so any time-only scheme repeats.
func TestGenerateSnapshotIDUniqueTightLoop(t *testing.T) {
	const iterations = 100_000

	seen := make(map[string]struct{}, iterations)
	collisions := 0
	for i := 0; i < iterations; i++ {
		id := generateSnapshotID()
		if _, dup := seen[id]; dup {
			collisions++
		}
		seen[id] = struct{}{}
	}

	if collisions != 0 {
		t.Fatalf("snapshot IDs collided in tight loop: %d collisions across %d generations (%d unique)",
			collisions, iterations, len(seen))
	}
}

// TestGenerateSnapshotIDShape guards the contract relied on elsewhere: the ID
// keeps the "snapshot-" prefix and stays longer than the 28-char bare timestamp
// so List's newest-first ordering (which sorts on Timestamp, not ID) is unaffected
// and downstream format assertions still hold.
func TestGenerateSnapshotIDShape(t *testing.T) {
	id := generateSnapshotID()
	if !strings.HasPrefix(id, "snapshot-") {
		t.Errorf("ID %q does not start with %q", id, "snapshot-")
	}
	const bareTimestampLen = 28 // len("snapshot-2006-01-02T15-04-05")
	if len(id) <= bareTimestampLen {
		t.Errorf("ID %q (len %d) is not longer than the bare timestamp (%d)", id, len(id), bareTimestampLen)
	}
}
