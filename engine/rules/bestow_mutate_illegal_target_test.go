package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 608.3b with CR 702.103e (bestow) and CR 702.140b (mutate): a bestowed
// Aura spell or a mutating creature spell whose target is illegal as it
// begins resolving does NOT fizzle. It ceases to be bestowed / mutating and
// resolves as a creature spell, entering the battlefield under its
// controller's control. Corroborated by Forge's MagicStack resolution
// (unanimateBestow, then resolve as a creature; forge-game
// src/main/java/forge/game/zone/MagicStack.java:600-615).

// shockInResponse hands priority to seat 1 and has it cast the real Shock
// at victim, leaving both spells on the stack (Shock on top). Seat 1's {R}
// must already be in its pool.
func shockInResponse(t *testing.T, e *Engine, shockID, victim state.ObjID) {
	t.Helper()
	passPriorityOnce(t, e) // seat 0 passes with its spell on the stack
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("seat 1 should hold priority in response: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == shockID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seat 1 is not offered Shock: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Shock target ask: %+v", d)
	}
	ti := indexOfObjOption(d, victim)
	if ti < 0 {
		t.Fatalf("Shock does not offer %d: %+v", victim, d.Options)
	}
	submitChoices(t, e, ti)
}

// sawFizzle reports whether id left the stack through a "fizzled" MoveZone.
func sawFizzle(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZStack &&
			strings.HasPrefix(ev.Text, "fizzled") {
			return true
		}
	}
	return false
}

func bestowIllegalTargetGame(t *testing.T, seed uint64, seat1Extra ...*cards.Card) (e *Engine, cfg Config, satyrID, bearsID, shockID state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	satyr := mustCorpusCard(t, reg, "Boon Satyr")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	shock := mustCorpusCard(t, reg, "Shock")
	e, cfg = tokenReplGameSeats(t, seed, []*cards.Card{satyr, bears},
		append([]*cards.Card{shock}, seat1Extra...))
	satyrID = moveSeededCard(t, e, 0, satyr, state.ZHand)
	bearsID = moveSeededCard(t, e, 0, bears, state.ZBattlefield)
	shockID = moveSeededCard(t, e, 1, shock, state.ZHand)
	addMana(t, e, 1, "R")
	addMana(t, e, 0, "GGGGG") // the bestow cost {3}{G}{G}
	return e, cfg, satyrID, bearsID, shockID
}

// TestBoonSatyrBestowedOntoShockedBearsEntersAsCreature is the bestow repro:
// Boon Satyr cast bestowed targeting Grizzly Bears, Shock kills the Bears in
// response. CR 702.103e: the Satyr stops being bestowed and resolves as a
// creature spell -- a 4/2 unattached creature on the battlefield, never the
// graveyard.
func TestBoonSatyrBestowedOntoShockedBearsEntersAsCreature(t *testing.T) {
	e, cfg, satyrID, bearsID, shockID := bestowIllegalTargetGame(t, 7101)

	submitChoices(t, e, bestowedCastOption(t, e, satyrID).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("bestowed target ask: %+v", d)
	}
	submitChoices(t, e, indexOfObjOption(d, bearsID))
	shockInResponse(t, e, shockID, bearsID)
	passUntilStackEmpty(t, e, 20)

	if b := e.G.Obj(bearsID); b == nil || b.Zone != state.ZGraveyard {
		t.Fatalf("setup: Shock did not kill the Bears: %+v", b)
	}
	o := e.G.Obj(satyrID)
	if o == nil || o.Zone != state.ZBattlefield {
		zone := "nil"
		if o != nil {
			zone = o.Zone.String()
		}
		t.Fatalf("Boon Satyr zone = %s, want battlefield (CR 608.3b/702.103e: an illegal-target bestowed Aura spell resolves as a creature spell)", zone)
	}
	if sawFizzle(e, satyrID) {
		t.Fatal("Boon Satyr fizzled; CR 608.3b says it resolves as a creature spell")
	}
	if o.AttachedTo != 0 {
		t.Fatalf("Boon Satyr attached to %d, want unattached", o.AttachedTo)
	}
	if o.CastFlags&state.FlagBestowed != 0 {
		t.Fatal("Boon Satyr still carries FlagBestowed: CR 702.103e says it ceased to be bestowed")
	}
	if !e.IsCreature(satyrID) {
		t.Fatal("Boon Satyr is not a creature")
	}
	if p, tt := e.Power(satyrID), e.Toughness(satyrID); p != 4 || tt != 2 {
		t.Fatalf("Boon Satyr is %d/%d, want the printed 4/2", p, tt)
	}
	replayCheck(t, e, cfg)
}

