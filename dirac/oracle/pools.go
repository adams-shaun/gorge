package oracle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// pools.go: the frozen lockstep pools as data. A pool's pairs are a pure
// function of pool.tsv's order and length (PairingAt), so a pool is pinned by
// the digest of its whole tree, not only its deck set. A new set's pool is a
// row here plus a digest; kfuzz -kernel <name> resolves it with no new code.
// dirac/oracle/pools/decks.py builds a pool and writes the same digest to
// its pool.manifest.
//
// Pool trees are not committed. PoolsRoot says where they live: $GORGE_POOLS,
// else .pools at the module root. A worktree links .pools the way it links
// .cards; without either, every pool test skips and the commands play the
// committed slices (slices.go).

// Pool is one registered lockstep pool: Name is the cardset name and the
// kfuzz -kernel value, Inputs the directory holding decks/ and pool.tsv (set by
// PoolByName, empty from Pools), Label where the decks come from, Tree the
// TreeDigest of Inputs and Decks the number of decks.
type Pool struct {
	Name, Inputs, Label, Tree string
	Decks                     int
}

// Pools is the registry, in fixed order.
func Pools() []Pool {
	return []Pool{
		{Name: "fdn", Label: "gorge self-play", Tree: "b6ecdc390c432582f34348ee28f36cf3f0bf03d15d7f61814931ad93fe6c3b65", Decks: 31516},
		{Name: "blb", Label: "17lands", Tree: "e01290b140e80bd559b2df92aaa9f9a846d8db647e098effce742c92fdefc7f0", Decks: 145},
		{Name: "pauper", Label: "SpellBench pauper-bo1", Tree: "63423c92b1c867dc4e0406a23a08e6a84ba8e8975ff226902024674f40f9655a", Decks: 8},
		{Name: "cmd5", Label: "gorge Foundations Commander precons", Tree: "bf7d262947055c44eb84c14e648249e7d907b0bfcca788f9fd04f9f53f300072", Decks: 5},
		{Name: "standard-meta", Label: "MTGGoldfish Standard", Tree: "3788c0cc4bcecf3807a3c2045e33d466d04270f0d73145ed6c39a212f0122a9f", Decks: 60},
		{Name: "mbc", Label: "draftsim MBC Commander drafts, 50-card decks", Tree: "92732986d6ecb20f20b86231ad525c7f8ca2c0a2fb1e1ac2970c07774659bbc0", Decks: 234},
	}
}

// PoolsRoot is the directory holding <name>/decks/*.json and <name>/pool.tsv:
// $GORGE_POOLS, else .pools in the module root found by walking up from the
// working directory to go.mod. It does not check the directory exists. "" means
// neither: GORGE_POOLS is unset and no go.mod is above the working directory.
func PoolsRoot() string {
	if v := os.Getenv("GORGE_POOLS"); v != "" {
		return v
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, ".pools")
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

// PoolInputs is the directory holding a pool's decks/ and pool.tsv.
// $DIRAC_POOL_<UPPER(name)> wins; for blb only, $KORACLE_BLB_DECKS is second;
// then <PoolsRoot>/<name>. "" when none resolves.
func PoolInputs(name string) string {
	if v := os.Getenv("DIRAC_POOL_" + strings.ToUpper(name)); v != "" {
		return v
	}
	if name == "blb" {
		if v := os.Getenv("KORACLE_BLB_DECKS"); v != "" {
			return v
		}
	}
	root := PoolsRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, name)
}

// PoolDecks is PoolInputs(name)/decks, or "" when that is "".
func PoolDecks(name string) string {
	in := PoolInputs(name)
	if in == "" {
		return ""
	}
	return filepath.Join(in, "decks")
}

// NeedDecks is nil when decks is a directory, else the error a command prints
// instead of failing later on a bare path.
func NeedDecks(decks string) error {
	if st, err := os.Stat(decks); err == nil && st.IsDir() {
		return nil
	}
	return fmt.Errorf("no deck pool at %q: set GORGE_POOLS to a directory holding <pool>/decks and <pool>/pool.tsv, link it as .pools at the module root, or pass -decks", decks)
}

// PoolByName returns the registered pool with Inputs set by PoolInputs.
func PoolByName(name string) (Pool, error) {
	for _, p := range Pools() {
		if p.Name != name {
			continue
		}
		p.Inputs = PoolInputs(name)
		return p, nil
	}
	names := make([]string, 0, len(Pools()))
	for _, p := range Pools() {
		names = append(names, p.Name)
	}
	return Pool{}, fmt.Errorf("unknown pool %q (registered: %s)", name, strings.Join(names, ", "))
}

// TreeDigest is the sha256 of the `sha256sum` lines of decks/** and the top-level
// pool.tsv, with forward-slash relative paths in byte order. Nothing else in the
// tree is in scope: a README, summary.txt, pool.manifest or ATTRIBUTION.txt can
// change without moving the pin. dir may be a symlink, and in a pool farm it is.
//
//	find ./decks ./pool.tsv -type f | LC_ALL=C sort | xargs -d '\n' sha256sum | sha256sum
func TreeDigest(dir string) (string, error) {
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	rels := []string{"pool.tsv"}
	err = filepath.WalkDir(filepath.Join(dir, "decks"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.ContainsAny(rel, "\\\n") {
			return fmt.Errorf("%q: backslash or newline in a path (sha256sum escapes it)", rel)
		}
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rels) // Go string order is byte order
	all := sha256.New()
	for _, rel := range rels {
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(all, "%s  ./%s\n", hex.EncodeToString(h.Sum(nil)), rel)
	}
	return hex.EncodeToString(all.Sum(nil)), nil
}
