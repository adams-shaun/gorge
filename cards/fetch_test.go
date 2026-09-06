package cards

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDigestDirIsStableAndOrderIndependent(t *testing.T) {
	build := func(order []string) string {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "a"), 0o755)
		os.MkdirAll(filepath.Join(dir, "b"), 0o755)
		for _, name := range order {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("content of "+name), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		d, n, err := DigestDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(order) {
			t.Fatalf("files = %d, want %d", n, len(order))
		}
		return d
	}
	one := build([]string{"a/one.txt", "b/two.txt", "a/three.txt"})
	two := build([]string{"a/three.txt", "a/one.txt", "b/two.txt"})
	if one != two {
		t.Fatalf("digest depends on write order: %s vs %s", one, two)
	}
}

func TestDigestDirChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.txt"), []byte("one"), 0o644)
	a, _, _ := DigestDir(dir)
	os.WriteFile(filepath.Join(dir, "x.txt"), []byte("two"), 0o644)
	b, _, _ := DigestDir(dir)
	if a == b {
		t.Fatal("digest did not change when content changed")
	}
}

func TestLockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &Lock{
		Repo: "https://github.com/Card-Forge/forge.git", Ref: "master",
		Commit: "deadbeef", License: "GPL-3.0",
		FetchedPath:  "forge-gui/res/cardsfolder",
		FetchedPaths: []string{"forge-gui/res/cardsfolder", "forge-gui/res/tokenscripts"},
		Files:        33669, Digest: "abc123",
	}
	if err := WriteLock(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Lock now carries a slice field (FetchedPaths), so it is no longer a
	// comparable type: reflect.DeepEqual replaces the old `*got != *want`.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lock round trip: got %+v want %+v", got, want)
	}
}

// TestLockReadsOldFormatFetchedPath verifies a lock file written before
// FetchedPaths existed (only the singular "fetched_path" JSON key) still
// reads: ReadLock must not choke on the new plural field being absent.
func TestLockReadsOldFormatFetchedPath(t *testing.T) {
	dir := t.TempDir()
	old := `{
  "repo": "https://github.com/Card-Forge/forge.git",
  "ref": "master",
  "commit": "deadbeef",
  "license": "GPL-3.0",
  "fetched_path": "forge-gui/res/cardsfolder",
  "files": 33669,
  "digest": "abc123"
}`
	if err := os.WriteFile(lockPath(dir), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLock(dir)
	if err != nil {
		t.Fatalf("old-format lock failed to read: %v", err)
	}
	if got.FetchedPath != "forge-gui/res/cardsfolder" {
		t.Fatalf("FetchedPath = %q", got.FetchedPath)
	}
	if len(got.FetchedPaths) != 0 {
		t.Fatalf("FetchedPaths = %v, want empty for an old-format lock", got.FetchedPaths)
	}
}

func TestFetchDownloadsCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	dir := t.TempDir()
	l, err := Fetch(dir, "master")
	if err != nil {
		t.Skipf("fetch unavailable: %v", err)
	}
	if l.Files < 30000 {
		t.Fatalf("fetched %d card scripts, expected >30000", l.Files)
	}
	if l.Commit == "" || l.Digest == "" {
		t.Fatalf("lock incomplete: %+v", l)
	}
}

func TestDigestDirNULCollisionResistance(t *testing.T) {
	// Before length-framing, a.txt containing "X\x00b.txt\x00Y" would hash
	// identically to two files a.txt="X" and b.txt="Y". Verify the collision
	// is closed with the new length-framing scheme.
	embeddedDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(embeddedDir, "a.txt"),
		[]byte("X\x00b.txt\x00Y"), 0o644); err != nil {
		t.Fatal(err)
	}
	embeddedDigest, _, _ := DigestDir(embeddedDir)

	separateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(separateDir, "a.txt"), []byte("X"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(separateDir, "b.txt"), []byte("Y"), 0o644); err != nil {
		t.Fatal(err)
	}
	separateDigest, _, _ := DigestDir(separateDir)

	if embeddedDigest == separateDigest {
		t.Fatalf("digest collision: embedded NULs should not hash identically: %s", embeddedDigest)
	}
}

