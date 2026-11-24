package lockstep

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Classes lists the ledger's divergence classes in count-file order.
var Classes = []string{"mtgish-bug", "cardc-gap", "forge-bug", "equivalent"}

// Sources of a ledger row.
const (
	SourceStatic   = "static"
	SourceLockstep = "lockstep"
)

// ledgerHeader is every ledger file's column line.
const ledgerHeader = "source\tkey\tsig\tclass\tissue\tnote"

// UnattributedHeader is the column line of the unattributed register.
const UnattributedHeader = "witness\tclass\tnote"

// MaxNote is the longest note, in bytes.
const MaxNote = 100

// Row is one ledger row of one card.
type Row struct {
	Card, Source, Key, Sig, Class string
	Issue                         int
	Note                          string
}

// ValidClass reports whether c is a ledger class.
func ValidClass(c string) bool {
	for _, k := range Classes {
		if k == c {
			return true
		}
	}
	return false
}

func rowLess(a, b Row) bool {
	if a.Source != b.Source {
		return a.Source < b.Source
	}
	return a.Key < b.Key
}

// ReadLedger reads every card file under dir; a malformed one is an error naming it.
// A missing directory is an empty ledger. Rows come out by file name, then in file order.
func ReadLedger(dir string) ([]Row, error) {
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tsv") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		rs, err := readCard(p, strings.TrimSuffix(e.Name(), ".tsv"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		rows = append(rows, rs...)
	}
	return rows, nil
}

func readCard(path, stem string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	if !sc.Scan() {
		return nil, fmt.Errorf("empty file")
	}
	name, ok := strings.CutPrefix(sc.Text(), "# ")
	if !ok || name == "" {
		return nil, fmt.Errorf("line 1: want \"# <card name>\"")
	}
	if gorgebe.Stem(name) != stem {
		return nil, fmt.Errorf("card %q has stem %q, not %q", name, gorgebe.Stem(name), stem)
	}
	if !sc.Scan() || sc.Text() != ledgerHeader {
		return nil, fmt.Errorf("line 2: want the column line")
	}
	var rows []Row
	for n := 3; sc.Scan(); n++ {
		f := strings.Split(sc.Text(), "\t")
		if len(f) != 6 {
			return nil, fmt.Errorf("line %d: want 6 fields, got %d", n, len(f))
		}
		issue, err := strconv.Atoi(f[4])
		if err != nil || issue < 0 {
			return nil, fmt.Errorf("line %d: bad issue %q", n, f[4])
		}
		rows = append(rows, Row{Card: name, Source: f[0], Key: f[1], Sig: f[2], Class: f[3], Issue: issue, Note: f[5]})
	}
	return rows, sc.Err()
}

