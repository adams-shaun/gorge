package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ctScen: p0 has the given permanents (none summoning sick) and Balustrade
// Spy ({3}{B}) in hand; life is p0's life.
func ctScen(t *testing.T, life int16, cfg Config, board ...string) (sc *scen, spy Obj, id map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Balustrade Spy", Count: 1}, {Name: "Swamp", Count: 10}}
	seen := map[string]bool{}
	for _, n := range board {
		if !seen[n] {
			seen[n] = true
			deck = append(deck, cardset.Entry{Name: n, Count: 1})
		}
	}
	set := cardsettest.Set(t, "caretaker")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Swamp"], cfg)
	for _, n := range board {
		sc.perm(id[n], 0)
	}
	spy = sc.hand(id["Balustrade Spy"], 0)
	sc.s.pl[0].life = life
	return sc, spy, id
}

// TestPainBudgetBoundsTheGapsWalk (the autopay panic of the
// prioritydiff ablation): three Elves of Deep Shadow and a Wall of Roots at 2
// life. The Elves' summed damage is bounded (a plan never takes lethal pain:
// one Elf at most), so Balustrade Spy ({3}{B}) is out of reach: one B, the
// Wall's {G}, nothing else. The gaps walk (the Wall is a census gap) counted
// every Elf's {B} alone, offered the Spy, and the cast's payment panicked
// "autopay found no plan". At 4 life the three Elves and the Wall pay it.
func TestPainBudgetBoundsTheGapsWalk(t *testing.T) {
	for _, cfg := range []Config{{NoElide: true}, {NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true}} {
		sc, spy, _ := ctScen(t, 2, cfg, "Elves of Deep Shadow", "Elves of Deep Shadow", "Elves of Deep Shadow", "Wall of Roots")
		d := sc.s.Step()
		if castOffered(&d, spy) {
			t.Fatalf("cfg %+v: Spy offered at 2 life: the Elves' pain bounds the mana to {B} and the Wall's {G}", cfg)
		}
		sc, spy, _ = ctScen(t, 4, cfg, "Elves of Deep Shadow", "Elves of Deep Shadow", "Elves of Deep Shadow", "Wall of Roots")
		d = sc.s.Step()
		if cfg.GorgeAutopaySurface {
			continue // the autopay surface drops a scripted plain cast
		}
		if !castOffered(&d, spy) {
			t.Fatalf("cfg %+v: Spy not offered at 4 life", cfg)
		}
		sc.s.Apply(MakeAction(AkCast, TCode(spy), 0, 0))
		sc.s.Step() // the payment: no "autopay found no plan"
		if sc.s.sp != 1 {
			t.Fatalf("cfg %+v: stack %d after the Spy's cast, want 1", cfg, sc.s.sp)
		}
	}
}

// TestUnpayableCostIsNeverOffered: every cast the priority walk offers is
// payable (a panic "autopay found no plan" is a kernel defect, never a
// result). Boards of Elves of Deep Shadow, a Wall of Roots, Saruli Caretaker
// and Springleaf Drum at every life from 1 to 5 offer nothing they cannot pay.
func TestUnpayableCostIsNeverOffered(t *testing.T) {
	boards := [][]string{
		{"Elves of Deep Shadow", "Elves of Deep Shadow", "Wall of Roots"},
		{"Elves of Deep Shadow", "Elves of Deep Shadow", "Elves of Deep Shadow", "Wall of Roots"},
		{"Elves of Deep Shadow", "Elves of Deep Shadow", "Saruli Caretaker", "Springleaf Drum"},
		{"Elves of Deep Shadow", "Saruli Caretaker", "Springleaf Drum", "Wall of Roots", "Grizzly Bears"},
		{"Elves of Deep Shadow", "Elves of Deep Shadow", "Elves of Deep Shadow", "Elves of Deep Shadow", "Saruli Caretaker"},
	}
	offered := 0
	for bi, board := range boards {
		for life := int16(1); life <= 5; life++ {
			for _, cfg := range []Config{{NoElide: true}, {NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true}} {
				sc, spy, _ := ctScen(t, life, cfg, board...)
				d := sc.s.Step()
				if !castOffered(&d, spy) {
					continue
				}
				offered++
				func() {
					defer func() {
						if e := recover(); e != nil {
							t.Errorf("board %d life %d cfg %+v: the Spy was offered and its payment panicked: %v", bi, life, cfg, e)
						}
					}()
					sc.s.Apply(MakeAction(AkCast, TCode(spy), 0, 0))
					sc.s.Step()
					if sc.s.sp != 1 && sc.s.ctl != ctlPriority {
						t.Errorf("board %d life %d cfg %+v: stack %d after the cast", bi, life, cfg, sc.s.sp)
					}
				}()
			}
		}
	}
	if offered < 4 {
		t.Errorf("only %d of the boards' casts were offered: the test no longer exercises the offer", offered)
	}
	_ = kop.ColAny
}
