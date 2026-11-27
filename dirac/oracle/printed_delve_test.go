package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kgen"
)

// delveTables memoizes printedDelveSet by corpus and card set (11-27, the oracle
// lane budget): a test's table depends only on the corpus it lowers from and the
// sorted cards its decks hold, and the package builds one for tens of recordings
// whose decks are the same - TestRefusedEntriesStillDiverge rebuilt the table of
// each custom entry's decks with kgen on every run of that entry. The build is
// deterministic, so sharing one table across tests cannot move a verdict; the
// Set itself is read-only to callers (only cardset.Raw/AllowUnverified wrap it,
// and each wrapper derives from the same PoolData).
var delveTables sync.Map

// printedDelveSet builds (or shares) the kernel table a repro's decks need: a
// card no synthetic pool holds (a printed Delve card outside the Standard set
// lists, say) has no table of its own. The table is a test artifact, never
// committed (kgen's tables are corpus-derived).
func printedDelveSet(t *testing.T, cards [2][]string) cardset.Set {
	t.Helper()
	var names []string
	for _, d := range cards {
		names = append(names, d...)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	key := cardsDir + "\x00" + strings.Join(names, "\x00")
	if v, ok := delveTables.Load(key); ok {
		return v.(cardset.Set)
	}
	set := buildPrintedDelveSet(t, cardsDir, names)
	delveTables.Store(key, set)
	return set
}

// buildPrintedDelveSet lowers the cards with kgen and loads the table they
// build: printedDelveSet's once-per-(corpus, cards) build.
func buildPrintedDelveSet(t *testing.T, cardsDir string, names []string) cardset.Set {
	t.Helper()
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "printed-delve", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "printed-delve.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	return cardset.AllowUnverified(cardset.NewDataSet(pd))
}

// reproSet is the set a repro plays: its synthetic pool, or the table built
// from its own decks for a card no synthetic pool holds. A custom table plays
// every lowered card, refused ones included (cardset.Raw): a recording replays
// the game it recorded, so a card refused after the recording (11-16: Maze of
// Ith, dealt in unless_offer_counts_restricted_mana's decks) cannot fail an
// unrelated repro. Whether a refused card still diverges is its own refusal
// repro's question (checkRefusedRepro).
func reproSet(t *testing.T, pool string, r *Repro) cardset.Set {
	t.Helper()
	if pool != "custom" {
		return cardsettest.Synth(t, pool)
	}
	return cardset.Raw(printedDelveSet(t, r.Cards))
}

// TestPrintedDelveReprosLockstep replays the two printed-delve repros on the
// table their decks need and requires each to reach the delve ask and pick a
// graveyard card: the lockstep entry alone would accept a script that skipped
// the cast.
func TestPrintedDelveReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range []struct{ file, card string }{
		{"printed_delve_gurmag_angler", "Gurmag Angler"},
		{"printed_delve_murderous_cut", "Murderous Cut"},
	} {
		t.Run(c.card, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.file+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := printedDelveSet(t, r.Cards)
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			entries := func(names []string) []cardset.Entry {
				es := make([]cardset.Entry, len(names))
				for i, n := range names {
					es[i] = cardset.Entry{Name: n, Count: 1}
				}
				return es
			}
			tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions = len(r.Actions) + 200
			sawDelve := false
			inner := rc.Check
			rc.Check = func(ref *GorgeSide, ker Side) *Divergence {
				if d := ref.Pending(); d != nil && d.Prompt == "Delve: exile cards from your graveyard to help cast "+c.card {
					sawDelve = true
				}
				if inner != nil {
					return inner(ref, ker)
				}
				return nil
			}
			k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			script := &ScriptPolicy{Actions: r.Actions}
			res := Lockstep(m, r.Seed, k, script, rc)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !sawDelve {
				t.Fatalf("the recorded game never posed the delve ask for %s", c.card)
			}
			code, ok := chooseCode("exile")
			if !ok {
				t.Fatal("no choose:exile code")
			}
			picked := 0
			for _, a := range res.Actions {
				if a.Kind() == ActChoose+CanonKind(code) {
					picked++
				}
			}
			if picked == 0 {
				t.Fatalf("the recorded game picked no graveyard card to exile")
			}
			t.Logf("%s: %d delve pick(s), seed %d", c.card, picked, r.Seed)
		})
	}
}
