package cards

import (
	"os"
	"path/filepath"
	"strings"
)

// Single-flight for the IR cache. When CompilerFingerprint changes, every
// process that opens the corpus finds no cache under the new name; before
// this lock each one -- 20-45 test binaries per gate run -- compiled the
// whole corpus itself and raced to Save it (a thundering herd: measured
// at ~1.0-1.2 s wall CompileDir plus ~4.6 s Save per process).
// Now the first opener takes an exclusive lock on <cache>.lock, compiles and
// saves; the rest block on the lock and, once it is released, find the fresh
// cache and load it.
//
// The lock is advisory and best effort: when it cannot be taken (a read-only
// corpus directory, a platform without flock) the open proceeds unlocked,
// exactly as before, because Save's temp+rename keeps concurrent writers
// atomic on its own. The lock only removes duplicated work.

// cacheLockPath is the lock file guarding cache's (re)compile.
func cacheLockPath(cache string) string { return cache + ".lock" }

// pruneCacheLocks removes the lock file of every fingerprint whose cache is
// gone, so the per-fingerprint lock files do not accumulate the way the
// caches themselves would without PruneCaches. A lock somebody holds (a
// cold-cache compile still running) is never removed: tryRemoveCacheLock
// only unlinks a lock it can take without waiting, and lockCache re-checks
// after locking that the file it holds is still the one at the path.
func pruneCacheLocks(dir string) {
	locks, _ := filepath.Glob(filepath.Join(dir, "ir-*.gob.gz.lock"))
	for _, l := range locks {
		if _, err := os.Stat(strings.TrimSuffix(l, ".lock")); os.IsNotExist(err) {
			tryRemoveCacheLock(l)
		}
	}
}
