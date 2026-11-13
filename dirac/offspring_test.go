package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
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
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) && a.Mode() == m {
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
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(vl)) {
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

// passUntilOnBF passes priority until o is on the battlefield.
func (sc *scen) passUntilOnBF(t *testing.T, o Obj) {
	t.Helper()
	for i := 0; i < 20 && !sc.onBF(o); i++ {
		if d := sc.s.Step(); d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			sc.s.Apply(d.Options[0])
		}
	}
	if !sc.onBF(o) {
		t.Fatal("the spell never entered")
	}
	sc.s.Step()
}

// "if its offspring cost was paid" is an intervening if (CR 702.175a, 603.4): a plain
// cast puts nothing on the stack as the creature enters.
func TestOffspringUnpaidDoesNotTrigger(t *testing.T) {
	sc, id := giftScen(t, "Iridescent Vinelasher", "Swamp")
	sc.perm(id["Swamp"], 0)
	vl := sc.hand(id["Iridescent Vinelasher"], 0)
	sc.castMode(t, vl, 0)
	sc.passUntilOnBF(t, vl)
	if sc.s.sp != 0 || sc.s.npend != 0 {
		t.Fatalf("stack %d pending %d after a plain cast entered, want neither", sc.s.sp, sc.s.npend)
	}
}

// Ravenous's draw (CR 702.156a) has the same shape: X below 5 puts nothing on the stack,
// and X of 5 puts a trigger on it that keeps the X for the recheck.
func TestRavenousInterveningIf(t *testing.T) {
	for _, tc := range []struct {
		x    uint8
		want bool
	}{{3, false}, {5, true}} {
		deck := []cardset.Entry{{Name: "Termagant Swarm", Count: 1}, {Name: "Forest", Count: 1}}
		tbl, da, _, err := cardsettest.Set(t, "mixedarms").Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		sc := newScenTable(t, tbl, Config{NoElide: true})
		for i := 0; i < 6; i++ {
			sc.perm(id["Forest"], 0)
		}
		hand := sc.s.pl[0].handN
		r := sc.hand(id["Termagant Swarm"], 0)
		sc.castCard(t, r)
		sc.chooseX(t, tc.x)
		sc.passUntilOnBF(t, r)
		if got := sc.s.sp > 0; got != tc.want {
			t.Fatalf("X=%d: stack %d after entering, want a trigger: %v", tc.x, sc.s.sp, tc.want)
		}
		if tc.want && uint8(sc.s.stack[sc.s.sp-1].x) != tc.x {
			t.Fatalf("X=%d: the trigger binds X=%d", tc.x, sc.s.stack[sc.s.sp-1].x)
		}
		sc.resolveAll(t)
		drew := int(sc.s.pl[0].handN) - int(hand)
		if want := b2i(tc.want); drew != want {
			t.Fatalf("X=%d: hand grew by %d, want %d", tc.x, drew, want)
		}
	}
}

// A Vinelasher that left with its paid offspring trigger on the stack still leaves its
// 1/1: the trigger read the payment as it fired (gorge TestOffspringSourceThatLeftStillMints).
func TestOffspringSourceThatLeftStillMints(t *testing.T) {
	sc, id := giftScen(t, "Iridescent Vinelasher", "Swamp")
	for i := 0; i < 3; i++ {
		sc.perm(id["Swamp"], 0)
	}
	vl := sc.hand(id["Iridescent Vinelasher"], 0)
	sc.castMode(t, vl, 1)
	sc.passUntilOnBF(t, vl)
	if sc.s.sp == 0 {
		t.Fatal("no offspring trigger on the stack")
	}
	sc.s.destroy(sc.k, sc.slot(vl))
	sc.resolveAll(t)
	cs := sc.copiesOf(id["Iridescent Vinelasher"])
	if len(cs) != 1 {
		t.Fatalf("%d copies, want the token", len(cs))
	}
	if p, _ := sc.s.pt(sc.k, cs[0]); p != 1 {
		t.Fatalf("the token has power %d, want 1", p)
	}
}
