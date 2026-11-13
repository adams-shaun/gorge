package kgen

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The synthetic evidence pools' tables (dirac/cardset/synth.go). Their lists are
// derived here on every build, from the tracked set lists and the corpus: nothing
// about a synthetic pool is written to the source tree, so there is no ids.tsv and
// a CardID is simply the pool index.

const (
	synthStandardList = "dirac/cardset/standard.txt"
	synthDraftList    = "dirac/cardset/draftpools.txt"
	synthSetlistDir   = "dirac/cardset/setlists"
)

// synthList is one pool to build, before any split.
type synthList struct {
	name  string
	names []string
}

// frontNames resolves list names to the corpus's own (a card's front face), sorted
// and unique. A set list spells a Room or a split card "A // B", which is not what a
// deck or a matchup calls it.
func frontNames(names []string, cardsDir string) ([]string, error) {
	reg, err := cards.OpenCorpusSubset(cardsDir, names)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok || len(c.Faces) == 0 || c.Faces[0] == nil || c.Faces[0].Name == "" {
			return nil, fmt.Errorf("card %q not in the corpus", n)
		}
		out = append(out, c.Faces[0].Name)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// withBasics is names, which may already hold some basics, plus all of them, sorted.
func withBasics(names []string) []string {
	out := append(slices.Clone(names), cardset.SynthBasics[:]...)
	slices.Sort(out)
	return slices.Compact(out)
}

// synthStandardLists is one list per set code in standard.txt, without basics.
func synthStandardLists(src fs.FS, cardsDir string) ([]synthList, error) {
	return synthCodeLists(src, cardsDir, synthStandardList, "std-")
}

// synthDraftLists is one list per code in draftpools.txt, without basics: a draft
// pool's dealt names, played as dr-<code>.
func synthDraftLists(src fs.FS, cardsDir string) ([]synthList, error) {
	return synthCodeLists(src, cardsDir, synthDraftList, cardset.SynthDraftPrefix)
}

// synthCodeLists reads a file of list codes and resolves each code's set list.
func synthCodeLists(src fs.FS, cardsDir, codesFile, prefix string) ([]synthList, error) {
	b, err := fs.ReadFile(src, codesFile)
	if err != nil {
		return nil, err
	}
	var out []synthList
	for _, code := range parseCardList(b) {
		lb, err := fs.ReadFile(src, path.Join(synthSetlistDir, code+".txt"))
		if err != nil {
			return nil, err
		}
		names, err := frontNames(parseCardList(lb), cardsDir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", code, err)
		}
		out = append(out, synthList{name: prefix + strings.ToLower(code), names: names})
	}
	return out, nil
}

// synthTail splits the corpus outside the Standard lists and SynthBasics into the
// names kgen lowers, in tail-NN buckets, and the names it does not. Lowering is
// judged on the whole tail, the way a card's status is judged on a whole pool.
func synthTail(std []synthList, cardsDir string) (tail []synthList, unlowered []string, err error) {
	body, err := corpusAllNames(filepath.Join(cardsDir, "cardsfolder"))
	if err != nil {
		return nil, nil, err
	}
	inStd := map[string]bool{}
	for _, l := range std {
		for _, n := range l.names {
			inStd[n] = true
		}
	}
	for _, n := range cardset.SynthBasics {
		inStd[n] = true
	}
	var names []string
	for _, n := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
		if !inStd[n] {
			names = append(names, n)
		}
	}
	p, err := loadCardNames(names, cardsDir)
	if err != nil {
		return nil, nil, err
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		return nil, nil, err
	}
	var lowered []string
	for _, c := range lw.cards {
		if c.loweredOK() {
			lowered = append(lowered, c.Name)
		} else {
			unlowered = append(unlowered, c.Name)
		}
	}
	for i, b := range cardset.TailBuckets(lowered) {
		tail = append(tail, synthList{name: fmt.Sprintf("tail-%02d", i), names: b})
	}
	slices.Sort(unlowered)
	return tail, unlowered, nil
}

// isSizeRefusal reports whether a table build failed on a limit a smaller list can
// pass: a table's subtype masks.
func isSizeRefusal(err error) bool {
	return err != nil && strings.Contains(err.Error(), tableLimitMark)
}

// synthTable is one built table.
type synthTable struct {
	name string
	data *bytes.Buffer
	cv   coverage
	row  cardset.ManifestRow
}

// buildOneTable builds the table of names plus basics, under its final name.
func buildOneTable(name string, names []string, cardsDir, key string) (synthTable, error) {
	p, err := loadCardNames(withBasics(names), cardsDir)
	if err != nil {
		return synthTable{}, fmt.Errorf("%s: %w", name, err)
	}
	d, cv, _, err := tableOf(p, name, nil)
	if err != nil {
		return synthTable{}, fmt.Errorf("%s: %w", name, err)
	}
	table := cardset.EncodeTable(d, key)
	sum, err := cardset.TableSum(table)
	if err != nil {
		return synthTable{}, fmt.Errorf("%s: %w", name, err)
	}
	return synthTable{name: name, data: bytes.NewBuffer(table), cv: cv,
		row: cardset.ManifestRow{Pool: name, NumCards: d.NumCards, NumTokens: d.NumTokens, Sum: sum}}, nil
}