// TestBoonSatyrUnbestowedIsACreatureToEntryReplacements: the un-bestowed
// spell is a CREATURE spell as it enters, so the opponent's Thalia, Heretic
// Cathar ("Creatures ... your opponents control enter tapped") applies to it.
// This pins that the bestowed type switch (state.Object.BestowedAuraSpell)
// is actually turned off before the battlefield move, not merely skipped:
// a Satyr still reading as an Aura at entry would enter untapped.
func TestBoonSatyrUnbestowedIsACreatureToEntryReplacements(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	thalia := mustCorpusCard(t, reg, "Thalia, Heretic Cathar")
	e, cfg, satyrID, bearsID, shockID := bestowIllegalTargetGame(t, 7102, thalia)
	moveSeededCard(t, e, 1, thalia, state.ZBattlefield)
	e.priorityRound()

	submitChoices(t, e, bestowedCastOption(t, e, satyrID).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("bestowed target ask: %+v", d)
	}
	submitChoices(t, e, indexOfObjOption(d, bearsID))
	shockInResponse(t, e, shockID, bearsID)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(satyrID)
	if o == nil || o.Zone != state.ZBattlefield {
		zone := "nil"
		if o != nil {
			zone = o.Zone.String()
		}
		t.Fatalf("Boon Satyr zone = %s, want battlefield (CR 702.103e)", zone)
	}
	if !o.Tapped {
		t.Fatal("Boon Satyr entered untapped: as an un-bestowed creature spell Thalia's enter-tapped replacement must apply")
	}
	replayCheck(t, e, cfg)
}

// TestGemrazerMutatingOntoShockedTargetEntersAsCreature is the mutate repro:
// Gemrazer cast for its mutate cost onto Pouncing Jaguar, Shock kills the
// Jaguar in response. CR 702.140b: Gemrazer stops being a mutating creature
// spell and enters as its own 4/4 creature; nothing mutates, so its
// "whenever this creature mutates" trigger does not fire.
func TestGemrazerMutatingOntoShockedTargetEntersAsCreature(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	gem := mustCorpusCard(t, reg, "Gemrazer")
	jaguar := mustCorpusCard(t, reg, "Pouncing Jaguar")
	shock := mustCorpusCard(t, reg, "Shock")
	e, cfg := tokenReplGameSeats(t, 7103, []*cards.Card{gem, jaguar}, []*cards.Card{shock})
	gemID := moveSeededCard(t, e, 0, gem, state.ZHand)
	jaguarID := moveSeededCard(t, e, 0, jaguar, state.ZBattlefield)
	shockID := moveSeededCard(t, e, 1, shock, state.ZHand)
	addMana(t, e, 1, "R")
	addMana(t, e, 0, "GGG") // the mutate cost {1}{G}{G}

	mutateCastOntoUndrained(t, e, mutatedCastOption(t, e, gemID), jaguarID, true)
	shockInResponse(t, e, shockID, jaguarID)
	passUntilStackEmpty(t, e, 20)

	if j := e.G.Obj(jaguarID); j == nil || j.Zone != state.ZGraveyard {
		t.Fatalf("setup: Shock did not kill the Jaguar: %+v", j)
	}
	o := e.G.Obj(gemID)
	if o == nil || o.Zone != state.ZBattlefield {
		zone := "nil"
		if o != nil {
			zone = o.Zone.String()
		}
		t.Fatalf("Gemrazer zone = %s, want battlefield (CR 608.3b/702.140b: an illegal-target mutating creature spell resolves as a creature spell)", zone)
	}
	if sawFizzle(e, gemID) {
		t.Fatal("Gemrazer fizzled; CR 608.3b says it resolves as a creature spell")
	}
	if o.CastFlags&(state.FlagMutated|state.FlagMutatedTop) != 0 {
		t.Fatal("Gemrazer still carries FlagMutated: CR 702.140b says it ceased to be a mutating creature spell")
	}
	if len(o.MergedCards) != 0 || o.TimesMutated != 0 {
		t.Fatalf("Gemrazer mutated into something: merged %+v times %d", o.MergedCards, o.TimesMutated)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Mutate {
			t.Fatalf("a Mutate fold was emitted with no legal target: %+v", ev)
		}
	}
	if p, tt := e.Power(gemID), e.Toughness(gemID); p != 4 || tt != 4 {
		t.Fatalf("Gemrazer is %d/%d, want the printed 4/4", p, tt)
	}
	replayCheck(t, e, cfg)
}
