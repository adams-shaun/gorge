package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestAtomDiffGoadedFocus: the IsGoaded atom read both ways over a focus pool
// around The Rani, whose Mark of the Rani token goads (a granted Goad$ True
// static) and whose trigger reads Creature.IsGoaded. No plain seed goads a
// creature, so the atom is pinned unproven there (atomdiff-unproven.txt); this
// seed reads it true and false and a divergence names it.
func TestAtomDiffGoadedFocus(t *testing.T) {
	const pool, focus = "tail-43", "The Rani"
	set := cardsettest.Synth(t, pool)
	if i, ok := set.Lookup(focus); !ok || set.Status(i) == cardset.StatusNotLowered {
		t.Skipf("%s does not lower %s", pool, focus)
	}
	reg := synthReg(t)
	decks, files, err := FocusPool(reg, set, pool, []FocusCard{{Name: focus}}, FocusPerDeck, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: reg, decks: map[string][]*cards.Card{}}
	c.AddDecks(files)
	var atom []FilterAtom
	fa, _ := RegisteredAtoms()
	for _, a := range fa {
		if a.Name == "fw:Card.IsGoaded" {
			atom = append(atom, a)
		}
	}
	if len(atom) != 1 {
		t.Fatalf("%d IsGoaded atoms registered, want 1", len(atom))
	}
	cfg := Config{GorgeAutopaySurface: true}
	total := NewAtomStats()
	games := 0
	for i := 0; i < 6; i++ {
		p := PairingAt(decks, i, 1)
		m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		k, err := SetSide(set, tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		st := NewAtomStats()
		st.Seed = p.Seed
		rc := RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 60, Check: AtomCheckFor(st, atom, nil)}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
		CloseSide(k)
		games++
		total.Add(st)
		if d := res.Div; d != nil && d.What == "atom" {
			t.Errorf("pair %d: decision %d: %s: %s", i, d.At, d.Field, d.Detail)
		}
	}
	if games == 0 || total.Decisions == 0 {
		t.Fatalf("%d games, %d compared decisions", games, total.Decisions)
	}
	n := atom[0].Name
	if total.True[n] == 0 || total.False[n] == 0 {
		t.Errorf("%s read true %d, false %d over %d decisions: the seed must read it both ways", n, total.True[n], total.False[n], total.Decisions)
	}
}
