package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Miracle (CR 702.94a, kw_miracle.go) on FDN's table plus one synthetic
// sorcery: the first draw of a turn queues the trigger and asks the reveal,
// a yes puts it on the stack with priority, its resolution asks the cast.

// miracleScen is a main phase with a Plains untapped for seat 0 and the
// miracle sorcery (miracle {W}, "gain 3 life") on top of seat 0's library.
func miracleScen(t *testing.T) (sc *scen, mir Obj, plains CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	c := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Miracle", Types: kop.TSorcery,
		Cost: cost(5, kop.W), HasMiracle: true, MiracleCost: cost(0, kop.W),
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(3)}}}}})
	sc = newScenTableLib(t, tbl, plains, Config{NoElide: true})
	if !sc.k.gen.anyMiracle {
		t.Fatal("anyMiracle not set")
	}
	sc.perm(plains, 0)
	mir = sc.obj(c, 0)
	sc.s.objs[mir].zone = kop.ZoneLibrary
	sc.s.pl[0].libTop(mir)
	return sc, mir, plains
}

// miracleAnswer applies the pending ChOptional decision's yes or no.
func miracleAnswer(t *testing.T, sc *scen, yes bool) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChOptional || d.Player != 0 {
		t.Fatalf("decision kind %d sub %d player %d, want seat 0's optional ask", d.Kind, d.Sub, d.Player)
	}
	want := uint8(0)
	if yes {
		want = 1
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == want {
			sc.s.Apply(d.Options[i])
			return
		}
	}
	t.Fatalf("no option with mode %d among %v", want, d.Options[:d.N])
}

// TestMiracleFirstDrawCasts: the first draw reveals (yes), the trigger waits on
// the stack with the card in hand and gives priority, resolves into the cast
// ask, and a yes casts the card for {W}: the Plains is tapped, the spell
// resolves and gains 3 life.
func TestMiracleFirstDrawCasts(t *testing.T) {
	sc, mir, _ := miracleScen(t)
	s := sc.s
	life := s.pl[0].life
	s.draw(0)
	if s.npend != 1 || s.pend[0].ab != abMiracle || s.objs[mir].zone != kop.ZoneHand {
		t.Fatalf("pending %d, zone %d: want the queued miracle trigger and the card in hand", s.npend, s.objs[mir].zone)
	}
	miracleAnswer(t, sc, true) // reveal
	if d := s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision kind %d, want priority with the trigger on the stack", d.Kind)
	}
	if s.sp != 1 || s.stack[0].ab != abMiracle || s.objs[mir].zone != kop.ZoneHand {
		t.Fatalf("stack %d ab %d card zone %d: want the miracle trigger on the stack, the card in hand", s.sp, s.stack[0].ab, s.objs[mir].zone)
	}
	sc.passBoth(t)             // the trigger resolves
	miracleAnswer(t, sc, true) // cast
	if s.ctl != ctlBuild || s.bld().e.kind != seSpell || s.bld().e.obj != mir || s.bld().e.trigPl != trigPlMiracle {
		t.Fatalf("ctl %d: want the miracle cast of the card being built", s.ctl)
	}
	sc.resolveAll(t)
	if s.pl[0].life != life+3 || s.objs[mir].zone != kop.ZoneGraveyard {
		t.Fatalf("life %d (was %d), zone %d: want the spell cast and resolved", s.pl[0].life, life, s.objs[mir].zone)
	}
}

// TestMiracleRevealDeclined: a no to the reveal drops the trigger and keeps
// the card in hand with nothing on the stack.
func TestMiracleRevealDeclined(t *testing.T) {
	sc, mir, _ := miracleScen(t)
	s := sc.s
	s.draw(0)
	if s.npend != 1 {
		t.Fatalf("pending %d, want the queued trigger", s.npend)
	}
	miracleAnswer(t, sc, false)
	if d := s.Step(); d.Kind != DkPriority || s.sp != 0 || s.objs[mir].zone != kop.ZoneHand {
		t.Fatalf("kind %d stack %d zone %d: want priority, an empty stack and the card in hand", d.Kind, s.sp, s.objs[mir].zone)
	}
}

// TestMiracleCastDeclined: a yes to the reveal then a no to the cast leaves
// the card in hand.
func TestMiracleCastDeclined(t *testing.T) {
	sc, mir, _ := miracleScen(t)
	s := sc.s
	s.draw(0)
	miracleAnswer(t, sc, true)
	sc.passBoth(t)
	miracleAnswer(t, sc, false)
	if d := s.Step(); d.Kind != DkPriority || s.sp != 0 || s.objs[mir].zone != kop.ZoneHand {
		t.Fatalf("kind %d stack %d zone %d: want priority, an empty stack and the card in hand", d.Kind, s.sp, s.objs[mir].zone)
	}
}

// TestMiracleSecondDrawNoAsk: a miracle card that is the turn's second draw
// queues nothing.
func TestMiracleSecondDrawNoAsk(t *testing.T) {
	sc, mir, plains := miracleScen(t)
	s := sc.s
	filler := sc.obj(plains, 0)
	s.objs[filler].zone = kop.ZoneLibrary
	s.pl[0].libTop(filler)
	s.draw(0)
	if s.pl[0].drawnTurn != 1 || s.npend != 0 {
		t.Fatalf("after the first draw: drawn %d pending %d", s.pl[0].drawnTurn, s.npend)
	}
	s.draw(0)
	if s.objs[mir].zone != kop.ZoneHand || s.pl[0].drawnTurn != 2 || s.npend != 0 {
		t.Fatalf("zone %d drawn %d pending %d: want the miracle card drawn second with no trigger", s.objs[mir].zone, s.pl[0].drawnTurn, s.npend)
	}
}

// TestMiracleTurnZeroNoAsk: the opening deal (turn 0) is no turn's draw.
func TestMiracleTurnZeroNoAsk(t *testing.T) {
	sc, mir, _ := miracleScen(t)
	s := sc.s
	s.turn = 0
	s.draw(0)
	if s.objs[mir].zone != kop.ZoneHand || s.pl[0].drawnTurn != 1 || s.npend != 0 {
		t.Fatalf("zone %d drawn %d pending %d: want a first draw on turn 0 with no trigger", s.objs[mir].zone, s.pl[0].drawnTurn, s.npend)
	}
}

// TestMiracleLeftHandNoAsk: the card leaving the hand while the trigger is on
// the stack ends it with no cast ask.
func TestMiracleLeftHandNoAsk(t *testing.T) {
	sc, mir, _ := miracleScen(t)
	s := sc.s
	s.draw(0)
	miracleAnswer(t, sc, true)
	if d := s.Step(); d.Kind != DkPriority || s.sp != 1 {
		t.Fatalf("kind %d stack %d: want priority with the trigger on the stack", d.Kind, s.sp)
	}
	from := s.detach(mir)
	s.placeFrom(mir, kop.ZoneGraveyard, false, from, false)
	if s.objs[mir].zone != kop.ZoneGraveyard {
		t.Fatalf("zone %d, want the graveyard", s.objs[mir].zone)
	}
	sc.passBoth(t)
	if d := s.Step(); d.Kind != DkPriority || s.sp != 0 || s.objs[mir].zone != kop.ZoneGraveyard {
		t.Fatalf("kind %d stack %d zone %d: want no ask, an empty stack and the card still in the graveyard", d.Kind, s.sp, s.objs[mir].zone)
	}
}
