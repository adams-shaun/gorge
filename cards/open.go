package cards

import (
	"os"
	"path/filepath"
)

// OpenCorpus loads dir's compiled IR cache (dir/ir.gob.gz), or compiles
// dir/cardsfolder afresh when the cache is absent, unreadable, or older
// than dir/cards.lock — a fetch since the last compile invalidates it, the
// same staleness rule forgec's own fetch-then-compile pipeline assumes. It
// returns a plain error, never a panic, when neither cache nor corpus is
// present, so a clean checkout with nothing fetched is the caller's
// decision (tests Skip; a server refuses to start).
func OpenCorpus(dir string) (*Registry, error) {
	cache := filepath.Join(dir, "ir.gob.gz")
	cacheInfo, cacheErr := os.Stat(cache)
	lockInfo, lockErr := os.Stat(filepath.Join(dir, "cards.lock"))
	stale := cacheErr != nil || (lockErr == nil && lockInfo.ModTime().After(cacheInfo.ModTime()))
	if !stale {
		if r, err := LoadRegistry(cache); err == nil {
			return r, nil
		}
	}
	r, _, err := CompileDir(CorpusDir(dir))
	if err != nil {
		return nil, err
	}
	return r, nil
}
