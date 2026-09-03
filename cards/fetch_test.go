package cards

import (
	"os"
	"os/exec"
	"path/filepath"
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
		FetchedPath: "forge-gui/res/cardsfolder", Files: 33669, Digest: "abc123",
	}
	if err := WriteLock(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *want {
		t.Fatalf("lock round trip: got %+v want %+v", got, want)
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
	cmd := exec.Command("git", "init", "--bare", repoDir)
	if err := cmd.Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}

	// Create a temporary checkout to commit a file that is NOT at forgeSubpath.
	checkoutDir := filepath.Join(baseDir, "checkout")
	if err := os.Mkdir(checkoutDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "clone", repoDir, checkoutDir)
	if err := cmd.Run(); err != nil {
		t.Skipf("git clone failed: %v", err)
	}

	// Create and commit a file at the root (not at the expected subpath).
	dummyFile := filepath.Join(checkoutDir, "dummy.txt")
	if err := os.WriteFile(dummyFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "-C", checkoutDir, "add", "dummy.txt")
	if err := cmd.Run(); err != nil {
		t.Skipf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "-C", checkoutDir, "-c", "user.email=test@example.com",
		"-c", "user.name=Test", "commit", "-m", "test")
	if err := cmd.Run(); err != nil {
		t.Skipf("git commit failed: %v", err)
	}
	cmd = exec.Command("git", "-C", checkoutDir, "push", "origin", "master")
	if err := cmd.Run(); err != nil {
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
