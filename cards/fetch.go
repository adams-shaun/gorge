package cards

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	forgeRepo    = "https://github.com/Card-Forge/forge.git"
	forgeSubpath = "forge-gui/res/cardsfolder"
	forgeLicense = "GPL-3.0"
)

// Lock records exactly which upstream card data a build used, so a match's
// card definitions are as reproducible as its seed.
type Lock struct {
	Repo        string `json:"repo"`
	Ref         string `json:"ref"`
	Commit      string `json:"commit"`
	License     string `json:"license"`
	FetchedPath string `json:"fetched_path"`
	Files       int    `json:"files"`
	Digest      string `json:"digest"`
}

// CorpusDir is where Fetch places the scripts inside dir.
func CorpusDir(dir string) string { return filepath.Join(dir, "cardsfolder") }

func lockPath(dir string) string { return filepath.Join(dir, "cards.lock") }

// Fetch sparse-clones the Forge card corpus at ref into dir. The scripts are
// GPL-3.0 and must never be committed or shipped; dir is gitignored.
func Fetch(dir, ref string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	work := filepath.Join(dir, "forge.git-checkout")
	if err := os.RemoveAll(work); err != nil {
		return nil, err
	}
	run := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}
	if _, err := run("clone", "--depth", "1", "--filter=blob:none", "--sparse",
		"--branch", ref, forgeRepo, work); err != nil {
		return nil, err
	}
	if _, err := run("-C", work, "sparse-checkout", "set", forgeSubpath); err != nil {
		return nil, err
	}
	commit, err := run("-C", work, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	dest := CorpusDir(dir)
	if err := os.RemoveAll(dest); err != nil {
		return nil, err
	}
	if err := os.Rename(filepath.Join(work, forgeSubpath), dest); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(work); err != nil {
		return nil, err
	}

	digest, files, err := DigestDir(dest)
	if err != nil {
		return nil, err
	}
	l := &Lock{Repo: forgeRepo, Ref: ref, Commit: commit, License: forgeLicense,
		FetchedPath: forgeSubpath, Files: files, Digest: digest}
	if err := WriteLock(dir, l); err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr,
		"fetched %d card scripts from %s at %s (%s)\n"+
			"These scripts are licensed %s. They are a build input only:\n"+
			"never commit them, never include them in a published image.\n",
		files, forgeRepo, commit[:min(8, len(commit))], ref, forgeLicense)
	return l, nil
}

// DigestDir hashes a directory's .txt contents in sorted path order, so the
// digest depends on content and layout but not on filesystem iteration order.
func DigestDir(dir string) (string, int, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".txt") {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, rel := range paths {
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return "", 0, err
		}
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), len(paths), nil
}

func WriteLock(dir string, l *Lock) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(lockPath(dir), append(b, '\n'), 0o644)
}

func ReadLock(dir string) (*Lock, error) {
	b, err := os.ReadFile(lockPath(dir))
	if err != nil {
		return nil, err
	}
	var l Lock
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, err
	}
	return &l, nil
}