// buildSynthList builds a list's table. A list too big for a table splits in two by
// hash, into name-a and name-b, on the lowest hash bit whose two halves both build.
// Where no bit does, it splits on the lowest bit that divides the list and each half
// splits again (name-aa, name-ab). So the split is a pure function of the names and
// of what a table can hold, and a list is cut as few times as the hash allows.
func buildSynthList(l synthList, suffix string, cardsDir, key string) ([]synthTable, error) {
	join := func(sfx string) string {
		if sfx == "" {
			return l.name
		}
		return l.name + "-" + sfx
	}
	t, err := buildOneTable(join(suffix), l.names, cardsDir, key)
	if err == nil {
		return []synthTable{t}, nil
	}
	if !isSizeRefusal(err) {
		return nil, err
	}
	first := -1
	for bit := 0; bit < 32; bit++ {
		a, b := cardset.SplitByHash(l.names, bit)
		if len(a) == 0 || len(b) == 0 {
			continue
		}
		if first < 0 {
			first = bit
		}
		ta, ea := buildOneTable(join(suffix+"a"), a, cardsDir, key)
		if ea != nil && !isSizeRefusal(ea) {
			return nil, ea
		}
		tb, eb := buildOneTable(join(suffix+"b"), b, cardsDir, key)
		if eb != nil && !isSizeRefusal(eb) {
			return nil, eb
		}
		if ea == nil && eb == nil {
			return []synthTable{ta, tb}, nil
		}
	}
	if first < 0 {
		return nil, fmt.Errorf("%d cards with one hash do not fit a table: %w", len(l.names), err)
	}
	a, b := cardset.SplitByHash(l.names, first)
	ta, err := buildSynthList(synthList{l.name, a}, suffix+"a", cardsDir, key)
	if err != nil {
		return nil, err
	}
	tb, err := buildSynthList(synthList{l.name, b}, suffix+"b", cardsDir, key)
	if err != nil {
		return nil, err
	}
	return append(ta, tb...), nil
}

// buildSynthTables lowers every synthetic pool, serially, and returns the key
// directory's files in a fixed order. It writes nothing. src is the source tree, as
// in buildTables.
func buildSynthTables(src fs.FS, cardsDir, key string) (files []tableFile, tables, size int, err error) {
	std, err := synthStandardLists(src, cardsDir)
	if err != nil {
		return nil, 0, 0, err
	}
	draft, err := synthDraftLists(src, cardsDir)
	if err != nil {
		return nil, 0, 0, err
	}
	// The tail is the corpus outside the Standard lists only: a draft pool's
	// cards stay in their tail buckets too, so adding a draft pool moves no tail.
	tail, unlowered, err := synthTail(std, cardsDir)
	if err != nil {
		return nil, 0, 0, err
	}
	var built []synthTable
	for _, l := range append(append(slices.Clone(std), draft...), tail...) {
		ts, err := buildSynthList(l, "", cardsDir, key)
		if err != nil {
			return nil, 0, 0, err
		}
		built = append(built, ts...)
	}
	slices.SortFunc(built, func(a, b synthTable) int { return strings.Compare(a.name, b.name) })
	rows := make([]cardset.ManifestRow, len(built))
	var coverages []tableFile
	for i, t := range built {
		rows[i] = t.row
		files = append(files, tableFile{t.name + ".ktab", t.data.Bytes()})
		size += t.data.Len()
		var md bytes.Buffer
		writeCoverage(&md, t.name, t.cv)
		coverages = append(coverages, tableFile{t.name + ".coverage.md", md.Bytes()})
	}
	files = append(files, coverages...)
	var names bytes.Buffer
	for _, n := range unlowered {
		names.WriteString(n)
		names.WriteByte('\n')
	}
	files = append(files, tableFile{cardset.TailUnloweredFile, names.Bytes()})
	files = append(files, tableFile{cardset.ManifestFile, cardset.EncodeManifest(rows)})
	return files, len(built), size, nil
}

// runSynthPools is kgen -synth-pools. cards is the -cards flag when given, else the
// corpus is located the way the loader locates it.
func runSynthPools(cards string, stdout, stderr io.Writer) int {
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
		if errors.Is(err, cardset.ErrNoCorpus) {
			return fail(fmt.Errorf("%w: -synth-pools needs a corpus", err))
		}
		return fail(err)
	}
	resolved, err := filepath.EvalSymlinks(cards)
	if err != nil {
		return fail(err)
	}
	root := filepath.Join(resolved, cardset.SynthTablesSubdir)
	dir := filepath.Join(root, key)
	if tablesComplete(dir) {
		refreshKey(dir)
		fmt.Fprintf(stdout, "kgen: synthetic pools up to date: key %s in %s\n", key, dir)
		return 0
	}
	files, tables, size, err := buildSynthTables(gorge.TableSources(), cards, key)
	if err != nil {
		return fail(err)
	}
	if _, err := publishTables(root, key, files); err != nil {
		return fail(err)
	}
	fmt.Fprintf(stdout, "kgen: %d synthetic pools, %d bytes, key %s in %s\n", tables, size, key, dir)
	return 0
}
