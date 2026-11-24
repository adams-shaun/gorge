package lockstep

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/diff"
	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cardc/slice"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const ledgerWorkersTest = 4

var (
	proofOnce  sync.Once
	proofReg   *gorgebe.Registry
	proofErr   error
	proofForge *cards.Registry
	proofRoot  string
)

// proof builds the proof registry once per package, skipping without a corpus.
func proof(t *testing.T) (*gorgebe.Registry, *cards.Registry, string) {
	t.Helper()
	forge := testutil.CorpusRegistry(t)
	proofOnce.Do(func() {
		proofRoot, proofErr = mtgish.ModuleRoot()
		if proofErr != nil {
			return
		}
		proofForge = forge
		proofReg, proofErr = slice.Build(proofRoot, forge, "fdn", "legacy")
	})
	if errors.Is(proofErr, slice.ErrNoData) {
		t.Skip(mtgish.SkipMessage)
	}
	if proofErr != nil {
		t.Fatal(proofErr)
	}
	return proofReg, proofForge, proofRoot
}

func ledgerPaths(root string) (dir, un, counts string) {
	td := filepath.Join(root, "engine", "cardc", "testdata")
	return filepath.Join(td, "ledger"), filepath.Join(td, "ledger-unattributed.tsv"), filepath.Join(td, "ledger-counts.tsv")
}

func readLedgerFiles(t *testing.T) ([]Row, []Unattributed) {
	t.Helper()
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir, unPath, _ := ledgerPaths(root)
	rows, err := ReadLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	un, err := ReadUnattributed(unPath)
	if err != nil {
		t.Fatal(err)
	}
	return rows, un
}

func TestBlame(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	forest, ok := forge.Lookup("Forest")
	if !ok {
		t.Fatal("no Forest")
	}
	bear := func(pt string) *cards.Card {
		c, diags := gorgebe.ParseScript("zz/bear.txt", []byte("Name:Zz Bear\nManaCost:G\nTypes:Creature Bear\nPT:"+pt+"\n"))
		if c == nil || len(diags) > 0 {
			t.Fatalf("%v", diags)
		}
		if err := gorgebe.Bind([]*cards.Card{c}); err != nil {
			t.Fatal(err)
		}
		return c
	}
	two, three := bear("2/2"), bear("3/3")
	var d []*cards.Card
	for i := 0; i < 40; i++ {
		d = append(d, forest)
	}
	for i := 0; i < 20; i++ {
		d = append(d, two)
	}
	g := Game{ID: "zz/bears", Cfg: rules.Config{Seed: 1, Names: []string{"A", "B"}, Decks: [][]*cards.Card{d, d},
		Tokens: forge.Tokens, NameUniverse: forge.Universe(), Mulligans: 1, LoopGuard: rules.WithDefaultObjectCap(nil)}}
	mapper := func(to *cards.Card) Swap {
		return func(d []*cards.Card) []*cards.Card {
			out := make([]*cards.Card, len(d))
			for i, c := range d {
				out[i] = c
				if c == two {
					out[i] = to
				}
			}
			return out
		}
	}
	for _, dir := range []Direction{Forward, Reverse} {
		if got := Blame(g, dir, mapper(three), newBot); got != "Zz Bear" {
			t.Errorf("%s 3/3: blamed %q", dir, got)
		}
		if got := Blame(g, dir, mapper(two), newBot); got != "" {
			t.Errorf("%s identity: blamed %q", dir, got)
		}
	}
}

