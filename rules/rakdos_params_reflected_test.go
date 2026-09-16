package rules

// The Rakdos-params brief, gap 11: AB$ ManaReflected — the reflected-mana
// family (24 raw corpus lines) whose colour set is computed at RESOLUTION
// from the objects a Valid$ selector names, rather than printed on the
// script. Before this work the API was unregistered: activating one of these
// abilities resolved to the unhandled-API no-op and added nothing.
//
// Implemented (effects/reflected.go): ReflectProperty$ Is reads ColorsOf,
// Produce reads Face.ManaProduction; ColorOrType$ Type adds colourless; the
// selector shapes Defined.Self / Defined.Imprinted / Defined.ExiledWith /
// Defined.ValidGraveyard <spec> / Defined.Sacrificed / Defined.Untapped and
// bare card specs over the battlefields resolve deterministically; a
// multi-colour set degenerates to the full Amount in EVERY reflected colour
// (the documented stand-in effMana's Combo head keeps). Imprint$ True on the
// exile move now records ExiledWith provenance (Chrome Mox's ETB trigger),
// the same event payload the SVar ExiledWithSource shapes already carried.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// activateAbilityOf finds the pending "ability" option for obj's API-named
// ability and submits it.
func activateAbilityOf(t *testing.T, e *Engine, obj state.ObjID, api string) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision to activate %s from: %+v", api, d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind != "ability" || o.Obj != obj {
			continue
		}
		face := e.G.Obj(obj).Face()
		if face == nil || o.Ability < 0 || o.Ability >= len(face.Abilities) {
			continue
		}
		if face.Abilities[o.Ability].API == api {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("%s's %s ability not offered: %+v", e.G.Obj(obj).Face().Name, api, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 50)
}

func TestChromeMoxReflectsTheImprintedCardColour(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Chrome Mox"))
	mox := e.G.Zone(state.ZHand, 0)[0]
	// Seed the imprint candidate BEFORE the trigger resolves (the mover ask
	// reads the hand at resolution time, so the card must be there first).
	cleric := e.G.AddObject(card(t, "Name:Imprinted Cleric\nManaCost:1 W\nTypes:Creature Cleric\nPT:1/2\nOracle:x\n"), 0)
	// A real move (not SetZone) so the object's Zone field matches the slice
	// the hidden-hand mover ask reads.
	e.emit(events.Event{Kind: events.MoveZone, Obj: cleric.ID, From: state.ZLibrary, To: state.ZHand})
	e.emit(events.Event{Kind: events.MoveZone, Obj: mox, From: state.ZHand, To: state.ZBattlefield})
	e.priorityRound()
	// The optional ask is posed at RESOLUTION (CR 603.5, the trigger sits on
	// the stack until then), so resolve the top and expect the yes/no ask.
	e.resolveTop()
	// The imprint trigger is optional (OptionalDecider$ You): a yes/no ask.
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("no optional imprint ask: %+v", d)
	}
	yes := -1
	for _, o := range d.Options {
		if o.Kind == "yes" {
			yes = o.Index
		}
	}
	if yes < 0 {
		t.Fatalf("imprint ask without a yes option: %+v", d.Options)
	}
	submitChoices(t, e, yes)
	// The mover ask (Min 0 -- "you may exile"): pick the Cleric when posed.
	if d = e.Pending(); d != nil && d.Kind == decision.KChoose {
		submitChoices(t, e, d.Options[0].Index)
	}
	passUntilStackEmpty(t, e, 50)
	if o := e.G.Obj(cleric.ID); o == nil || o.Zone != state.ZExile || o.ExiledWith != mox {
		t.Fatalf("imprinted card zone=%v exiledWith=%v, want exile/%d", o, o.ExiledWith, mox)
	}
	addMana(t, e, 0, "")
	activateAbilityOf(t, e, mox, "ManaReflected")
	if got := e.G.Players[0].Pool[state.MW]; got != 1 {
		t.Fatalf("pool W=%d, want 1 (the exiled Cleric's colour)", got)
	}
	for _, m := range []int{state.MU, state.MB, state.MR, state.MG, state.MC} {
		if e.G.Players[0].Pool[m] != 0 {
			t.Fatalf("pool slot %d = %d, want 0", m, e.G.Players[0].Pool[m])
		}
	}
	if !e.G.Obj(mox).Tapped {
		t.Fatal("the {T} cost did not tap Chrome Mox")
	}
}

