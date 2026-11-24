package cards

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fetchedFixture fetches the local fixture repo into a fresh dir.
func fetchedFixture(t *testing.T) string {
	t.Helper()
	repo := newLocalForgeRepo(t)
	addTokenScript(t, repo, "r_1_1_goblin.txt", goblinTokenSrc)
	dir := t.TempDir()
	if _, err := fetchRepo(repo, dir, headSHA(t, repo)); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A fetch records what it fetched, and the lock then verifies.
func TestFetchLockDescribesItsContent(t *testing.T) {
	dir := fetchedFixture(t)
	l, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if l.Listing == "" || l.TokenFiles != 1 || l.Files < 1 {
		t.Fatalf("lock does not describe its content: %+v", l)
	}
	if err := VerifyLock(dir); err != nil {
		t.Fatalf("a fresh fetch fails its own lock: %v", err)
	}
}

// The mismatch classes seen on the boxes: stale files left beside the right
// ones (an extract or rsync without --delete), a missing script, and the
// same count under other names (another ref's listing under this lock).
func TestVerifyLockCatchesContentDrift(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, dir string)
		want   string
	}{
		{"extra stale card", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(CorpusDir(dir), "stale_card.txt"), []byte("Name:Stale\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "card scripts"},
		{"missing card", func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(CorpusDir(dir), "mountain.txt")); err != nil {
				t.Fatal(err)
			}
		}, "card scripts"},
		{"extra stale token", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(TokensDir(dir), "old_token.txt"), []byte("Name:Old\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "token scripts"},
		{"same count, other name", func(t *testing.T, dir string) {
			if err := os.Rename(filepath.Join(CorpusDir(dir), "mountain.txt"), filepath.Join(CorpusDir(dir), "renamed.txt")); err != nil {
				t.Fatal(err)
			}
		}, "different scripts"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := fetchedFixture(t)
			c.mutate(t, dir)
			err := VerifyLock(dir)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("VerifyLock = %v, want an error naming %q", err, c.want)
			}
		})
	}
}

// A lock from before Listing existed is checked by its card count alone, and a
// stub lock (no counts) or no lock is not checked.
func TestVerifyLockOldAndStubLocks(t *testing.T) {
	dir := fetchedFixture(t)
	l, _ := ReadLock(dir)
	l.Listing, l.TokenFiles = "", 0
	if err := WriteLock(dir, l); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLock(dir); err != nil {
		t.Fatalf("old-format lock: %v", err)
	}
	l.Files++
	WriteLock(dir, l)
	if err := VerifyLock(dir); err == nil {
		t.Fatal("old-format lock with a wrong count passed")
	}
	if err := WriteLock(dir, &Lock{Ref: "abc"}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLock(dir); err != nil {
		t.Fatalf("stub lock: %v", err)
	}
	os.Remove(filepath.Join(dir, "cards.lock"))
	if err := VerifyLock(dir); err != nil {
		t.Fatalf("no lock: %v", err)
	}
}

// The overlay bug: cards.lock is a symlink into the shared corpus (what
// cards-overlay.sh makes), cardsfolder is replaced by a fetch, and the old
// os.WriteFile followed the link and rewrote the SHARED lock. The shared lock
// must stay as it was, and the overlay must end with its own lock.
func TestFetchInOverlayNeverWritesThroughTheLockSymlink(t *testing.T) {
	shared := fetchedFixture(t)
	before, err := os.ReadFile(filepath.Join(shared, "cards.lock"))
	if err != nil {
		t.Fatal(err)
	}
	overlay := t.TempDir()
	for _, n := range []string{"cards.lock", "cardsfolder", "tokenscripts"} {
		if err := os.Symlink(filepath.Join(shared, n), filepath.Join(overlay, n)); err != nil {
			t.Fatal(err)
		}
	}
	repo := newLocalForgeRepo(t)
	addTokenScript(t, repo, "r_1_1_goblin.txt", goblinTokenSrc)
	if err := os.WriteFile(filepath.Join(repo, "forge-gui/res/cardsfolder/zzz_extra.txt"), []byte("Name:Extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-q", "-m", "extra")
	if _, err := fetchRepo(repo, overlay, headSHA(t, repo)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(shared, "cards.lock"))
	if string(after) != string(before) {
		t.Fatalf("a fetch in an overlay rewrote the shared cards.lock:\n%s", after)
	}
	if err := VerifyLock(shared); err != nil {
		t.Fatalf("shared corpus no longer matches its lock: %v", err)
	}
	if err := VerifyLock(overlay); err != nil {
		t.Fatalf("overlay corpus does not match its own lock: %v", err)
	}
	if fi, err := os.Lstat(filepath.Join(overlay, "cards.lock")); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("overlay cards.lock should be its own file: %v %v", fi, err)
	}
}

// An interrupted fetch must not leave the old lock over new or half-moved
// scripts: a failed fetch after the scripts were replaced leaves no lock.
func TestFetchDropsTheOldLockBeforeReplacingScripts(t *testing.T) {
	dir := fetchedFixture(t)
	repo := newLocalForgeRepo(t)
	// A tokenscripts-less repo makes the rename of the second subpath fail
	// after the first was replaced (the failure path of TestFetchCleansUpOnFailure).
	if _, err := fetchRepo(repo, dir, headSHA(t, repo)); err == nil {
		t.Skip("fixture fetch unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(dir, "cards.lock")); err == nil {
		t.Fatal("a failed fetch left the previous cards.lock over replaced scripts")
	}
}

// ListingOf is the hash scripts/test-remote*.sh compute in a shell on a box
// with no Go; the two must be the same number.
func TestListingOfMatchesTheShellListing(t *testing.T) {
	dir := fetchedFixture(t)
	l, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", "-c", "cd "+dir+" && find -L cardsfolder tokenscripts -type f -name '*.txt' | LC_ALL=C sort | sha256sum | cut -d' ' -f1").Output()
	if err != nil {
		t.Skipf("no shell find/sort: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != l.Listing {
		t.Fatalf("shell listing %s != lock listing %s", got, l.Listing)
	}
}