func TestLedgerFormat(t *testing.T) {
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir, unPath, _ := ledgerPaths(root)
	rows, err := ReadLedger(dir) // checks names, stems, columns, issues parse
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, s := range []string{"fdn", "legacy"} {
		ns, err := mtgish.SliceNames(root, s)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range ns {
			names[n] = true
			if f, _, ok := strings.Cut(n, " // "); ok {
				names[f] = true
			}
		}
	}
	type id struct{ card, source, key string }
	seen := map[id]bool{}
	var prev Row
	lockstepCards := map[string]bool{}
	for _, r := range rows {
		where := fmt.Sprintf("%s %s %s", r.Card, r.Source, r.Key)
		if !names[r.Card] {
			t.Errorf("%s: card is in neither slice", where)
		}
		if r.Source != SourceStatic && r.Source != SourceLockstep {
			t.Errorf("%s: bad source", where)
		}
		if !ValidClass(r.Class) {
			t.Errorf("%s: bad class %q", where, r.Class)
		}
		if r.Issue != 0 && r.Class != "mtgish-bug" {
			t.Errorf("%s: issue %d on class %s", where, r.Issue, r.Class)
		}
		if len(r.Note) > MaxNote || r.Note == "" {
			t.Errorf("%s: note length %d", where, len(r.Note))
		}
		if r.Source == SourceLockstep {
			lockstepCards[r.Card] = true
			if r.Sig != "-" {
				t.Errorf("%s: lockstep sig %q", where, r.Sig)
			}
			if !strings.HasSuffix(r.Key, "/forward") && !strings.HasSuffix(r.Key, "/reverse") {
				t.Errorf("%s: bad witness", where)
			}
		}
		k := id{r.Card, r.Source, r.Key}
		if seen[k] {
			t.Errorf("%s: duplicate", where)
		}
		seen[k] = true
		if prev.Card == r.Card && rowLess(r, prev) {
			t.Errorf("%s: out of order", where)
		}
		prev = r
	}
	un, err := ReadUnattributed(unPath)
	if err != nil {
		t.Fatal(err)
	}
	for i, u := range un {
		if !ValidClass(u.Class) || u.Note == "" || len(u.Note) > MaxNote || (i > 0 && un[i-1].Witness >= u.Witness) {
			t.Errorf("unattributed %s: bad row", u.Witness)
		}
	}
}

func TestLedgerStatic(t *testing.T) {
	reg, _, _ := proof(t)
	rows, _ := readLedgerFiles(t)
	var ledger, live []Row
	for _, r := range rows {
		if r.Source == SourceStatic {
			ledger = append(ledger, r)
		}
	}
	for _, e := range reg.Entries() {
		if e.Origin != gorgebe.OriginCardc {
			continue
		}
		name := e.Forge.Faces[0].Name
		for _, r := range diff.Card(name, e.Card, e.Forge) {
			live = append(live, Row{Card: name, Source: SourceStatic, Key: r.Key(), Sig: r.Sig()})
		}
	}
	missing, stale := Hold(live, ledger)
	for _, r := range missing {
		t.Errorf("missing: %s %s sig %s", r.Card, r.Key, r.Sig)
	}
	for _, r := range stale {
		t.Errorf("stale: %s %s sig %s", r.Card, r.Key, r.Sig)
	}
}

// holdChunk sweeps games and holds the findings against the ledger (not the stale side: a chunk is partial).
func holdChunk(t *testing.T, games []Game) {
	t.Helper()
	reg, _, _ := proof(t)
	rows, un := readLedgerFiles(t)
	found := Sweep(games, []Direction{Forward, Reverse}, reg.Deck, newBot, ledgerWorkersTest)
	missing, _, missingUn, _ := HoldFound(found, rows, un, false)
	for _, r := range missing {
		t.Errorf("blamed card %q has no lockstep row (witness %s)", r.Card, r.Key)
	}
	for _, u := range missingUn {
		t.Errorf("unattributed witness %s is not in the register", u.Witness)
	}
}

func fdnChunk(t *testing.T, k int) {
	_, forge, root := proof(t)
	games, err := FDN(root, forge, k*100, 100)
	if err != nil {
		t.Fatal(err)
	}
	holdChunk(t, games)
}

func legacyGames(t *testing.T, seats ...int) []Game {
	_, forge, _ := proof(t)
	var games []Game
	for _, s := range seats {
		names, decks, err := testutil.AcceptanceDecks(forge, s)
		if err != nil {
			t.Fatal(err)
		}
		games = append(games, Legacy(forge, names, decks))
	}
	return games
}

