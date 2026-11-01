package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Offspring (CR 702.175, kw_offspring.go) on BLB's table: the "(offspring)"
// cast beside the plain one, the paid cost, and the enter trigger's 1/1
// token copy (none for a plain cast, none for the copy itself).

// castMode casts o with AkCast mode m, failing when it is not offered.
func (sc *scen) castMode(t *testing.T, o Obj, m uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == uint8(o) && a.Mode() == m {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("cast of %d mode %d not offered: %v", o, m, d.Options[:d.N])
}

// copiesOf lists the battlefield permanents showing card c.
func (sc *scen) copiesOf(c CardID) (out []Slot) {
	for m := sc.s.occ; m.any(); m = m.rest() {
		if sl := m.first(); sc.s.cardAt(sl) == c {
			out = append(out, sl)
		}
	}
	return out
}

// The copy keeps the card's abilities: a land entering triggers each
// Vinelasher's landfall (1 damage to each opponent).
func TestOffspringPaidMintsOneOneCopy(t *testing.T) {
	for _, paid := range []bool{false, true} {
		sc, id := giftScen(t, "Iridescent Vinelasher", "Swamp")
		for i := 0; i < 3; i++ {
			sc.perm(id["Swamp"], 0)
		}
		vl := sc.hand(id["Iridescent Vinelasher"], 0)
		land := sc.hand(id["Swamp"], 0)
		if !sc.s.k().t.Cards[id["Iridescent Vinelasher"]].Offspring || sc.s.k().cards[id["Iridescent Vinelasher"]].kw&kop.KwKicker != 0 {
			t.Fatal("Vinelasher is not an Offspring card without KwKicker")
		}
		m := uint8(0)
		if paid {
			m = 1
		}
		sc.castMode(t, vl, m)
		sc.resolveAll(t)
		if !sc.onBF(vl) {
			t.Fatal("Vinelasher did not enter")
		}
		cs := sc.copiesOf(id["Iridescent Vinelasher"])
		want := 1
		if paid {
			want = 2
		}
		if len(cs) != want {
			t.Fatalf("paid=%v: %d Vinelashers on the battlefield, want %d", paid, len(cs), want)
		}
		untapped := 0
		for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
			if sl := m.first(); sc.s.cardAt(sl) == id["Swamp"] && sc.s.perms[sl].flags&pfTapped == 0 {
				untapped++
			}
		}
		if paid && untapped != 0 || !paid && untapped != 2 {
			t.Fatalf("paid=%v: %d Swamps untapped (the offspring {2} is paid only when chosen)", paid, untapped)
		}
		for _, sl := range cs {
			o := sc.s.perms[sl].obj
			p, tg := sc.s.FeatPT(o)
			if sc.s.objs[o].of&ofToken != 0 {
				if p != 1 || tg != 1 {
					t.Fatalf("token copy is %d/%d, want 1/1", p, tg)
				}
			} else if p != 1 || tg != 2 {
				t.Fatalf("Vinelasher is %d/%d, want 1/2", p, tg)
			}
		}
		life := sc.s.pl[1].life
		d := sc.s.Step()
		played := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkPlayLand && Obj(a.Source()) == land {
				sc.s.Apply(a)
				played = true
				break
			}
		}
		if !played {
			t.Fatal("land play not offered")
		}
		sc.resolveAll(t)
		if got := int(life - sc.s.pl[1].life); got != want {
			t.Fatalf("paid=%v: landfall dealt %d, want %d (one per Vinelasher)", paid, got, want)
		}
	}
}

// TestOffspringNotOfferedUnaffordable: with mana for the plain cast only,
// the offspring cast is not offered.
func TestOffspringNotOfferedUnaffordable(t *testing.T) {
	sc, id := giftScen(t, "Iridescent Vinelasher", "Swamp")
	sc.perm(id["Swamp"], 0)
	vl := sc.hand(id["Iridescent Vinelasher"], 0)
	d := sc.s.Step()
	plain := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == uint8(vl) {
			if a.Mode()&1 != 0 {
				t.Fatal("offspring cast offered without its {2}")
			}
			plain = true
		}
	}
	if !plain {
		t.Fatal("plain cast not offered")
	}
}
