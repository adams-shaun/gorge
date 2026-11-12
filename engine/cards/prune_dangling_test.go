package cards

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPruneCachesDanglingSymlinks proves PruneCaches never panics when some
// ir-*.gob.gz names are dangling symlinks: a seat worktree symlinks the main
// checkout's caches, which the main checkout prunes away, so the glob can
// match more than maxSiblingCaches names while fewer than maxSiblingCaches
// stat. pruneCaches used to slice entries[maxSiblingCaches:] on the stat'able
// count alone and panicked with "slice bounds out of range [4:3]".
func TestPruneCachesDanglingSymlinks(t *testing.T) {
	dir := t.TempDir()

	// Live caches, oldest first; keepPath is the oldest live one.
	live := []string{"0001", "0002", "0003"}
	for i, fp := range live {
		p := cachePathFor(dir, fp)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := time.Unix(int64(1_700_000_000+i), 0)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	// Two dangling symlinks with ir-*.gob.gz-shaped names: they match the
	// glob but stat nothing.
	for _, fp := range []string{"dead1", "dead2"} {
		if err := os.Symlink(filepath.Join(dir, "gone-"+fp), cachePathFor(dir, fp)); err != nil {
			t.Fatal(err)
		}
	}

	// Precondition: the glob matches more than maxSiblingCaches names while
	// fewer than maxSiblingCaches can be stat'ed.
	paths, err := filepath.Glob(filepath.Join(dir, "ir-*.gob.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) <= maxSiblingCaches {
		t.Fatalf("precondition: glob matched %d names, want > %d", len(paths), maxSiblingCaches)
	}
	statable := 0
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			statable++
		}
	}
	if statable >= maxSiblingCaches {
		t.Fatalf("precondition: %d paths stat, want < %d", statable, maxSiblingCaches)
	}

	// 5 names (> maxSiblingCaches), only 3 stat-able: used to panic with
	// "slice bounds out of range [4:3]".
	keepPath := cachePathFor(dir, live[0])
	PruneCaches(dir, keepPath) // must return

	for _, fp := range live {
		if _, err := os.Stat(cachePathFor(dir, fp)); err != nil {
			t.Fatalf("live cache %s pruned or missing: %v", fp, err)
		}
	}
}

// TestPruneCachesDanglingSymlinksPrunesToCap is the healthy-path sibling:
// enough live caches that pruning genuinely runs, plus dangling links on top.
// It asserts the actual cap, not just survival.
func TestPruneCachesDanglingSymlinksPrunesToCap(t *testing.T) {
	dir := t.TempDir()

	live := []string{"0001", "0002", "0003", "0004", "0005", "0006"}
	for i, fp := range live {
		p := cachePathFor(dir, fp)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := time.Unix(int64(1_700_000_000+i), 0)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	for _, fp := range []string{"dead1", "dead2"} {
		if err := os.Symlink(filepath.Join(dir, "gone-"+fp), cachePathFor(dir, fp)); err != nil {
			t.Fatal(err)
		}
	}

	paths, err := filepath.Glob(filepath.Join(dir, "ir-*.gob.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) <= maxSiblingCaches {
		t.Fatalf("precondition: glob matched %d names, want > %d", len(paths), maxSiblingCaches)
	}

	// keepPath is the OLDEST live cache: the sort must still protect it.
	keepPath := cachePathFor(dir, live[0])
	PruneCaches(dir, keepPath)

	// Count via os.Stat, not the glob, which the dangling links also match.
	remaining := 0
	for _, fp := range live {
		if _, err := os.Stat(cachePathFor(dir, fp)); err == nil {
			remaining++
		}
	}
	if remaining != maxSiblingCaches {
		t.Fatalf("%d live caches remain, want exactly %d", remaining, maxSiblingCaches)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("keepPath (oldest live cache) was pruned: %v", err)
	}
}
