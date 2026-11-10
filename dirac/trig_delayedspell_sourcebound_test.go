package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDelaySpellSourceBoundFires: a delayed SpellCast filter whose bound is
// the registration's own power reads that source at the cast. A three-mana
// instant misses at power 2; a two-mana one fires; two +1/+1 counters put on
// the source between the registration and the cast move the bound, so the
// three-mana instant fires.
func TestDelaySpellSourceBoundFires(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant, Pred: kop.PredCMCLEV, PredV: kop.ValueRef{K: kop.VSelfPower}})
	source := CardID(len(tbl.Cards))
	twoMV := source + 1
	threeMV := source + 2
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Powerbound Source", Types: kop.TCreature, Power: 2, Toughness: 1,
			Abilities: []kop.Ability{
				{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpDelayedProgram, A: kop.Const(kop.DelaySpellCast), C: 1}}},
				{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Trig: kop.Trigger{Event: kop.TrigSpellCast, Filter: fid}},
			}},
		kop.CardDef{Name: "Two", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 2}},
		kop.CardDef{Name: "Three", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 3}},
	)
	sc := newScenTableLib(t, tbl, da[0], Config{NoElide: true})
	src := sc.perm(source, 0)
	sl := sc.s.objs[src].slot
	if pw, _ := sc.s.ptGen(sc.k, sl); pw != 2 {
		t.Fatalf("source starts at power %d, want 2", pw)
	}

	register := func() {
		e := &sc.s.res.e
		e.ctrl, e.obj, e.card, e.ab = 0, src, source, 0
		sc.s.exec(sc.k, &sc.k.t.Cards[source].Abilities[0].Prog[0])
	}
	cast := func(c CardID) Obj {
		o := sc.obj(c, 0)
		sc.s.objs[o].zone = kop.ZoneStack
		e := stackEntry{card: c, ctrl: 0, obj: o}
		sc.s.onCast(sc.k, &e)
		return o
	}

	register()
	cast(threeMV)
	if sc.s.npend != 0 || sc.s.g.ndp != 1 {
		t.Fatalf("three-mana cast at power 2: %d pending, %d registered; want 0 and 1", sc.s.npend, sc.s.g.ndp)
	}

	cast(twoMV)
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("two-mana cast at power 2: %d pending, %d registered; want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
	if p := sc.s.pend[0]; p.card != source || p.ab != 1 || p.ctrl != 0 {
		t.Fatalf("pending %+v, want the source's latent over the cast spell", p)
	}
	sc.s.npend = 0

	sc.s.perms[sl].p1 += 2
	if pw, _ := sc.s.ptGen(sc.k, sl); pw != 4 {
		t.Fatalf("source pumped to power %d, want 4", pw)
	}
	register()
	cast(threeMV)
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("three-mana cast at power 4: %d pending, %d registered; want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
}
