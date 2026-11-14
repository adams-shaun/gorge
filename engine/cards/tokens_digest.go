package cards

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// The token-script digest sidecar.
//
// The staleness rule for the card corpus is mtime-based
// (corpusInputNewerThan), and that rule has a blind spot tokenscripts falls
// into: a cache compiled by a process whose corpus view had NO token scripts
// — a seat worktree whose .cards mirror lacked the tokenscripts directory,
// saving its cache into the shared directory through the symlink — is newer
// than every reader's tokenscripts mtime, so the mtime rule serves it forever
// and every reader gets a registry with no Tokens at all. Measured 2026-11-13
// on the shared corpus: one such cache lowered corpus-wide kgen acceptance by
// 2834 cards (every token-dependent lowering refused, `token script
// "c_a_treasure_sac" not in the corpus`), and it was served until someone
// deleted the cache files by hand.
//
// The sidecar closes it. Save records, beside the cache, the digest of the
// token scripts the registry was compiled from, and cacheFresh refuses a
// cache whose sidecar is missing or disagrees with the directory it is opened
// from. A missing sidecar (a cache written by an older build, or a save whose
// sidecar write failed) is staleness, not freshness: the cache's token
// provenance is unknown, and the only safe reading is a recompile, which
// records the digest so every later open takes the cache-hit path again.

// TokenDigestPath is the digest sidecar that sits beside the gob cache at
// cache. Like SegmentPath and ImagePath it is a derived name, so PruneCaches
// and the fingerprint keying apply to it the same way.
func TokenDigestPath(cache string) string {
	// Slice arithmetic like ImagePath's: the caller always passes a
	// CachePath-shaped ir-*.gob.gz name.
	return cache[:len(cache)-len(".gob.gz")] + ".tok"
}

// tokenDigest hashes the token scripts dir compiles from, the way
// compileTokens reads them: every .txt under TokensDir(dir), in sorted path
// order, hashed as (relative path, 0, contents, 0). Content, not mtime: a
// digest must not agree with a cache compiled from different bytes that
// happen to carry older timestamps, which is exactly the view-mismatch the
// mtime rule cannot see. An absent tokenscripts directory hashes the empty
// string, so a digest recorded where none existed stops matching the moment
// one appears. An unreadable directory is an error — the caller treats every
// error as staleness (recompile), never as a match.
func tokenDigest(dir string) (string, error) {
	root := TokensDir(dir)
	var paths []string
	err := filepath.WalkDir(walkRoot(root), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(p) == ".txt" {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No tokenscripts directory: the empty digest, the state a
			// token-less cache was compiled from.
			return emptyTokenDigest, nil
		}
		return "", err
	}
	// WalkDir yields entries in lexical order within each directory, but a
	// file beside a subdirectory can sort the other way against the
	// subdirectory's contents ("a/b!" vs "a/b/x"), so sort the paths.
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		// The same 0-separated framing computeCompilerFingerprint uses, so no
		// concatenation of file bytes can collide across a different split
		// into names and contents.
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

// emptyTokenDigest is tokenDigest's value for a corpus with no tokenscripts
// directory.
var emptyTokenDigest = hex.EncodeToString(sha256.New().Sum(nil))[:32]

// tokenDigestFresh reports whether cache's digest sidecar agrees with the
// token scripts dir holds now. It is part of cacheFresh, the one home of the
// staleness rule, so the whole-corpus path (openCorpus) and the segment path
// (subset.go) both refuse a token-less cache the same way.
func tokenDigestFresh(dir, cache string) bool {
	want, err := os.ReadFile(TokenDigestPath(cache))
	if err != nil {
		return false
	}
	got, err := tokenDigest(dir)
	if err != nil {
		return false
	}
	return string(want) == got
}

// saveTokenDigest records beside cache the digest of filepath.Dir(cache)'s
// token scripts — the directory compileTokens read r's Tokens from (Save is
// always called with the corpus cache path, and compileTokens always read
// TokensDir of the corpus root that open was made from). It is written after
// saveGob, so the sidecar never appears before the cache it describes, and
// renamed into place atomically like saveGob: a concurrent reader either sees
// the previous consistent pair or a mismatch, and a mismatch only costs a
// recompile.
func saveTokenDigest(cache string) error {
	d, err := tokenDigest(filepath.Dir(cache))
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(cache), filepath.Base(TokenDigestPath(cache))+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	fail := func(err error) error {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if _, err := f.WriteString(d); err != nil {
		return fail(err)
	}
	if err := f.Chmod(0o644); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, TokenDigestPath(cache)); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
