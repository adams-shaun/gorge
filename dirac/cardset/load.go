package cardset

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge"
)

// Finding and loading binary card tables. Tables live, gitignored, under the
// corpus they were built from:
//
//	<cards>/kernel-tables/<key>/manifest.tsv    pool, numCards, numTokens, sha256(payload); sorted by pool
//	<cards>/kernel-tables/<key>/<pool>.ktab     one table
//	<cards>/kernel-tables/<key>/<pool>.coverage.md  kgen's refusal report
//
// The key is <16 hex of cards.lock's digest>-<16 hex of gorge.TableSourceHash>.
// The binary embeds the second half, so the bytes under a key never change and
// this package never writes. A stale tree fails loudly with the command to run;
// only a tree with no corpus at all reports ErrNoCorpus, which tests skip on.

var (
	// ErrNoCorpus: no corpus was located, or it has no cards.lock.
	ErrNoCorpus = errors.New("cardset: no card corpus (link .cards, or set GORGE_CARDS or DIRAC_TABLES)")
	// ErrNoTables: a corpus is there but no tables were built for this binary.
	ErrNoTables = errors.New("cardset: no card tables for this binary")
)

// ManifestFile is the name of a key directory's index, written last.
const ManifestFile = "manifest.tsv"

// TablesSubdir is the directory under a corpus that holds the key directories.
const TablesSubdir = "kernel-tables"

const keyPartLen = 16

// CorpusDir locates the corpus: $GORGE_CARDS, else the first .cards walking up from
// the cwd. It returns the path as found, not resolved. An explicit $GORGE_CARDS
// without a cards.lock is ErrNoCorpus rather than a walk: it is a misconfiguration
// a walk would hide.
func CorpusDir() (string, error) {
	if d := os.Getenv("GORGE_CARDS"); d != "" {
		return checkCorpus(d)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoCorpus, err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		p := filepath.Join(dir, ".cards")
		if _, err := os.Lstat(p); err == nil {
			return checkCorpus(p)
		}
		if dir == filepath.Dir(dir) {
			return "", fmt.Errorf("%w: no .cards in or above %s", ErrNoCorpus, wd)
		}
	}
}

// checkCorpus stats the lock through the path as found: when that path lies inside
// the module, as a symlinked .cards does, go test's input tracking then sees a corpus
// swap, while the tables themselves are read resolved, outside its tracking.
func checkCorpus(d string) (string, error) {
	if _, err := os.Stat(filepath.Join(d, "cards.lock")); err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoCorpus, err)
	}
	return d, nil
}

var hexDigest = regexp.MustCompile(`^[0-9a-f]{16,}$`)

// KeyFor is the table key this binary uses with the corpus at dir.
func KeyFor(dir string) (string, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoCorpus, err)
	}
	path := filepath.Join(resolved, "cards.lock")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %v", ErrNoCorpus, err)
		}
		return "", err
	}
	var lock struct {
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return "", fmt.Errorf("cardset: %s: %v", path, err)
	}
	if !hexDigest.MatchString(lock.Digest) {
		return "", fmt.Errorf("cardset: %s: digest %q is not hex", path, lock.Digest)
	}
	return lock.Digest[:keyPartLen] + "-" + sourceKeyPart(), nil
}

func sourceKeyPart() string { return gorge.TableSourceHash()[:keyPartLen] }

// Key is the table key for the located corpus.
func Key() (string, error) {
	d, err := CorpusDir()
	if err != nil {
		return "", err
	}
	return KeyFor(d)
}

// published reports whether dir is a finished key directory, statting through the
// path as given (see checkCorpus: a test that skipped because tables were missing
// must not stay cached as a skip once they exist).
func published(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ManifestFile))
	return err == nil
}

// TablesDir returns this binary's table directory, symlinks resolved, first match
// winning:
//
//  1. $DIRAC_TABLES/<key>, for a deployed or pinned binary; $DIRAC_TABLES is a copy
//     of a kernel-tables directory
//  2. $GORGE_CARDS/kernel-tables/<key>
//  3. <first .cards walking up from the cwd>/kernel-tables/<key>
//
// A deployed binary may have $DIRAC_TABLES and no corpus, so no cards.lock to read
// the corpus half of the key from. Then the key directory is the one whose source
// half is this binary's; more than one is an error, never a guess.
func TablesDir() (string, error) {
	corpus, cerr := CorpusDir()
	var key string
	if cerr == nil {
		var err error
		if key, err = KeyFor(corpus); err != nil {
			if !errors.Is(err, ErrNoCorpus) {
				return "", err
			}
			cerr = err
		}
	}
	if d := os.Getenv("DIRAC_TABLES"); d != "" {
		if cerr != nil {
			return tablesWithoutCorpus(d)
		}
		if dir := filepath.Join(d, key); published(dir) {
			return filepath.EvalSymlinks(dir)
		}
	}
	if cerr != nil {
		return "", cerr
	}
	dir := filepath.Join(corpus, TablesSubdir, key)
	if !published(dir) {
		return "", fmt.Errorf("%w: key %s, not in %s: run make dirac-tables", ErrNoTables, key, dir)
	}
	return filepath.EvalSymlinks(dir)
}

