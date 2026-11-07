package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kgen"
)

// A printed Delve card outside the Standard set lists has no synthetic pool:
// printedDelveSet builds the kernel table its repro's decks need. The table is
// a test artifact, never committed (kgen's tables are corpus-derived).
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
// from its own decks for a printed-delve card no Standard set holds.
func reproSet(t *testing.T, pool string, r *Repro) cardset.Set {
	t.Helper()
	if pool != "custom" {
		return cardsettest.Synth(t, pool)
	}
	return printedDelveSet(t, r.Cards)
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
