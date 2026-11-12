package rules

// A spell is cast under ONE may-play permission (CR 601.2b/601.2f; the player
// chooses it, as for an alternative cost, CR 118.9), so only that
// permission's non-mana RaiseCost$ applies. Two granting statics with
// different raises must charge one, never their sum. Noctis, Prince of Lucis
// grants artifact spells from the graveyard for 3 life; Wickerfolk
// Indomitable -- itself an artifact creature in the graveyard -- also
// self-grants its own cast for 2 life plus a sacrifice. Before this fix the
// cast paid 5 life and a sacrifice; this pins 3 life with the sacrifice
// candidate untouched.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTwoMayPlayRaisesChargeOne: two granting raises charge exactly one
// permission's raise, the first in the walk order (Noctis's 3 life here).
func TestTwoMayPlayRaisesChargeOne(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Noctis, Prince of Lucis"))
	noctis := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, noctis)

	wicker := e.G.AddObject(corpusAlternativeCard(t, "Wickerfolk Indomitable"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: wicker.ID, From: state.ZLibrary, To: state.ZGraveyard})

	// The artifact creature Wickerfolk's own permission would have to
	// sacrifice if its raise were charged.
	fodder := e.G.AddObject(card(t, "Name:Raise Fodder\nTypes:Artifact Creature Thopter\nPT:1/1\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: fodder.ID, From: state.ZLibrary, To: state.ZBattlefield})

	// Precondition: BOTH statics grant this card and carry a priced raise,
	// and the life parts differ (3 vs 2), so "one permission" and "the sum"
	// are observably different charges.
	var raises []Cost
	for _, sv := range e.activeStatics("Continuous") {
		if !mayPlayStaticBenefits(0, &sv) {
			continue
		}
		_, grants, _, r, has, priced := e.mayPlayStatic(sv.Params, wicker.ID, mayPlayStaticYou(0, &sv), sv.Source)
		if grants && has && priced {
			raises = append(raises, r)
		}
	}
	for _, st := range mayPlayOwnStatics(e.G.Obj(wicker.ID), 0) {
		if st.Mode != "Continuous" {
			continue
		}
		_, grants, _, r, has, priced := e.mayPlayStatic(st.Params, wicker.ID, 0, wicker.ID)
		if grants && has && priced {
			raises = append(raises, r)
		}
	}
	if len(raises) != 2 {
		t.Fatalf("precondition: %d granting raises over Wickerfolk, want 2", len(raises))
	}
	if raises[0].Life == raises[1].Life {
		t.Fatalf("precondition: both raises carry the same %d life; a sum cannot be told from one permission", raises[0].Life)
	}
	if raises[0].Life != 3 {
		t.Fatalf("precondition: the first granting raise carries %d life, want Noctis's 3", raises[0].Life)
	}

	addMana(t, e, 0, "BBBB")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("priority = %+v, want a priority decision for seat 0", d)
	}
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == wicker.ID && o.Mode == "mayplay" {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("Wickerfolk's graveyard cast not offered under the two permissions: %+v", d.Options)
	}

	before := e.G.Players[0].Life
	submitChoices(t, e, castIdx)
	// Pre-fix the summed raise also owes Wickerfolk's sacrifice; fail on
	// that ask rather than letting a later drain answer it silently.
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		for _, o := range d.Options {
			if o.Obj == fodder.ID {
				t.Fatalf("the second permission's sacrifice was charged: %+v", d.Options)
			}
		}
	}
	passUntilStackEmpty(t, e, 50)

	if got := before - e.G.Players[0].Life; got != 3 {
		t.Fatalf("the cast paid %d life, want 3 (one permission's raise; 5 is the sum of both)", got)
	}
	if o := e.G.Obj(fodder.ID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("sacrifice candidate = %+v, want battlefield (only Noctis's 3-life raise applied)", o)
	}
	if o := e.G.Obj(wicker.ID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Wickerfolk = %+v, want battlefield", o)
	}
}
