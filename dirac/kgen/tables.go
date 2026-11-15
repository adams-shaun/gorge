package kgen

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// tableOf renders a pool as the data a binary table holds. It returns the pool's
// coverage and its CardID index under prev, which the caller must compare with
// prev's bytes.
func tableOf(p *pool, poolName string, prev []idEntry) (*cardset.PoolData, coverage, []idEntry, error) {
	pb, err := buildForOutput(p, prev)
	if err != nil {
		return nil, coverage{}, nil, err
	}
	if !validTableName(poolName) {
		return nil, pb.cv, nil, fmt.Errorf("pool name %q must be letters, digits, underscores and hyphens only", poolName)
	}
	all, bs, pt := pb.all, pb.bs, pb.pt
	if len(pt.subtypes) > cardset.PoolMaxSubtypes {
		return nil, pb.cv, nil, fmt.Errorf("refusing the pool: %s %d subtypes, a pool table holds %d", tableLimitMark, len(pt.subtypes), cardset.PoolMaxSubtypes)
	}
	sup, err := embeddedSupport()
	if err != nil {
		return nil, pb.cv, nil, err
	}
	d := &cardset.PoolData{Name: poolName, NumCards: len(pb.lw.cards), NumTokens: len(pb.lw.tokens), NumIDs: len(pb.idx)}
	inPool := map[int]bool{}
	for _, id := range pb.ids {
		inPool[id] = true
		d.PoolID = append(d.PoolID, uint16(id))
	}
	for _, e := range pb.idx {
		if !inPool[e.id] {
			d.Absent = append(d.Absent, cardset.AbsentID{ID: uint16(e.id), Name: e.name, Token: e.token})
		}
	}
	d.Defs = make([]kop.CardDef, len(all))
	d.CardSubs = make([]cardset.PoolSubMask, len(all))
	d.CardPrinted = make([][]uint8, len(all))
	d.CardTokens = make([][]uint16, len(all))
	d.CardRefSubs = make([][]uint8, len(all))
	d.AbilityOrigin = make([][]uint8, len(all))
	d.StaticSubs = make([][]cardset.AbSubs, len(all))
	for i, c := range all {
		d.Names = append(d.Names, c.Name)
		st, why := statusOf(c, sup)
		d.Status = append(d.Status, st)
		d.Reason = append(d.Reason, why)
		d.Defs[i] = bs[i].def
		d.Defs[i].Name = "" // names live in the name table only
		for _, s := range bs[i].subs {
			d.CardSubs[i] = d.CardSubs[i].With(int(s))
		}
		seen := map[int]bool{}
		for _, t := range bs[i].tokens {
			if !seen[t] {
				seen[t] = true
				d.CardTokens[i] = append(d.CardTokens[i], uint16(t))
			}
		}
		if len(bs[i].printed) > 0 {
			d.CardPrinted[i] = bs[i].printed
		}
		if len(bs[i].refSubs) > 0 {
			d.CardRefSubs[i] = bs[i].refSubs
		}
		if len(bs[i].def.Abilities) > 0 && len(bs[i].origins) > 0 {
			d.AbilityOrigin[i] = bs[i].origins
		}
		for ai, add := range bs[i].stAddSub {
			if len(add) > 0 {
				d.StaticSubs[i] = append(d.StaticSubs[i], cardset.AbSubs{Ab: ai, Subs: add})
			}
		}
	}
	d.CreatureTypes = append(d.CreatureTypes, effects.CreatureTypeWordList()...)
	d.OverrideSubs = make([]cardset.OvSubs, len(pt.ovSubs))
	for i, o := range pt.ovSubs {
		if len(o.set) > 0 {
			d.OverrideSubs[i].Set = o.set
		}
		if len(o.add) > 0 {
			d.OverrideSubs[i].Add = o.add
		}
		if len(o.keep) > 0 {
			d.OverrideSubs[i].Keep = o.keep
		}
	}
	d.Filters = nonNil(pt.filters)
	d.Exprs = nonNil(pt.exprs)
	d.Conds = nonNil(pt.conds)
	d.Overrides = nonNil(pt.overrides)
	d.Emblems = nonNil(pt.emblems)
	d.Subtypes = append([]string{}, pt.subtypes...)
	for i, c := range pb.cls {
		if c.Any != 0 {
			d.LayerCls = append(d.LayerCls, cardset.LayerEntry{I: i, C: c})
		}
	}
	d.LayerConflicts = nonNil(layerConflicts)
	d.RoomCards, d.NameReaders, d.AbReducers, d.AbReducerFloor, d.AbReducerTypes, d.AbManaCosted, d.AbManaCostedMV, d.AbManaCostedTypes = guardLists(bs, pt.filters)
	return d, pb.cv, pb.idx, nil
}

// tableLimitMark starts a refusal that a smaller list can pass.
const tableLimitMark = "table limit:"