func tablesWithoutCorpus(d string) (string, error) {
	suffix := "-" + sourceKeyPart()
	ents, err := os.ReadDir(d)
	if err != nil {
		return "", fmt.Errorf("%w: DIRAC_TABLES: %v", ErrNoTables, err)
	}
	var found []string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), suffix) && published(filepath.Join(d, e.Name())) {
			found = append(found, e.Name())
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("%w: nothing matching *%s in DIRAC_TABLES=%s, and no corpus to rebuild from: run make dirac-tables and copy the result", ErrNoTables, suffix, d)
	case 1:
		return filepath.EvalSymlinks(filepath.Join(d, found[0]))
	}
	return "", fmt.Errorf("cardset: DIRAC_TABLES=%s holds %v for this binary: set GORGE_CARDS to choose by corpus, or remove all but one", d, found)
}

// ManifestRow is one line of manifest.tsv.
type ManifestRow struct {
	Pool                string
	NumCards, NumTokens int
	Sum                 string // hex sha256(payload)
}

// EncodeManifest renders rows, which the caller has sorted by pool.
func EncodeManifest(rows []ManifestRow) []byte {
	var b bytes.Buffer
	for _, r := range rows {
		fmt.Fprintf(&b, "%s\t%d\t%d\t%s\n", r.Pool, r.NumCards, r.NumTokens, r.Sum)
	}
	return b.Bytes()
}

// ReadManifest reads a key directory's manifest.
func ReadManifest(dir string) ([]ManifestRow, error) {
	path := filepath.Join(dir, ManifestFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cardset: %w", err)
	}
	var rows []ManifestRow
	for n, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("cardset: %s:%d: malformed line %q", path, n+1, line)
		}
		cards, err1 := strconv.Atoi(f[1])
		tokens, err2 := strconv.Atoi(f[2])
		if _, err3 := hex.DecodeString(f[3]); err1 != nil || err2 != nil || err3 != nil || f[0] == "" {
			return nil, fmt.Errorf("cardset: %s:%d: malformed line %q", path, n+1, line)
		}
		rows = append(rows, ManifestRow{Pool: f[0], NumCards: cards, NumTokens: tokens, Sum: f[3]})
	}
	return rows, nil
}

// LoadFile reads and decodes one table, naming the file in any error.
func LoadFile(path string) (*PoolData, error) {
	pd, _, err := loadTable(path)
	return pd, err
}

func loadTable(path string) (*PoolData, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cardset: %w", err)
	}
	pd, _, err := DecodeTable(b)
	if err != nil {
		return nil, nil, fmt.Errorf("cardset: %s: %w", path, err)
	}
	return pd, b, nil
}

// The lazy per-pool cache. ByName runs in parallel in tests and kfuzz workers, so
// everything is under the mutex. It is keyed by directory: a test or a long-lived
// process can change which directory TablesDir resolves to.
var (
	loadMu    sync.Mutex
	loadedSet = map[string]Set{}
	manifests = map[string][]ManifestRow{}
)

// tableNames is the pools in this binary's manifest, which kgen writes sorted.
func tableNames() ([]string, error) {
	dir, err := TablesDir()
	if err != nil {
		return nil, err
	}
	loadMu.Lock()
	defer loadMu.Unlock()
	rows, err := manifestOf(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i := range rows {
		out[i] = rows[i].Pool
	}
	return out, nil
}

// manifestOf is dir's manifest, read once. The caller holds loadMu.
func manifestOf(dir string) ([]ManifestRow, error) {
	if rows, ok := manifests[dir]; ok {
		return rows, nil
	}
	rows, err := ReadManifest(dir)
	if err != nil {
		return nil, err
	}
	manifests[dir] = rows
	return rows, nil
}

// load returns the Set for pool name from this binary's tables, decoding it on
// first use. The key in the file must be the key directory's own name.
func load(name string) (Set, error) {
	dir, err := TablesDir()
	if err != nil {
		return nil, err
	}
	return loadFrom(dir, name)
}

// loadFrom is load from a given key directory.
func loadFrom(dir, name string) (Set, error) {
	loadMu.Lock()
	defer loadMu.Unlock()
	path := filepath.Join(dir, name+".ktab")
	if s, ok := loadedSet[path]; ok {
		return s, nil
	}
	rows, err := manifestOf(dir)
	if err != nil {
		return nil, err
	}
	var row *ManifestRow
	for i := range rows {
		if rows[i].Pool == name {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		have := make([]string, len(rows))
		for i := range rows {
			have[i] = rows[i].Pool
		}
		return nil, fmt.Errorf("cardset: no set named %q in the tables at %s (have: %v)", name, dir, have)
	}
	pd, b, err := loadTable(path)
	if err != nil {
		return nil, err
	}
	if k, _, _, err := parseTableHeader(b); err != nil || k != filepath.Base(dir) {
		return nil, fmt.Errorf("cardset: %s: belongs to key %q, not this directory", path, k)
	}
	if sum, err := TableSum(b); err != nil || sum != row.Sum {
		return nil, fmt.Errorf("cardset: %s: checksum %s, manifest says %s", path, sum, row.Sum)
	}
	if pd.Name != name {
		return nil, fmt.Errorf("cardset: %s: holds pool %q", path, pd.Name)
	}
	s := NewDataSet(pd)
	loadedSet[path] = s
	return s, nil
}
