package cards

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A corpus view whose tokenscripts (or script) root is a symlink — the shape
// cards-overlay.sh makes — must read the scripts behind the link. filepath.WalkDir
// does not follow a symlinked root: it yields the link as one non-directory entry,
// so every walk saw zero scripts and the build published token-starved tables
// that later runs accepted as current.

func TestSymlinkedTokenRootIsRead(t *testing.T) {
	realDir, stem, _ := seedTokenCorpus(t, "Linkgob")

	linked := t.TempDir()
	writeScript(t, linked, "mountain.txt", cardBody("Mountain", "Basic Land Mountain"))
	writeOldLock(t, linked)
	if err := os.Symlink(TokensDir(realDir), TokensDir(linked)); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(TokensDir(linked)); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("precondition: %s is not a symlink (%v)", TokensDir(linked), err)
	}

	want, err := tokenDigest(realDir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tokenDigest(linked)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("tokenDigest through a symlinked root = %s, want the real tree's %s", got, want)
	}
	if want == emptyTokenDigest {
		t.Fatal("precondition: the real tree hashes as empty")
	}

	reg, err := OpenCorpus(linked)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Token(stem); !ok {
		t.Errorf("OpenCorpus through a symlinked tokenscripts root compiled %d tokens, want %s", len(reg.Tokens), stem)
	}

	d1, n1, err := DigestDir(TokensDir(realDir))
	if err != nil {
		t.Fatal(err)
	}
	d2, n2, err := DigestDir(TokensDir(linked))
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 1 || d1 != d2 || n1 != n2 {
		t.Errorf("DigestDir through a symlinked root = (%s, %d), want (%s, %d)", d2, n2, d1, n1)
	}
}

// A symlinked directory below the root is the same hole one level down, and the
// overlay cannot be trusted to have none: refuse it by name.
func TestSymlinkedScriptDirBelowRootIsRefused(t *testing.T) {
	realDir, _, _ := seedTokenCorpus(t, "Linkgob")
	dir := t.TempDir()
	if err := os.MkdirAll(TokensDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(TokensDir(realDir), filepath.Join(TokensDir(dir), "sub")); err != nil {
		t.Fatal(err)
	}
	_, err := tokenDigest(dir)
	if err == nil || !strings.Contains(err.Error(), "symlinked directory") {
		t.Fatalf("tokenDigest over a symlinked subdirectory = %v, want a symlinked directory error", err)
	}
}
