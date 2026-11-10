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
	f := newSourceBoundScen(t, cardsettest.Set(t, "fdn"))
	sc, source, twoMV, threeMV, sl := f.sc, f.source, f.twoMV, f.threeMV, f.sl
	if pw, _ := sc.s.ptGen(sc.k, sl); pw != 2 {
		t.Fatalf("source starts at power %d, want 2", pw)
	}

	register, cast := f.register, f.cast

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

// sourceBoundScen is the synthetic table of the source-bound delayed SpellCast
// tests: a power-2 source whose activation registers "when you next cast an
// instant with mana value <= my power", and a two- and three-mana instant.
type sourceBoundScen struct {
	sc                     *scen
	source, twoMV, threeMV CardID
	src                    Obj
	sl                     Slot
}

func newSourceBoundScen(t *testing.T, set cardset.Set) *sourceBoundScen {
	t.Helper()
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
	return &sourceBoundScen{sc: sc, source: source, twoMV: twoMV, threeMV: threeMV, src: src, sl: sc.s.objs[src].slot}
}

func (f *sourceBoundScen) register() {
	e := &f.sc.s.res.e
	e.ctrl, e.obj, e.card, e.ab = 0, f.src, f.source, 0
	f.sc.s.exec(f.sc.k, &f.sc.k.t.Cards[f.source].Abilities[0].Prog[0])
}

func (f *sourceBoundScen) cast(c CardID) Obj {
	o := f.sc.obj(c, 0)
	f.sc.s.objs[o].zone = kop.ZoneStack
	e := stackEntry{card: c, ctrl: 0, obj: o}
	f.sc.s.onCast(f.sc.k, &e)
	return o
}

// TestDelaySpellSourceBoundDeparted: the bound reads the source's last known
// power once it has left the battlefield (CR 608.2h), not the printed power of
// the card now in the graveyard. Pumped to 4 after the registration, a
// three-mana cast after the departure fires; with no pump (last known 2) it
// misses and a two-mana cast fires.
func TestDelaySpellSourceBoundDeparted(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pump     bool
		cast     func(f *sourceBoundScen) CardID
		wantPend uint8
	}{
		{"pumped, three-mana cast fires", true, func(f *sourceBoundScen) CardID { return f.threeMV }, 1},
		{"unpumped, three-mana cast misses", false, func(f *sourceBoundScen) CardID { return f.threeMV }, 0},
		{"unpumped, two-mana cast fires", false, func(f *sourceBoundScen) CardID { return f.twoMV }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := cardsettest.Set(t, "fdn")
			f := newSourceBoundScen(t, set)
			sc := f.sc
			f.register()
			if sc.s.g.ndp != 1 {
				t.Fatalf("precondition: %d registrations, want 1", sc.s.g.ndp)
			}
			want := 2
			if tc.pump {
				sc.s.perms[f.sl].p1 += 2
				want = 4
			}
			if pw, _ := sc.s.ptGen(sc.k, f.sl); pw != want {
				t.Fatalf("precondition: battlefield power %d, want %d", pw, want)
			}
			sc.s.leave(sc.k, f.sl, kop.ZoneGraveyard, false)
			if z := sc.s.objs[f.src].zone; z != kop.ZoneGraveyard || sc.s.g.ndp != 1 {
				t.Fatalf("precondition: source zone %v, %d registrations; want the graveyard and 1", z, sc.s.g.ndp)
			}
			f.cast(tc.cast(f))
			wantNdp := uint8(1)
			if tc.wantPend == 1 {
				wantNdp = 0
			}
			if sc.s.npend != tc.wantPend || sc.s.g.ndp != wantNdp {
				t.Fatalf("npend=%d ndp=%d, want %d and %d", sc.s.npend, sc.s.g.ndp, tc.wantPend, wantNdp)
			}
		})
	}
}