// validTableName is validPoolName plus the hyphen: a table is a file, not a Go
// package, and the synthetic pools are named std-<set>.
func validTableName(name string) bool {
	return validPoolName(strings.ReplaceAll(name, "-", "_"))
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// tableFile is one output file of a table build.
type tableFile struct {
	name string
	data []byte
}

// idsChange is an ids.tsv a build needs rewritten.
type idsChange struct {
	pool string
	data []byte
}

// tableBuild is a whole table build, in memory: the key directory's files in a
// fixed order, or the ids.tsv files to rewrite first.
type tableBuild struct {
	files  []tableFile
	tables int
	bytes  int // the .ktab files alone
	ids    []idsChange
}

// poolsDir holds each pool's cards.txt and ids.tsv in the source tree.
const poolsDir = "dirac/cardset/pools"

// fixturesDir holds the pools only tests play. A pool's name is its directory's
// base name, unique over both levels.
const fixturesDir = poolsDir + "/fixtures"

// poolDir is pool name's directory in src.
func poolDir(src fs.FS, name string) string {
	if _, err := fs.Stat(src, path.Join(fixturesDir, name, "cards.txt")); err == nil {
		return path.Join(fixturesDir, name)
	}
	return path.Join(poolsDir, name)
}

// readPool reads pool name's card list and CardID index from src and loads its
// cards from the corpus. It returns the index's bytes and their parse.
func readPool(src fs.FS, name, cardsDir string) (*pool, []byte, []idEntry, error) {
	dir := poolDir(src, name)
	list := path.Join(dir, "cards.txt")
	lb, err := fs.ReadFile(src, list)
	if err != nil {
		return nil, nil, nil, err
	}
	names := parseCardList(lb)
	if len(names) == 0 {
		return nil, nil, nil, fmt.Errorf("%s: no cards in %s", name, list)
	}
	p, err := loadCardNames(names, cardsDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	idsPath := path.Join(dir, idsFile)
	oldIDs, err := fs.ReadFile(src, idsPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil, err
	}
	prev, err := parseIDs(oldIDs, idsPath)
	if err != nil {
		return nil, nil, nil, err
	}
	return p, oldIDs, prev, nil
}

// buildTables lowers every pool listed in src, in sorted order, in this process,
// serially, against the corpus at cardsDir. It writes nothing. src is the source
// tree (gorge.TableSources): a build never reads pool inputs from the working
// tree, so a stale binary cannot publish a newer tree's pools under its old key.
func buildTables(src fs.FS, cardsDir, key string) (*tableBuild, error) {
	lists, err := fs.Glob(src, path.Join(poolsDir, "*", "cards.txt"))
	if err != nil {
		return nil, err
	}
	if len(lists) == 0 {
		return nil, fmt.Errorf("no pool lists under %s", poolsDir)
	}
	fixtures, err := fs.Glob(src, path.Join(fixturesDir, "*", "cards.txt"))
	if err != nil {
		return nil, err
	}
	lists = append(lists, fixtures...)
	sort.Slice(lists, func(i, j int) bool { return path.Base(path.Dir(lists[i])) < path.Base(path.Dir(lists[j])) })
	out := &tableBuild{}
	var rows []cardset.ManifestRow
	var coverages []tableFile
	for _, list := range lists {
		name := path.Base(path.Dir(list))
		p, oldIDs, prev, err := readPool(src, name, cardsDir)
		if err != nil {
			return nil, err
		}
		d, cv, idx, err := tableOf(p, name, prev)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if newIDs := idsBytes(idx); !bytes.Equal(newIDs, oldIDs) {
			out.ids = append(out.ids, idsChange{pool: name, data: newIDs})
			continue
		}
		table := cardset.EncodeTable(d, key)
		sum, err := cardset.TableSum(table)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		rows = append(rows, cardset.ManifestRow{Pool: name, NumCards: d.NumCards, NumTokens: d.NumTokens, Sum: sum})
		var md bytes.Buffer
		writeCoverage(&md, name, cv)
		out.files = append(out.files, tableFile{name + ".ktab", table})
		coverages = append(coverages, tableFile{name + ".coverage.md", md.Bytes()})
		out.tables++
		out.bytes += len(table)
	}
	out.files = append(out.files, coverages...)
	out.files = append(out.files, tableFile{cardset.ManifestFile, cardset.EncodeManifest(rows)})
	return out, nil
}

// tablesComplete reports whether dir is a whole key directory.
func tablesComplete(dir string) bool {
	rows, err := cardset.ReadManifest(dir)
	if err != nil || len(rows) == 0 {
		return false
	}
	for _, r := range rows {
		for _, f := range []string{r.Pool + ".ktab", r.Pool + ".coverage.md"} {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				return false
			}
		}
	}
	return true
}

// keepKeyDirs is how many key directories publishing leaves behind: about 320 MB.
// The corpus is shared across worktrees, and one editing kgen publishes a key per
// build, so a small number removes a key another worktree is still reading.
const keepKeyDirs = 64

// staleTempAge is when a temp directory is taken to be a dead build's, not a peer's.
const staleTempAge = time.Hour

// publishTables puts files at <root>/<key> atomically: into a temp directory, then
// a rename. If a peer published first, its bytes are equal (the key fixes them),
// so the temp is dropped. Then it prunes.
func publishTables(root, key string, files []tableFile) (string, error) {
	final := filepath.Join(root, key)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", final, os.Getpid())
	if err := os.RemoveAll(tmp); err != nil {
		return "", err
	}
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmp, f.name), f.data, 0o644); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, final); err != nil && !tablesComplete(final) {
		return "", err
	}
	return final, pruneTables(root, key)
}

