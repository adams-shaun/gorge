package cards

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The card-script census sidecar.
//
// The staleness rule (corpusInputNewerThan) compares mtimes, and mtimes
// compare one directory's timeline: a cache compiled by a process whose
// corpus view held FEWER or DIFFERENT card scripts (a seat worktree's
// mirror, a partial checkout) is newer than every reader's cardsfolder and
// letter-folder mtimes, so the mtime rule calls it fresh and every reader is
// served the other view's card set.
//
// The census closes the added/removed/renamed-script shape of that blind
// spot. Save records, beside the cache, a digest of the relative path of
// every card script under the corpus it was saved for, and cacheFresh
// refuses a cache whose recorded census disagrees with the reader's
// cardsfolder.
//
// Deliberately names only -- no content, size or mtime. Measured 2026-11-13
// on the shared corpus (34,444 scripts, warm, a loaded 8-core box): names
// ~28 ms per open; names plus one lstat per script (size) ~194 ms; content
// would read every file. mtime also differs between byte-identical views (a
// mirror copied without preserving times), so it would turn such opens into
// recompiles without catching a missing script any better. An edit to an
// existing script is not caught here; that stays the mtime rule's job.
//
// A cache with NO census sidecar (written before the census existed, or by a
// Save whose sidecar write failed) falls back to the mtime rule alone. The
// shared .cards is read-only to seat sandboxes, so refusing every legacy
// cache there would recompile the corpus in memory on every process start
// until a writable process happened to rewrite it.

// cardCensusPath is the census sidecar that sits beside the gob cache at
// cache. Like SegmentPath it is a derived name; PruneCaches drops it with its
// cache.
func cardCensusPath(cache string) string {
	return strings.TrimSuffix(cache, ".gob.gz") + ".cen"
}

// cardCensus digests the card script names under folder in the corpus
// layout compileScripts compiles: the .txt files directly in folder and in
// each immediate (letter) subfolder, sorted, hashed as (relative path, 0).
// One ReadDir of folder plus one name-only read per subfolder: no per-script
// stat. An absent folder is errCensusNoCorpus, so the caller can tell a
// cache-only directory from an unreadable corpus.
func cardCensus(folder string) (string, error) {
	top, err := os.ReadDir(folder)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", errCensusNoCorpus
		}
		return "", err
	}
	var rels []string
	for _, e := range top {
		if !e.IsDir() {
			if strings.HasSuffix(e.Name(), ".txt") {
				rels = append(rels, e.Name())
			}
			continue
		}
		names, err := dirNames(filepath.Join(folder, e.Name()))
		if err != nil {
			return "", err
		}
		for _, n := range names {
			if strings.HasSuffix(n, ".txt") {
				rels = append(rels, e.Name()+"/"+n)
			}
		}
	}
	sort.Strings(rels)
	h := sha256.New()
	for _, r := range rels {
		h.Write([]byte(r))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

// dirNames lists dir's entry names without a per-entry stat.
func dirNames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

// errCensusNoCorpus is cardCensus's error for a corpus directory with no
// cardsfolder (a cache-only fixture).
var errCensusNoCorpus = errors.New("cards: no cardsfolder to census")

// noCorpusCensus is the census recorded for a cache saved where there is no
// cardsfolder: it matches no real census, so the cache is refused the moment
// a cardsfolder appears with any script in it that the mtime rule misses.
const noCorpusCensus = "none"

// cardCensusFresh reports whether cache's census sidecar agrees with the card
// scripts dir holds now. It is part of cacheFresh, the one home of the
// staleness rule. A missing sidecar or a dir with no cardsfolder defers to
// the mtime rule (true); an unreadable cardsfolder is stale (false).
func cardCensusFresh(dir, cache string) bool {
	want, err := os.ReadFile(cardCensusPath(cache))
	if err != nil {
		return true
	}
	got, err := cardCensus(CorpusDir(dir))
	if errors.Is(err, errCensusNoCorpus) {
		return true
	}
	if err != nil {
		return false
	}
	return string(want) == got
}

// saveCardCensus records beside cache the census of census, a digest
// cardCensus took of the corpus the cache is saved for. It is renamed into
// place atomically, like saveGob.
func saveCardCensus(cache, census string) error {
	path := cardCensusPath(cache)
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	fail := func(err error) error {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if _, err := f.WriteString(census); err != nil {
		return fail(err)
	}
	if err := f.Chmod(0o644); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// censusForSave is the census Save records for the cache at path: the corpus
// beside it, or noCorpusCensus when there is none.
func censusForSave(path string) (string, error) {
	c, err := cardCensus(CorpusDir(filepath.Dir(path)))
	if errors.Is(err, errCensusNoCorpus) {
		return noCorpusCensus, nil
	}
	return c, err
}
