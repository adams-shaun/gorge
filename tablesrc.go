// Package gorge holds the sources that decide a binary card table's bytes, so
// that a binary knows which tables match it without a source tree: a pinned
// tool, a gate's base tree and a deployed server all outlive their checkout, and
// walking up to hash dirac/kgen at run time would load whatever the cwd happens
// to be in. It sits at the module root because that is the only directory above
// both dirac/ and engine/. It imports the standard library only; only dirac/cardset
// and dirac/kgen import it, so a kgen edit does not recompile dirac.
package gorge

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// dirac.NonCreatureSubtype is the only dirac function kgen calls, and hashing all
// of dirac/ would re-key every dirac change, so it is left out on purpose. The
// engine patterns are the engine packages kgen builds against.
//
//go:embed dirac/kgen/*.go dirac/kop/*.go dirac/cardset/*.go dirac/cardset/pools dirac/cardset/standard.txt dirac/cardset/draftpools.txt dirac/cardset/setlists/*.txt dirac/support
//go:embed engine/cards/*.go engine/decision/*.go engine/effects/*.go engine/effects/params/*.go engine/events/*.go engine/rules/cost/*.go engine/rules/scriptfacts/*.go engine/state/*.go
var tableSources embed.FS

// TableSources is the embedded source tree, slash paths from the module root.
// kgen reads its pool lists from here, not from disk, so the tables it publishes
// under this binary's key are a function of this binary and the corpus alone.
func TableSources() fs.FS { return tableSources }

var sourceHash = sync.OnceValue(func() string {
	h, err := hashSources(tableSources)
	if err != nil {
		panic("gorge: embedded table sources: " + err.Error()) // an embed.FS cannot fail to walk
	}
	return h
})

// TableSourceHash is the hex sha256 of the embedded table sources (hashSources).
// Memoised.
func TableSourceHash() string { return sourceHash() }

// hashSources is sha256 over, for each file sorted by slash path and skipping
// names ending _test.go: path, NUL, decimal length, NUL, bytes. Test files do not
// change what a table holds, so editing one does not re-key.
func hashSources(fsys fs.FS) (string, error) {
	var paths []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !strings.HasSuffix(p, "_test.go") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	slices.Sort(paths)
	h := sha256.New()
	for _, p := range paths {
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return "", err
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(len(b))))
		h.Write([]byte{0})
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