// WriteLedger writes one file per card of rows (sorted by source then key) and removes the
// files of cards with no rows.
func WriteLedger(dir string, rows []Row) error {
	by := map[string][]Row{}
	for _, r := range rows {
		by[r.Card] = append(by[r.Card], r)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	want := map[string]bool{}
	for card, rs := range by {
		sort.SliceStable(rs, func(i, j int) bool { return rowLess(rs[i], rs[j]) })
		var b strings.Builder
		b.WriteString("# " + card + "\n" + ledgerHeader + "\n")
		for _, r := range rs {
			fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\n", r.Source, r.Key, r.Sig, r.Class, r.Issue, r.Note)
		}
		name := gorgebe.Stem(card) + ".tsv"
		want[name] = true
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tsv") && !want[e.Name()] {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// Hold returns the live rows with no ledger row (missing) and the ledger rows with no live row (stale),
// matched on (Card, Source, Key) and, for static rows, Sig.
func Hold(live, ledger []Row) (missing, stale []Row) {
	type id struct{ card, source, key string }
	match := func(a, b Row) bool { return a.Source != SourceStatic || a.Sig == b.Sig }
	lr := map[id]Row{}
	for _, r := range ledger {
		lr[id{r.Card, r.Source, r.Key}] = r
	}
	seen := map[id]Row{}
	for _, r := range live {
		k := id{r.Card, r.Source, r.Key}
		seen[k] = r
		if l, ok := lr[k]; !ok || !match(r, l) {
			missing = append(missing, r)
		}
	}
	for _, r := range ledger {
		if l, ok := seen[id{r.Card, r.Source, r.Key}]; !ok || !match(l, r) {
			stale = append(stale, r)
		}
	}
	return missing, stale
}

// Unattributed is one two-card divergence: a non-clean game no single card explains.
type Unattributed struct{ Witness, Class, Note string }

// ReadUnattributed reads the register at path; a missing file is an empty register.
func ReadUnattributed(path string) ([]Unattributed, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || sc.Text() != UnattributedHeader {
		return nil, fmt.Errorf("%s: line 1: want the column line", path)
	}
	var out []Unattributed
	for n := 2; sc.Scan(); n++ {
		f := strings.Split(sc.Text(), "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("%s: line %d: want 3 fields", path, n)
		}
		out = append(out, Unattributed{f[0], f[1], f[2]})
	}
	return out, sc.Err()
}

// WriteUnattributed writes the register, sorted by witness.
func WriteUnattributed(path string, rows []Unattributed) error {
	rows = append([]Unattributed(nil), rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Witness < rows[j].Witness })
	var b strings.Builder
	b.WriteString(UnattributedHeader + "\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%s\t%s\t%s\n", r.Witness, r.Class, r.Note)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// Count is one class's line of the counts file.
type Count struct{ Rows, Cards int }

// Counts tallies rows and distinct cards per class; unattributed rows count as rows, not cards.
func Counts(rows []Row, un []Unattributed) map[string]Count {
	out := map[string]Count{}
	cardSet := map[string]map[string]bool{}
	for _, c := range Classes {
		out[c] = Count{}
		cardSet[c] = map[string]bool{}
	}
	for _, r := range rows {
		c := out[r.Class]
		c.Rows++
		cardSet[r.Class][r.Card] = true
		c.Cards = len(cardSet[r.Class])
		out[r.Class] = c
	}
	for _, u := range un {
		c := out[u.Class]
		c.Rows++
		out[u.Class] = c
	}
	return out
}

// FormatCounts is the counts file's text: one line per class, class<TAB>rows<TAB>cards.
func FormatCounts(m map[string]Count) string {
	var b strings.Builder
	for _, c := range Classes {
		fmt.Fprintf(&b, "%s\t%d\t%d\n", c, m[c].Rows, m[c].Cards)
	}
	return b.String()
}

// Found is one non-clean result and the card blamed for it ("" when none).
type Found struct {
	Witness string // "<game id>/<direction>"
	Card    string
	Outcome string
}

// Witness is the ledger's key of a game in a direction.
func Witness(id string, d Direction) string { return id + "/" + d.String() }

// Sweep runs every game in every direction on workers goroutines and blames each non-clean result.
// Results are returned in game order, forward before reverse, whatever the scheduling.
func Sweep(games []Game, dirs []Direction, swap Swap, newBot func(seed uint64) rules.Answerer, workers int) []Found {
	type job struct {
		g Game
		d Direction
	}
	var jobs []job
	for _, g := range games {
		for _, d := range dirs {
			jobs = append(jobs, job{g, d})
		}
	}
	res := make([]*Found, len(jobs))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				j := jobs[i]
				if r := Run(j.g, j.d, swap, newBot); r.Outcome != "clean" {
					res[i] = &Found{Witness: Witness(j.g.ID, j.d), Card: Blame(j.g, j.d, swap, newBot), Outcome: r.Outcome}
				}
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()
	var out []Found
	for _, f := range res {
		if f != nil {
			out = append(out, *f)
		}
	}
	return out
}

// HoldFound holds a sweep against the ledger. missing: a found witness whose card has no live-matching
// lockstep row (one witness per card, the first), or an unattributed witness not in the register.
// With full set, ledger lockstep rows and register rows whose witness is not in found are stale.
func HoldFound(found []Found, ledger []Row, un []Unattributed, full bool) (missing, stale []Row, missingUn, staleUn []Unattributed) {
	live := map[string]string{} // witness -> card
	for _, f := range found {
		live[f.Witness] = f.Card
	}
	has := map[string]bool{} // card has a ledger lockstep row
	for _, r := range ledger {
		if r.Source == SourceLockstep {
			has[r.Card] = true
		}
	}
	reg := map[string]bool{}
	for _, u := range un {
		reg[u.Witness] = true
	}
	done := map[string]bool{}
	for _, f := range found {
		if f.Card == "" {
			if !reg[f.Witness] {
				missingUn = append(missingUn, Unattributed{Witness: f.Witness})
			}
		} else if !has[f.Card] && !done[f.Card] {
			done[f.Card] = true
			missing = append(missing, Row{Card: f.Card, Source: SourceLockstep, Key: f.Witness, Sig: "-"})
		}
	}
	if full {
		for _, r := range ledger {
			if r.Source == SourceLockstep && live[r.Key] != r.Card {
				stale = append(stale, r)
			}
		}
		for _, u := range un {
			if c, ok := live[u.Witness]; !ok || c != "" {
				staleUn = append(staleUn, u)
			}
		}
	}
	return
}
