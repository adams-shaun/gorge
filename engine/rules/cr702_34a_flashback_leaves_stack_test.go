package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr702FlashbackOnStack puts a Therapy-style sorcery (Flashback:
// Sac<1/Creature>) on the stack cast by its flashback ability, and returns
// the engine and the stack object.
func cr702FlashbackOnStack(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	src := "Name:Therapy\nManaCost:B\nTypes:Sorcery\nK:Flashback:Sac<1/Creature>\n" +
		"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, therapy := newFixtureDeck(t, seed, src, bearSrc)
	putCreature(t, e, 0, bearSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: therapy, From: state.ZHand, To: state.ZGraveyard})
	e.Advance()
	var fb *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "flashback" && o.Obj == therapy {
			fb = &o
		}
	}
	if fb == nil {
		t.Fatal("CR 702.34a: flashback not offered from the graveyard")
	}
	submitChoices(t, e, fb.Index)
	submitChoices(t, e, 0) // sacrifice the Bear
	if z := e.G.Obj(therapy).Zone; z != state.ZStack {
		t.Fatalf("CR 702.34a seq %d: therapy in %s, want the stack", len(e.L.Events), z)
	}
	return e, cfg, therapy
}

// TestCR702_34aFlashbackSpellExiledWhereverItWouldGo: CR 702.34a -- "exile
// this card instead of putting it anywhere else any time it would leave the
// stack". Remand (counter, to hand), Memory Lapse (counter, to the top of
// the library) and Unsubstantiate (return target spell to its owner's hand)
// each send a flashed-back spell to EXILE, not to the named zone.
func TestCR702_34aFlashbackSpellExiledWhereverItWouldGo(t *testing.T) {
	t.Parallel()
	for i, name := range []string{"Remand", "Memory Lapse", "Unsubstantiate"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg, therapy := cr702FlashbackOnStack(t, uint64(733+i))
			lib := len(e.G.Zone(state.ZLibrary, 0))
			effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: therapy}}, TargetsOffered: true},
				corpusCard(t, name).Faces[0].SpellAbility())
			if got := e.G.Obj(therapy).Zone; got != state.ZExile {
				t.Fatalf("CR 702.34a %s seq %d: flashed-back spell went to %s, want exile", name, len(e.L.Events), got)
			}
			if got := len(e.G.Zone(state.ZLibrary, 0)); got != lib {
				t.Fatalf("CR 702.34a %s seq %d: seat 0 library %d cards, want %d (unchanged)", name, len(e.L.Events), got, lib)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestCR702_34aRemandOrdinarySpellStillGoesToHand is the guard: a spell NOT
// cast with flashback follows Remand's own destination.
func TestCR702_34aRemandOrdinarySpellStillGoesToHand(t *testing.T) {
	t.Parallel()
	src := "Name:Therapy\nManaCost:B\nTypes:Sorcery\nK:Flashback:Sac<1/Creature>\n" +
		"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg, therapy := newFixtureDeck(t, 740, src)
	addMana(t, e, 0, "B")
	e.Advance()
	submitChoices(t, e, miscCastOption(t, e, therapy))
	if z := e.G.Obj(therapy).Zone; z != state.ZStack {
		t.Fatalf("CR 702.34a seq %d: therapy in %s, want the stack", len(e.L.Events), z)
	}
	effects.Resolve(e, &effects.Ctx{Controller: 1, Targets: []state.Target{{Obj: therapy}}, TargetsOffered: true},
		corpusCard(t, "Remand").Faces[0].SpellAbility())
	if got := e.G.Obj(therapy).Zone; got != state.ZHand {
		t.Fatalf("CR 702.34a seq %d: Remanded hand-cast spell went to %s, want hand", len(e.L.Events), got)
	}
	replayCheck(t, e, cfg)
}

// TestCR702_34aReversedFlashbackCastReturnsToGraveyard is the CR 733.1
// guard: a flashback cast reversed while it is still being proposed was never
// cast, so the card goes back to the graveyard it came from, not to exile.
// (At this base the flashback CastInfo flag is set only at payment, so the
// proposal carries no flag yet; the test pins the outcome whatever order a
// later cast-pipeline change gives the flag.)
func TestCR702_34aReversedFlashbackCastReturnsToGraveyard(t *testing.T) {
	t.Parallel()
	src := "Name:Therapy\nManaCost:B\nTypes:Sorcery\nK:Flashback:Sac<1/Creature>\n" +
		"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, _, therapy := newFixtureDeck(t, 741, src, bearSrc)
	putCreature(t, e, 0, bearSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: therapy, From: state.ZHand, To: state.ZGraveyard})
	e.Advance()
	var fb *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "flashback" && o.Obj == therapy {
			fb = &o
		}
	}
	if fb == nil {
		t.Fatal("CR 702.34a: flashback not offered from the graveyard")
	}
	submitChoices(t, e, fb.Index)
	if e.cast == nil || e.cast.stackObj != therapy || e.G.Obj(therapy).Zone != state.ZStack {
		t.Fatalf("CR 733.1 seq %d: want the flashback proposal pushed and pending its sacrifice, cast=%+v zone=%s",
			len(e.L.Events), e.cast, e.G.Obj(therapy).Zone)
	}
	e.abortCast(e.cast, "test reversal (CR 733.1)", false)
	if got := e.G.Obj(therapy).Zone; got != state.ZGraveyard {
		t.Fatalf("CR 733.1 seq %d: reversed flashback cast left the card in %s, want graveyard", len(e.L.Events), got)
	}
}