func TestFetchCleansUpOnFailure(t *testing.T) {
	// Create a local git repository that lacks the expected subpath.
	// Verify fetchRepo returns an error and leaves no work directory behind.
	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "test.git")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Initialize a bare repository.
	if err := gitCmd("init", "--bare", repoDir).Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}

	// Create a temporary checkout to commit a file that is NOT at forgeSubpath.
	checkoutDir := filepath.Join(baseDir, "checkout")
	if err := os.Mkdir(checkoutDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := gitCmd("clone", repoDir, checkoutDir).Run(); err != nil {
		t.Skipf("git clone failed: %v", err)
	}

	// Create and commit a file at the root (not at the expected subpath).
	dummyFile := filepath.Join(checkoutDir, "dummy.txt")
	if err := os.WriteFile(dummyFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitCmd("-C", checkoutDir, "add", "dummy.txt").Run(); err != nil {
		t.Skipf("git add failed: %v", err)
	}
	if err := gitCmd("-C", checkoutDir, "-c", "user.email=test@example.com",
		"-c", "user.name=Test", "commit", "-m", "test").Run(); err != nil {
		t.Skipf("git commit failed: %v", err)
	}
	if err := gitCmd("-C", checkoutDir, "push", "origin", "master").Run(); err != nil {
		t.Skipf("git push failed: %v", err)
	}

	// Now attempt to fetch from this local repository, pointing at master.
	// Use file:// URL to access the local bare repository.
	fetchDir := filepath.Join(baseDir, "fetch")
	if err := os.Mkdir(fetchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	fileURL := "file://" + filepath.ToSlash(repoDir)
	_, err := fetchRepo(fileURL, fetchDir, "master")
	if err == nil {
		t.Fatal("fetchRepo should have failed (missing subpath)")
	}

	// Verify the work directory was cleaned up.
	workDir := filepath.Join(fetchDir, "forge.git-checkout")
	if _, err := os.Stat(workDir); err == nil {
		t.Fatalf("work directory still exists after failure: %s", workDir)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking work directory: %v", err)
	}
}

// gitCmd builds a git command whose environment is scrubbed of every
// inherited GIT_* variable (see GitEnv). These tests init, add, commit and
// push throwaway repositories; a hook or wrapper that exported GIT_DIR or
// GIT_INDEX_FILE would otherwise redirect those writes into whatever
// repository the test runs under (the gitiso bug).
func gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Env = GitEnv()
	return cmd
}

// runGit runs a git command with dir as its working directory, failing the
// test on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := gitCmd(full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(full, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newLocalForgeRepo builds a local, non-bare git repository seeded with a
// forge-gui/res/cardsfolder tree, so fetchRepo can be exercised without
// network access. `git clone` treats a repository with a working tree the
// same as a bare one, so — unlike TestFetchCleansUpOnFailure's bare-repo +
// push dance above — a plain `git init` is enough here.
func newLocalForgeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	cardDir := filepath.Join(dir, "forge-gui", "res", "cardsfolder")
	if err := os.MkdirAll(cardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cardDir, "mountain.txt"),
		[]byte("Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "forge-gui")
	runGit(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=Test",
		"commit", "-m", "seed cardsfolder")
	return dir
}

// addTokenScript adds one file under forge-gui/res/tokenscripts to repo and
// commits it, giving fetchRepo a second sparse-checkout path to pull and a
// SHA that carries both subpaths.
func addTokenScript(t *testing.T, repo, name, src string) {
	t.Helper()
	tokenDir := filepath.Join(repo, "forge-gui", "res", "tokenscripts")
	if err := os.MkdirAll(tokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tokenDir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "forge-gui")
	runGit(t, repo, "-c", "user.email=test@example.com", "-c", "user.name=Test",
		"commit", "-m", "add token script")
}

// headSHA returns repo's current HEAD commit hash.
func headSHA(t *testing.T, repo string) string {
	t.Helper()
	return runGit(t, repo, "rev-parse", "HEAD")
}

// currentBranch returns repo's currently checked-out branch name. Used
// instead of hard-coding "master"/"main": newLocalForgeRepo never sets
// --initial-branch, so the name it gets depends on this environment's
// init.defaultBranch.
func currentBranch(t *testing.T, repo string) string {
	t.Helper()
	return runGit(t, repo, "branch", "--show-current")
}

func TestFetchByCommitSHAAndTokenScripts(t *testing.T) {
	repo := newLocalForgeRepo(t) // the local, non-bare repo built above
	addTokenScript(t, repo, "r_1_1_goblin.txt", goblinTokenSrc)
	sha := headSHA(t, repo)
	dir := t.TempDir()
	l, err := fetchRepo(repo, dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if l.Commit != sha || len(l.FetchedPaths) != 2 {
		t.Fatalf("lock %+v", l)
	}
	if _, err := os.Stat(filepath.Join(TokensDir(dir), "r_1_1_goblin.txt")); err != nil {
		t.Fatal("token script not fetched")
	}
}

// TestFetchByBranchFetchesBothSparsePaths is the branch-ref counterpart to
// TestFetchByCommitSHAAndTokenScripts: it hits the OTHER half of fetchRepo's
// ref-kind branch (the pre-existing --branch clone path), which neither
// TestFetchCleansUpOnFailure (deliberately missing both subpaths, so it
// exercises only the failure path) nor TestFetchDownloadsCorpus (skipped
// without network) actually proves fetches both sparse paths successfully.
func TestFetchByBranchFetchesBothSparsePaths(t *testing.T) {
	repo := newLocalForgeRepo(t)
	addTokenScript(t, repo, "r_1_1_goblin.txt", goblinTokenSrc)
	branch := currentBranch(t, repo)
	dir := t.TempDir()
	l, err := fetchRepo(repo, dir, branch)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.FetchedPaths) != 2 {
		t.Fatalf("lock %+v", l)
	}
	if _, err := os.Stat(filepath.Join(CorpusDir(dir), "mountain.txt")); err != nil {
		t.Fatal("card script not fetched via branch ref")
	}
	if _, err := os.Stat(filepath.Join(TokensDir(dir), "r_1_1_goblin.txt")); err != nil {
		t.Fatal("token script not fetched via branch ref")
	}
}

// TestFetchIgnoresInheritedGitEnv is the regression test for the gitiso bug:
// fetchRepo used to hand its child git processes the caller's whole
// environment, so under a git hook -- git exports GIT_DIR, GIT_INDEX_FILE and
// friends pointing at the ENCLOSING repository -- every run() invocation
// silently operated on that repository instead of on the throwaway `work`
// directory it names: `git init` reinitialized the wrong repository, `git
// remote add` and `git sparse-checkout` wrote config into it, `git fetch`
// pulled into its object store, and `git checkout FETCH_HEAD` rewrote its
// index. This destroyed a gorge repository's index twice in two days.
//
// The test arms the same environment -- GIT_DIR, GIT_WORK_TREE and
// GIT_INDEX_FILE pointing at a throwaway outer repo -- via t.Setenv so
// fetchRepo literally inherits them, runs fetchRepo against the local
// fixture (no network), and asserts the outer repo came through unchanged.
// On the unfixed fetchRepo the redirect changes the outer repo's config
// (remote origin, extensions.worktreeConfig) and index, so the test fails;
// with the scrub in place every child git process addresses its own
// repository explicitly and the outer repo is untouched.
func TestFetchIgnoresInheritedGitEnv(t *testing.T) {
	// A throwaway "enclosing" repository standing in for the repository a
	// caller might be operating under when fetchRepo runs.
	outer := t.TempDir()
	runGit(t, outer, "init")
	runGit(t, outer, "-c", "user.email=test@example.com", "-c", "user.name=Test",
		"commit", "--allow-empty", "-m", "init")
	beforeConfig, err := os.ReadFile(filepath.Join(outer, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	beforeListing := runGit(t, outer, "ls-files")

	// Simulate the git-hook environment: GIT_DIR, GIT_WORK_TREE and
	// GIT_INDEX_FILE exported, pointing at the enclosing repo. t.Setenv runs
	// in this test process, so fetchRepo and the fixtures built below inherit
	// them exactly as a hook's child would.
	t.Setenv("GIT_DIR", filepath.Join(outer, ".git"))
	t.Setenv("GIT_WORK_TREE", outer)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(outer, ".git", "index"))

	// The local fixture (no network) and the fetch into a fresh dir, under
	// the hostile environment.
	repo := newLocalForgeRepo(t)
	addTokenScript(t, repo, "r_1_1_goblin.txt", goblinTokenSrc)
	sha := headSHA(t, repo)
	dir := t.TempDir()
	l, ferr := fetchRepo(repo, dir, sha)

	// Every assertion is about the OUTER repo, which must be untouched
	// regardless of whether the fetch itself succeeded.
	afterConfig, err := os.ReadFile(filepath.Join(outer, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterConfig, beforeConfig) {
		t.Errorf("fetchRepo rewrote the enclosing repository's .git/config:\n--- after ---\n%s--- before ---\n%s", afterConfig, beforeConfig)
	}
	if _, err := os.Stat(filepath.Join(outer, ".git", "config.worktree")); !os.IsNotExist(err) {
		t.Errorf("fetchRepo created the enclosing repository's .git/config.worktree")
	}
	if afterListing := runGit(t, outer, "ls-files"); afterListing != beforeListing {
		t.Errorf("enclosing repository index changed: %d entries before, %d after",
			len(strings.Fields(beforeListing)), len(strings.Fields(afterListing)))
	}
	if ferr != nil {
		t.Fatalf("fetchRepo failed: %v", ferr)
	}
	if l.Commit != sha {
		t.Fatalf("fetchRepo lock commit = %s, want %s", l.Commit, sha)
	}
}