// pruneTables keeps the keepKeyDirs newest key directories by mtime and never keep,
// the one just published. kgen -tables refreshes a key's mtime, so a key rebuilt
// on demand stays. A pruned key fails with the rebuild message; a run pinned for
// longer sets DIRAC_TABLES. The loader never touches an mtime.
func pruneTables(root, keep string) error {
	ents, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	type keyDir struct {
		name string
		mod  time.Time
	}
	var keys []keyDir
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue // a peer removed it
		}
		if strings.Contains(e.Name(), ".tmp.") {
			if time.Since(fi.ModTime()) > staleTempAge {
				if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
					return err
				}
			}
			continue
		}
		if e.Name() != keep {
			keys = append(keys, keyDir{e.Name(), fi.ModTime()})
		}
	}
	slices.SortStableFunc(keys, func(a, b keyDir) int { return b.mod.Compare(a.mod) })
	if len(keys) < keepKeyDirs {
		return nil
	}
	for _, k := range keys[keepKeyDirs-1:] {
		if err := os.RemoveAll(filepath.Join(root, k.name)); err != nil {
			return err
		}
	}
	return nil
}

// refreshKey marks a key directory as wanted, so a peer's prune keeps it longest.
// A read-only corpus can't record that; the tables are still good.
func refreshKey(dir string) {
	now := time.Now()
	_ = os.Chtimes(dir, now, now)
}

// moduleRoot finds the module above the cwd, for writing an ids.tsv.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && bytes.HasPrefix(b, []byte("module github.com/adams-shaun/gorge\n")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside the module, so an ids.tsv cannot be written")
		}
		dir = parent
	}
}

// writeIDs rewrites the changed ids.tsv files under root. A file that is not what
// this binary embeds means the binary is older than the tree, and writing would
// undo someone's change.
func writeIDs(root string, src fs.FS, changes []idsChange) error {
	for _, c := range changes {
		rel := path.Join(poolDir(src, c.pool), idsFile)
		onDisk, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		embedded, err := fs.ReadFile(src, rel)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if !bytes.Equal(onDisk, embedded) {
			return fmt.Errorf("%s differs from this kgen binary's copy: rebuild kgen and rerun", rel)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), c.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Exit statuses of -tables: 0 published or up to date, 1 failed, 3 an ids.tsv was
// written and kgen must be rebuilt.
const tablesExitIDsChanged = 3

// runTables is kgen -tables. cards is the -cards flag when given, else the corpus
// is located the way the loader locates it.
func runTables(cards string, stdout, stderr io.Writer) int {
	fail := func(err error) int {
		fmt.Fprintln(stderr, "kgen:", err)
		return 1
	}
	if cards == "" {
		var err error
		if cards, err = cardset.CorpusDir(); err != nil {
			return fail(err)
		}
	}
	key, err := cardset.KeyFor(cards)
	if err != nil {
		return fail(err)
	}
	resolved, err := filepath.EvalSymlinks(cards)
	if err != nil {
		return fail(err)
	}
	root := filepath.Join(resolved, cardset.TablesSubdir)
	dir := filepath.Join(root, key)
	if tablesComplete(dir) {
		refreshKey(dir)
		fmt.Fprintf(stdout, "kgen: tables up to date: key %s in %s\n", key, dir)
		return 0
	}
	src := gorge.TableSources()
	tb, err := buildTables(src, cards, key)
	if err != nil {
		return fail(err)
	}
	if len(tb.ids) > 0 {
		root, err := moduleRoot()
		if err != nil {
			return fail(err)
		}
		if err := writeIDs(root, src, tb.ids); err != nil {
			return fail(err)
		}
		pools := make([]string, len(tb.ids))
		for i, c := range tb.ids {
			pools[i] = c.pool
		}
		fmt.Fprintf(stderr, "kgen: rewrote ids.tsv for %v: this binary embeds the old bytes, rebuild kgen and rerun (make dirac-tables does)\n", pools)
		return tablesExitIDsChanged
	}
	if _, err := publishTables(root, key, tb.files); err != nil {
		return fail(err)
	}
	fmt.Fprintf(stdout, "kgen: %d tables, %d bytes, key %s in %s\n", tb.tables, tb.bytes, key, dir)
	return 0
}
