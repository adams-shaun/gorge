package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReplDrawCauseCycling is the kernel's read of a ReplDrawProgram line's
// ValidCause$ Activated.Cycling+nonLand (kop.ReplDrawCauseCyclingNonLand,
// repl_drawprogram.go drawCauseCycling): the line only replaces a draw whose
// cause is a resolving activated cycling ability (kop.AfCycling) of a nonland
// card. The three leaves mirror gorge's pinned reference tests
// (engine/rules/draw_replacement_validcause_test.go): a cycling draw of a
// nonland card is replaced, an ordinary draw and a non-cycling activated
// ability's draw are not, and a land's cycling draw is not.
func TestReplDrawCauseCycling(t *testing.T) {
	// host is the gated replacement: a bodyless Prevent$ skip, so a replaced
	// draw simply takes nothing (no card, no library move).
	hostCard := kop.CardDef{Name: "Test Cause Cyclone", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement,
			Repl: kop.Repl{Kind: kop.ReplDrawProgram, C: kop.ReplDrawSkip | kop.ReplDrawCauseCyclingNonLand}}}}
	// cycler is a card whose activated ability discards itself and draws one:
	// the cycling form when cycling, the plain activated form otherwise.
	cycler := func(name string, types kop.TypeMask, cycling bool) kop.CardDef {
		a := kop.Ability{Kind: kop.AbActivated, ActZone: kop.ZoneHand,
			Cost: kop.Cost{Flags: kop.CostDiscardSelf},
			Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}
		if cycling {
			a.AFlags = kop.AfCycling
		}
		return kop.CardDef{Name: name, Types: types, Abilities: []kop.Ability{a}}
	}
	setup := func(t *testing.T, cyc kop.CardDef) (*scen, CardID) {
		t.Helper()
		tb := newSPTable()
		tb.tbl.Cards = append(tb.tbl.Cards, hostCard, cyc)
		host, cycler := CardID(len(tb.tbl.Cards)-2), CardID(len(tb.tbl.Cards)-1)
		sc := tb.scen(t)
		if !sc.onBF(sc.perm(host, 0)) {
			t.Fatal("precondition: the replacement host is not on the battlefield")
		}
		if sc.k.gen == nil || !sc.k.gen.hasDrawProgram() {
			t.Fatal("precondition: the gen flag hasDrawProgram is not set")
		}
		return sc, cycler
	}
	// cycle resolves card c's ability 0 the way a real activation does: the
	// discard-this cost first (the card to the graveyard), then the program
	// under the resolving entry, which is what the cause gate reads.
	cycle := func(t *testing.T, sc *scen, c CardID) {
		t.Helper()
		o := sc.hand(c, 0)
		e := stackEntry{kind: seAbility, obj: o, card: c, ab: 0, ctrl: 0, trigObj: NoObj}
		sc.s.discardSelfCost(sc.k, &e)
		if sc.s.objs[o].zone != kop.ZoneGraveyard {
			t.Fatalf("precondition: the cycled card is in zone %d, want the graveyard", sc.s.objs[o].zone)
		}
		if a := sc.k.t.Cards[c].Abilities[0]; a.AFlags&kop.AfCycling == 0 {
			t.Fatal("precondition: the resolving ability does not carry AfCycling")
		}
		sc.s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
		sc.s.runProg(sc.k)
		if sc.s.res.ask != raNone {
			t.Fatalf("precondition: the resolution parked on ask %d", sc.s.res.ask)
		}
	}
	// drawn asserts the plain draw happened: the hand gained one and the
	// library lost one (a replaced draw does neither).
	drawn := func(t *testing.T, sc *scen, hand, lib int) {
		t.Helper()
		if got := int(sc.s.pl[0].handN); got != hand+1 {
			t.Fatalf("hand = %d, want %d (the draw was replaced, but must not be)", got, hand+1)
		}
		if got := sc.s.pl[0].libLen(); got != lib-1 {
			t.Fatalf("library = %d, want %d (the draw took a card)", got, lib-1)
		}
	}

	t.Run("a cycling ability's draw of a nonland card is replaced", func(t *testing.T) {
		sc, c := setup(t, cycler("Test Cycler", kop.TInstant, true))
		hand, lib := int(sc.s.pl[0].handN), sc.s.pl[0].libLen()
		cycle(t, sc, c)
		if got := int(sc.s.pl[0].handN); got != hand {
			t.Fatalf("hand = %d, want %d (the cycling draw must be replaced)", got, hand)
		}
		if got := sc.s.pl[0].libLen(); got != lib {
			t.Fatalf("library = %d, want %d (a replaced draw takes no card)", got, lib)
		}
	})
	t.Run("an ordinary draw is left alone", func(t *testing.T) {
		sc, _ := setup(t, cycler("Test Cycler", kop.TInstant, true))
		hand, lib := int(sc.s.pl[0].handN), sc.s.pl[0].libLen()
		sc.s.draw(0)
		drawn(t, sc, hand, lib)
	})
	t.Run("a non-cycling activated ability's draw is left alone", func(t *testing.T) {
		sc, c := setup(t, cycler("Test Drawer", kop.TInstant, false))
		hand, lib := int(sc.s.pl[0].handN), sc.s.pl[0].libLen()
		o := sc.hand(c, 0)
		e := stackEntry{kind: seAbility, obj: o, card: c, ab: 0, ctrl: 0, trigObj: NoObj}
		sc.s.discardSelfCost(sc.k, &e)
		sc.s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
		sc.s.runProg(sc.k)
		drawn(t, sc, hand, lib)
	})
	t.Run("a land's cycling draw is left alone", func(t *testing.T) {
		sc, c := setup(t, cycler("Test Land Cycler", kop.TLand, true))
		hand, lib := int(sc.s.pl[0].handN), sc.s.pl[0].libLen()
		cycle(t, sc, c)
		drawn(t, sc, hand, lib)
	})
}
