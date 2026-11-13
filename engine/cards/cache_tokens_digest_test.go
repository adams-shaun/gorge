package cards

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The digest sidecar's job: a cache compiled by a corpus view WITHOUT the
// token scripts — the seat-worktree shape, whose .cards mirror can lack
// tokenscripts while it saves through the shared symlink — must not be served
// to a reader whose corpus view HAS them. The mtime rule cannot see the
// difference (the poisoned cache is newer than every reader's tokenscripts
// mtime), so these tests backdate nothing away from it: they attack the digest
// axis directly, the one cacheFresh checks after the mtimes pass.

// tokenScriptBody is a token script authored here; it is not a Forge file.
func tokenScriptBody(name string) string {
	return "Name:" + name + "\nTypes:Token Creature Goblin\nOracle:\n"
}

// seedTokenCorpus writes one card, one token script and the old lock, then
// opens the corpus once so the cache and its digest sidecar exist. It returns
// the corpus dir, the token stem the script compiles to and the first open.
func seedTokenCorpus(t *testing.T, tokenName string) (string, string, *Registry) {
	t.Helper()
	dir := t.TempDir()
	writeScript(t, dir, "mountain.txt", cardBody("Mountain", "Basic Land Mountain"))
	stem := "r_test_" + NormalizeName(tokenName)
	if err := os.MkdirAll(TokensDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(TokensDir(dir), stem+".txt"), []byte(tokenScriptBody(tokenName)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeOldLock(t, dir)
	first, err := OpenCorpus(dir)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if len(first.Tokens) != 1 {
		t.Fatalf("precondition: first open holds %d tokens, want 1", len(first.Tokens))
	}
	if _, ok := first.Token(stem); !ok {
		t.Fatalf("precondition: first open lacks token %s", stem)
	}
	return dir, stem, first
}

// copyCachePair copies the poisoned world's cache and digest sidecar from
// poisonDir (a compile that saw no tokenscripts) over dir's, so dir's next
// open is offered exactly what a seat worktree's save would leave in the
// shared .cards: a token-less cache whose sidecar records the empty digest.
func copyCachePair(t *testing.T, poisonDir, dir string) {
	t.Helper()
	for _, name := range []string{filepath.Base(cachePathFor(dir, CompilerFingerprint)), filepath.Base(TokenDigestPath(cachePathFor(dir, CompilerFingerprint)))} {
		b, err := os.ReadFile(filepath.Join(poisonDir, name))
		if err != nil {
			t.Fatalf("precondition: reading poisoned %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestReopenedCorpusKeepsItsTokens is the regression for the shared-corpus
// token-less cache: it compiles the corpus once, records len(registry.Tokens),
// then serves a cache saved by a token-less view through the cache path and
// asserts the re-opened token set does not shrink.
func TestReopenedCorpusKeepsItsTokens(t *testing.T) {
	dir, stem, first := seedTokenCorpus(t, "Test Goblin")

	// Re-open through the cache path: the token set must not shrink.
	second, err := OpenCorpus(dir)
	if err != nil {
		t.Fatalf("cache-hit open: %v", err)
	}
	if len(second.Tokens) != len(first.Tokens) {
		t.Fatalf("cache-hit open holds %d tokens, first open %d", len(second.Tokens), len(first.Tokens))
	}

	// The poisoning save: a registry compiled from a view with no
	// tokenscripts directory, written to a cache the reader then finds.
	poisonDir := t.TempDir()
	writeScript(t, poisonDir, "mountain.txt", cardBody("Mountain", "Basic Land Mountain"))
	writeOldLock(t, poisonDir)
	poisoned, err := OpenCorpus(poisonDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(poisoned.Tokens) != 0 {
		t.Fatalf("precondition: the token-less view compiled %d tokens, want 0", len(poisoned.Tokens))
	}
	if got, err := tokenDigest(poisonDir); err != nil || got != emptyTokenDigest {
		t.Fatalf("precondition: token-less view digest %q (%v), want the empty digest", got, err)
	}
	// The copied cache is newer than every mtime in dir (it is written now),
	// so the mtime rule alone would serve it: only the digest can refuse it.
	copyCachePair(t, poisonDir, dir)

	third, err := OpenCorpus(dir)
	if err != nil {
		t.Fatalf("open after the poisoned cache is in place: %v", err)
	}
	if len(third.Tokens) != len(first.Tokens) {
		t.Fatalf("poisoned cache shrunk the token set to %d, want %d: the token-less cache is being served", len(third.Tokens), len(first.Tokens))
	}
	if _, ok := third.Token(stem); !ok {
		t.Fatalf("re-opened corpus lost token %s", stem)
	}
	// And the recovery is stable: the recompile re-saved the cache with its
	// digest, so the next open takes the cache-hit path and holds the same
	// tokens again.
	fourth, err := OpenCorpus(dir)
	if err != nil {
		t.Fatalf("open after the recovery recompile: %v", err)
	}
	if len(fourth.Tokens) != len(first.Tokens) {
		t.Fatalf("post-recovery cache-hit open holds %d tokens, want %d", len(fourth.Tokens), len(first.Tokens))
	}
}

// TestCacheIsStaleWhenATokenScriptChangesUnderAnOldMtime pins the content half
// of the digest: a token script rewritten in place with its mtimes left older
// than the cache is invisible to the mtime rule, so only the digest can stop
// the old bytes being served.
func TestCacheIsStaleWhenATokenScriptChangesUnderAnOldMtime(t *testing.T) {
	dir, stem, first := seedTokenCorpus(t, "Old Goblin")

	// Rewrite the token script with different content and backdate the file
	// and its directory two hours: the mtime rule now sees an older world and
	// would serve the cache. Precondition that this is so.
	body := tokenScriptBody("New Goblin")
	if err := os.WriteFile(filepath.Join(TokensDir(dir), stem+".txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(TokensDir(dir), stem+".txt"), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(TokensDir(dir), past, past); err != nil {
		t.Fatal(err)
	}
	ci, err := os.Stat(cachePathFor(dir, CompilerFingerprint))
	if err != nil {
		t.Fatal(err)
	}
	ti, err := os.Stat(TokensDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if ti.ModTime().After(ci.ModTime()) {
		t.Fatal("precondition: tokenscripts is not older than the cache; the mtime rule would already refuse it")
	}

	if _, ok := first.Token(stem); !ok {
		t.Fatal("precondition: first open lacks the token")
	}
	second, err := OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := second.Token(stem)
	if !ok {
		t.Fatalf("re-opened corpus lost token %s", stem)
	}
	if c.Faces[0].Name != "New Goblin" {
		t.Fatalf("re-opened corpus serves the OLD token script (%q), not the rewritten one", c.Faces[0].Name)
	}
}