func TestCorruptedGrafstoneReflectsGraveyardColours(t *testing.T) {
	// Single colour in the graveyard: the exact behaviour.
	e := handEngine(t, corpusAlternativeCard(t, "Corrupted Grafstone"))
	stone := e.G.Obj(e.G.Zone(state.ZHand, 0)[0])
	stone.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{stone.ID})
	dead := e.G.AddObject(card(t, "Name:Grave Cleric\nManaCost:1 W\nTypes:Creature Cleric\nPT:1/2\nOracle:x\n"), 0)
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{dead.ID})
	addMana(t, e, 0, "")
	activateAbilityOf(t, e, stone.ID, "ManaReflected")
	if got := e.G.Players[0].Pool[state.MW]; got != 1 {
		t.Fatalf("pool W=%d, want 1", got)
	}

	// Two colours: the stand-in adds the full Amount in EVERY reflected
	// colour, recorded with a Note (the colour-choice ask is M4 work).
	e2 := handEngine(t, corpusAlternativeCard(t, "Corrupted Grafstone"))
	stone2 := e2.G.Obj(e2.G.Zone(state.ZHand, 0)[0])
	stone2.Zone = state.ZBattlefield
	e2.G.SetZone(state.ZBattlefield, 0, []state.ObjID{stone2.ID})
	w := e2.G.AddObject(card(t, "Name:Grave Cleric\nManaCost:1 W\nTypes:Creature Cleric\nPT:1/2\nOracle:x\n"), 0)
	u := e2.G.AddObject(card(t, "Name:Grave Wizard\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n"), 0)
	e2.G.SetZone(state.ZGraveyard, 0, []state.ObjID{w.ID, u.ID})
	start := len(e2.L.Events)
	addMana(t, e2, 0, "")
	activateAbilityOf(t, e2, stone2.ID, "ManaReflected")
	if e2.G.Players[0].Pool[state.MW] != 1 || e2.G.Players[0].Pool[state.MU] != 1 {
		t.Fatalf("pool W=%d U=%d, want the stand-in's 1/1", e2.G.Players[0].Pool[state.MW], e2.G.Players[0].Pool[state.MU])
	}
	note := false
	for _, ev := range e2.L.Events[start:] {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "colour-choice stand-in") {
			note = true
		}
	}
	if !note {
		t.Fatalf("the multi-colour stand-in recorded no Note: %+v", e2.L.Events[start:])
	}
}

func TestExoticOrchardReflectsProducedColours(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Exotic Orchard"))
	orchard := e.G.Obj(e.G.Zone(state.ZHand, 0)[0])
	orchard.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{orchard.ID})
	// The opponent's land produces white: the orchard reflects Produce.
	guest := e.G.AddObject(card(t, "Name:Guest Plains\nTypes:Land Plains\nA:AB$ Mana | Cost$ T | Produced$ W\nOracle:x\n"), 1)
	guest.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{guest.ID})
	addMana(t, e, 0, "")
	activateAbilityOf(t, e, orchard.ID, "ManaReflected")
	if got := e.G.Players[0].Pool[state.MW]; got != 1 {
		t.Fatalf("pool W=%d, want 1 (the guest land's production)", got)
	}
	if e.G.Players[0].Pool.Total() != 1 {
		t.Fatalf("pool total=%d, want 1", e.G.Players[0].Pool.Total())
	}
}
