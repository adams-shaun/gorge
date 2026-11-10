package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// delaySpellRegisterCard executes card c's delayed SpellCast registration
// over the permanent o, as resolving that ability would (the program's
// OpDelayedProgram names the latent and the source).
func (sc *scen) delaySpellRegisterCard(t *testing.T, c CardID, o Obj, p uint8) {
	t.Helper()
	ab := -1
	for j := range sc.k.t.Cards[c].Abilities {
		for _, in := range sc.k.t.Cards[c].Abilities[j].Prog {
			if in.Op == kop.OpDelayedProgram && in.A.N == kop.DelaySpellCast {
				ab = j
			}
		}
	}
	if ab < 0 {
		t.Fatalf("card %d has no delayed SpellCast program", c)
	}
	e := &sc.s.res.e
	e.ctrl, e.obj, e.card, e.ab = p, o, c, uint8(ab)
	prog := sc.k.t.Cards[c].Abilities[ab].Prog
	for i := range prog {
		sc.s.exec(sc.k, &prog[i])
	}
}

// TestDelaySpellSourceBoundFires: Loki's registration reads its own power at
// the cast. A three-mana instant misses at power 2; a two-mana one fires; two
// +1/+1 counters put on Loki between the registration and the cast move the
// bound, so the three-mana instant fires.
func TestDelaySpellSourceBoundFires(t *testing.T) {
	set := cardsettest.Synth(t, "std-msh")
	deck := []cardset.Entry{
		{Name: "Loki Laufeyson", Count: 1},
		{Name: "Blazing Crescendo", Count: 1},
		{Name: "Depower", Count: 1},
		{Name: "Mountain", Count: 20},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	loki := sc.perm(id["Loki Laufeyson"], 0)
	sl := sc.s.objs[loki].slot
	if got, _ := sc.s.ptGen(sc.k, sl); got != 2 {
		t.Fatalf("Loki starts at power %d, want 2", got)
	}

	sc.delaySpellRegisterCard(t, id["Loki Laufeyson"], loki, 0)
	sc.delaySpellCast(id["Depower"], 0) // mana value 3
	if sc.s.npend != 0 || sc.s.g.ndp != 1 {
		t.Fatalf("three-mana cast at power 2: %d pending, %d registered; want 0 and 1", sc.s.npend, sc.s.g.ndp)
	}

	cast := sc.delaySpellCast(id["Blazing Crescendo"], 0) // mana value 2
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("two-mana cast at power 2: %d pending, %d registered; want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
	if p := sc.s.pend[0]; p.trigObj != cast || p.card != id["Loki Laufeyson"] || p.ctrl != 0 {
		t.Fatalf("pending %+v, want Loki's latent over the cast spell %d", p, cast)
	}
	sc.s.npend = 0

	sc.s.perms[sl].p1 += 2
	if got, _ := sc.s.ptGen(sc.k, sl); got != 4 {
		t.Fatalf("Loki pumped to power %d, want 4", got)
	}
	sc.delaySpellRegisterCard(t, id["Loki Laufeyson"], loki, 0)
	sc.delaySpellCast(id["Depower"], 0) // mana value 3, now within power 4
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("three-mana cast at power 4: %d pending, %d registered; want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
}