func TestLedgerLockstepFDN00(t *testing.T) { fdnChunk(t, 0) }
func TestLedgerLockstepFDN01(t *testing.T) { fdnChunk(t, 1) }
func TestLedgerLockstepFDN02(t *testing.T) { fdnChunk(t, 2) }
func TestLedgerLockstepFDN03(t *testing.T) { fdnChunk(t, 3) }
func TestLedgerLockstepFDN04(t *testing.T) { fdnChunk(t, 4) }
func TestLedgerLockstepFDN05(t *testing.T) { fdnChunk(t, 5) }
func TestLedgerLockstepFDN06(t *testing.T) { fdnChunk(t, 6) }
func TestLedgerLockstepFDN07(t *testing.T) { fdnChunk(t, 7) }
func TestLedgerLockstepFDN08(t *testing.T) { fdnChunk(t, 8) }
func TestLedgerLockstepFDN09(t *testing.T) { fdnChunk(t, 9) }
func TestLedgerLockstepFDN10(t *testing.T) { fdnChunk(t, 10) }
func TestLedgerLockstepFDN11(t *testing.T) { fdnChunk(t, 11) }
func TestLedgerLockstepFDN12(t *testing.T) { fdnChunk(t, 12) }
func TestLedgerLockstepFDN13(t *testing.T) { fdnChunk(t, 13) }
func TestLedgerLockstepFDN14(t *testing.T) { fdnChunk(t, 14) }

func TestLedgerLockstepLegacy(t *testing.T) { holdChunk(t, legacyGames(t, 2, 4, 6, 8)) }

func TestLedgerWitnessesLive(t *testing.T) {
	reg, forge, root := proof(t)
	rows, un := readLedgerFiles(t)
	games := map[string]Game{}
	get := func(id string) (Game, bool) {
		if g, ok := games[id]; ok {
			return g, true
		}
		var g Game
		switch {
		case strings.HasPrefix(id, "fdn/"):
			var n int
			if _, err := fmt.Sscanf(id, "fdn/%d", &n); err != nil {
				return g, false
			}
			gs, err := FDN(root, forge, n, 1)
			if err != nil || len(gs) != 1 {
				return g, false
			}
			g = gs[0]
		case strings.HasPrefix(id, "legacy/"):
			var s int
			if _, err := fmt.Sscanf(id, "legacy/%d", &s); err != nil {
				return g, false
			}
			names, decks, err := testutil.AcceptanceDecks(forge, s)
			if err != nil {
				return g, false
			}
			g = Legacy(forge, names, decks)
		default:
			return g, false
		}
		games[id] = g
		return g, true
	}
	type w struct{ witness, card string }
	var ws []w
	for _, r := range rows {
		if r.Source == SourceLockstep {
			ws = append(ws, w{r.Key, r.Card})
		}
	}
	for _, u := range un {
		ws = append(ws, w{u.Witness, ""})
	}
	for _, x := range ws { // load games serially: the cache is not shared
		if _, ok := get(x.witness[:strings.LastIndex(x.witness, "/")]); !ok {
			t.Errorf("witness %s: no such game", x.witness)
		}
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, ledgerWorkersTest)
	var mu sync.Mutex
	for _, x := range ws {
		i := strings.LastIndex(x.witness, "/")
		g, ok := games[x.witness[:i]]
		if !ok {
			continue
		}
		var dir Direction
		switch x.witness[i+1:] {
		case "forward":
			dir = Forward
		case "reverse":
			dir = Reverse
		default:
			t.Errorf("witness %s: bad direction", x.witness)
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			var msg string
			if r := Run(g, dir, reg.Deck, newBot); r.Outcome == "clean" {
				msg = fmt.Sprintf("witness %s is clean", x.witness)
			} else if got := Blame(g, dir, reg.Deck, newBot); got != x.card {
				msg = fmt.Sprintf("witness %s blames %q, ledger says %q", x.witness, got, x.card)
			}
			if msg != "" {
				mu.Lock()
				t.Error(msg)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
}

func TestLedgerCounts(t *testing.T) {
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	_, _, countsPath := ledgerPaths(root)
	want, err := os.ReadFile(countsPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, un := readLedgerFiles(t)
	if got := FormatCounts(Counts(rows, un)); got != string(want) {
		t.Errorf("ledger counts differ from ledger-counts.tsv\ngot:\n%swant:\n%s", got, want)
	}
}
