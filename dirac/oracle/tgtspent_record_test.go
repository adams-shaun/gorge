package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// tgtSpentDeck is a blue deck of Unravel and the supported blue Warp spells:
// a Warp cast spends less than the card's mana value, so it is the target
// Unravel's gate reads.
func tgtSpentDeck(t *testing.T, set cardset.Set) []string {
	t.Helper()
	deck := []string{"Unravel", "Unravel", "Unravel"}
	for _, n := range []string{
		"Sinister Cryologist", "Starfield Vocalist", "Starbreach Whale", "Codecracker Hound",
		"Anticausal Vestige", "Mechanozoa", "Starwinder",
	} {
		i, ok := set.Lookup(n)
		if !ok || !set.Supported()[i] {
			continue
		}
		for k := 0; k < 3; k++ {
			deck = append(deck, n)
		}
	}
	for len(deck) < 40 {
		deck = append(deck, "Island")
	}
	return deck[:40]
}

// tgtSpentHit reports an Unravel on the stack over a spell cast for less than
// its mana value: the game reached the snapshot the repro exists for.
func tgtSpentHit(g *GorgeSide) bool {
	st := g.E.G
	for i, id := range st.Stack {
		o := st.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Unravel" {
			continue
		}
		for _, below := range st.Stack[:i] {
			t := st.Obj(below)
			if t == nil || t.Face() == nil || t.ManaSpent <= 0 {
				continue
			}
			if t.ManaSpent < state.SpellManaValue(st, t) {
				return true
			}
		}
	}
	return false
}

// TestUnravelTargetSpentReproReaches replays the fixture and requires it to
// reach Unravel over a spell cast for less than its mana value: the lockstep
// entry alone would accept a script that never cast the spell.
func TestUnravelTargetSpentReproReaches(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-eoe")
	r, err := ReadRepro(filepath.Join("testdata", "repro", "unravel_target_spent_snapshot.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
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
	hit := false
	rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
		hit = hit || tgtSpentHit(ref)
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
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !hit {
		t.Fatal("the recorded game never cast Unravel over a discounted spell")
	}
}

func TestTgtSpentRecord(t *testing.T) {
	if os.Getenv("TGTSPENT_RECORD") == "" {
		return
	}
	reg := synthReg(t)
	set := cardset.AllowUnverified(cardsettest.Synth(t, "std-eoe"))
	deck := tgtSpentDeck(t, set)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"tgtspent_0", "tgtspent_1"},
			Cards: [2][]string{deck, deck}, Seed: seed, PassElision: false, Surface: "autopay", Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{GorgeAutopaySurface: true})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		hit := false
		rc := r.RunConfig()
		rc.MaxDecisions = 4000
		res := Lockstep(m, seed, k, observed(ExplorePolicy(seed), tgtSpentHit, &hit), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !hit {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", "unravel_target_spent_snapshot.repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatal("no seed reached Unravel over a discounted spell")
}
