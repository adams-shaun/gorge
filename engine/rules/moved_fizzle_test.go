package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mfEngine is b5Engine with named corpus cards for both seats.
func mfEngine(t *testing.T, names0, names1 []string) (*Engine, Config) {
	t.Helper()
	reg := searchTestRegistry(t)
	plains := searchCorpusCard(t, reg, "Plains")
	deck := func(names []string) []*cards.Card {
		var d []*cards.Card
		for _, n := range names {
			d = append(d, searchCorpusCard(t, reg, n))
		}
		for len(d) < 40 {
			d = append(d, plains)
		}
		return d
	}
	cfg := seatZeroStart(Config{Seed: 7402, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck(names0), deck(names1)}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// mfKill moves a permanent to the graveyard so the spell targeting it fizzles.
func mfKill(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
		t.Fatalf("target is in %v before the kill, want battlefield", z)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
}

func TestMovedFizzleFeatherFizzledSpellGoesToGraveyard(t *testing.T) {
	t.Parallel()
	e, cfg := b5Engine(t, "Feather, the Redeemed", "Giant Growth", "Grizzly Bears")
	searchMoveByName(t, e, "Feather, the Redeemed", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	growth := searchMoveByName(t, e, "Giant Growth", state.ZHand)
	addMana(t, e, 0, "G")
	lbCast(t, e, growth, bear)
	if z := e.G.Obj(growth).Zone; z != state.ZStack {
		t.Fatalf("Giant Growth is in %v after the cast, want stack", z)
	}
	mfKill(t, e, bear)
	lbResolve(t, e)
	if z := e.G.Obj(growth).Zone; z != state.ZGraveyard {
		t.Fatalf("fizzled Giant Growth rests in %v, want graveyard (it never resolved)", z)
	}
	driveToStepAll(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		submitPass(t, e)
	}
	if z := e.G.Obj(growth).Zone; z != state.ZGraveyard {
		t.Fatalf("Giant Growth is in %v after the end step, want graveyard (no return)", z)
	}
	lbNoUnsupportedNote(t, e)
	replayCheck(t, e, cfg)
}

func TestMovedFizzleFeatherCounteredSpellGoesToGraveyard(t *testing.T) {
	t.Parallel()
	e, cfg := mfEngine(t, []string{"Feather, the Redeemed", "Giant Growth"}, []string{"Counterspell"})
	feather := searchMoveByName(t, e, "Feather, the Redeemed", state.ZBattlefield)
	growth := searchMoveByName(t, e, "Giant Growth", state.ZHand)
	counter := searchMoveByNameSeat(t, e, 1, "Counterspell", state.ZHand)
	addMana(t, e, 0, "G")
	addManaQuiet(t, e, 1, "UU")
	lbCast(t, e, growth, feather)
	if z := e.G.Obj(growth).Zone; z != state.ZStack {
		t.Fatalf("Giant Growth is in %v after the cast, want stack", z)
	}
	submitPass(t, e) // seat 0 passes; seat 1 holds priority
	if d := e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("seat 1 not holding priority: %+v", d)
	}
	lbCast(t, e, counter, growth)
	if z := e.G.Obj(counter).Zone; z != state.ZStack {
		t.Fatalf("Counterspell is in %v after the cast, want stack", z)
	}
	lbResolve(t, e)
	if z := e.G.Obj(growth).Zone; z != state.ZGraveyard {
		t.Fatalf("countered Giant Growth rests in %v, want graveyard", z)
	}
	lbNoUnsupportedNote(t, e)
	replayCheck(t, e, cfg)
}

func TestMovedFizzleLilahFizzledSpellIsNotPlotted(t *testing.T) {
	t.Parallel()
	e, cfg := b5Engine(t, "Lilah, Undefeated Slickshot", "Lightning Helix", "Grizzly Bears")
	searchMoveByName(t, e, "Lilah, Undefeated Slickshot", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	helix := searchMoveByName(t, e, "Lightning Helix", state.ZHand)
	addMana(t, e, 0, "RW")
	lbCast(t, e, helix, bear)
	if z := e.G.Obj(helix).Zone; z != state.ZStack {
		t.Fatalf("Lightning Helix is in %v after the cast, want stack", z)
	}
	mfKill(t, e, bear)
	lbResolve(t, e)
	o := e.G.Obj(helix)
	if o.Zone != state.ZGraveyard {
		t.Fatalf("fizzled Lightning Helix rests in %v, want graveyard", o.Zone)
	}
	if o.PlottedTurn != 0 {
		t.Fatal("a fizzled spell became plotted")
	}
	lbNoUnsupportedNote(t, e)
	replayCheck(t, e, cfg)
}
