package cards

import (
	"os"
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
