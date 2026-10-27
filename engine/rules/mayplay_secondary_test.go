package rules

// Secondary$ on a may-play static is Forge's text-dedup marker, never a
// rules rider: CardTraitBase.isSecondary() is read only by Card.java's
// text renderers and LandAbility.java. Both printed carriers in the corpus
// (Radha, Heart of Keld and Coram, the Undertaker's nonland static) must
// therefore grant exactly as their Oracle text says. The Effect-delivered
// route already accepted the key and stays unchanged.

import (
	"maps"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// secondaryMayPlayStatic returns the printed `MayPlay$ True | Secondary$
// True` static hosted by obj, failing when the fixture has none.
func secondaryMayPlayStatic(t *testing.T, e *Engine, obj state.ObjID) cards.Static {
	t.Helper()
	for _, st := range e.G.Obj(obj).Face().Statics {
		if st.Params["MayPlay"] == "True" && st.Params["Secondary"] == "True" {
			return st
		}
	}
	t.Fatalf("obj %d carries no printed MayPlay$ True + Secondary$ True static", obj)
	return cards.Static{}
}

// TestMayPlaySecondaryRadhaPlaysLandFromTop pins the printed route: Radha's
// `MayPlay$ True | Secondary$ True` top-of-library land grant offers the top
// land and the offer really plays it. Under M1 (restoring the
// allowAfterStack guard on Secondary$) the mayPlayGrant precondition and the
// option both disappear.
func TestMayPlaySecondaryRadhaPlaysLandFromTop(t *testing.T) {
	t.Parallel()
	e, cfg, ids := mpRiderGame(t, nil, nil, []string{"Radha, Heart of Keld"}, nil)
	radha := ids[0]["Radha, Heart of Keld"][0]
	toMain1(t, e)
	e.priorityRound()

	top := e.G.Zone(state.ZLibrary, 0)[0]
	if top == 0 || top == radha {
		t.Fatalf("precondition: no top-of-library card (top %d, Radha %d)", top, radha)
	}
	if o := e.G.Obj(top); o == nil || o.Zone != state.ZLibrary || o.Face() == nil || !o.Face().IsLand() {
		t.Fatalf("precondition: top card %d is not a land in the library: %+v", top, e.G.Obj(top))
	}
	if e.G.Obj(radha).Zone != state.ZBattlefield || e.G.Turn != 1 || e.G.Active != 0 ||
		e.G.Step != state.StepMain1 || e.G.Players[0].LandsPlayed != 0 {
		t.Fatalf("precondition: Radha must be on seat 0's battlefield in turn-1 main 1 with no land played")
	}
	if _, ok := e.mayPlayGrant(0, top); !ok {
		t.Fatalf("precondition: Radha's printed Secondary$ static grants no may-play permission")
	}
	if !mayPlayGrant(secondaryMayPlayStatic(t, e, radha)) {
		t.Fatalf("Radha's printed Secondary$ static is refused by the may-play whitelist")
	}
	if !playOneLand(t, e, 0, top) {
		t.Fatalf("Radha's top land was not offered: %+v", e.Pending().Options)
	}
	if z := e.G.Obj(top).Zone; z != state.ZBattlefield {
		t.Fatalf("played top land zone = %s, want battlefield", z)
	}
	replayCheck(t, e, cfg)
}

// TestMayPlaySecondaryCoramSecondStaticGrants pins the second printed
// carrier: Coram's graveyard-cast static carries Secondary$ and must still
// offer a nonland that entered the graveyard from the library this turn.
func TestMayPlaySecondaryCoramSecondStaticGrants(t *testing.T) {
	t.Parallel()
	e, cfg, ids := mpRiderGame(t, []string{"Grizzly Bears"}, nil,
		[]string{"Coram, the Undertaker"}, nil)
	coram := ids[0]["Coram, the Undertaker"][0]
	bear := ids[0]["Grizzly Bears"][0]
	// A logged hand->library->graveyard move is the "put there from a
	// library this turn" provenance Coram's filter reads.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZHand, To: state.ZLibrary})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZLibrary, To: state.ZGraveyard})
	addMana(t, e, 0, "GG")

	o := e.G.Obj(bear)
	if o == nil || o.Zone != state.ZGraveyard || !o.EnteredThisTurn || o.EnteredFrom != state.ZLibrary ||
		o.Face().IsLand() {
		t.Fatalf("precondition: bear must be a nonland in the graveyard, entered this turn from the library: %+v", o)
	}
	if e.G.Obj(coram).Zone != state.ZBattlefield || e.G.Turn != 1 || e.G.Active != 0 {
		t.Fatalf("precondition: Coram must be on seat 0's battlefield during seat 0's turn")
	}
	if !mayPlayGrant(secondaryMayPlayStatic(t, e, coram)) {
		t.Fatalf("Coram's printed Secondary$ static is refused by the may-play whitelist")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision, got %+v", d)
	}
	found := false
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == bear {
			found = true
		}
	}
	if !found {
		t.Fatalf("Coram's Secondary$ graveyard grant did not offer the bear: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestMayPlaySecondaryStillRefusesValidSA pins the fail-closed sibling: the
// text marker is carried, but ValidSA$ remains an unread gate (Ninja Teen's
// static), so a printed `MayPlay$ True | ValidSA$ Spell | Secondary$ True`
// still grants nothing.
func TestMayPlaySecondaryStillRefusesValidSA(t *testing.T) {
	t.Parallel()
	const src = "Name:Test ValidSA Grant\nManaCost:1\nTypes:Artifact\n" +
		"S:Mode$ Continuous | Affected$ Card.YouOwn | AffectedZone$ Graveyard | MayPlay$ True | ValidSA$ Spell | Secondary$ True | Description$ x\n" +
		"Oracle:x\n"
	e, cfg, grantor := newFixtureDeck(t, 8318, src,
		"Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	placeOnBattlefield(t, e, grantor)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard)
	addMana(t, e, 0, "GG")

	if o := e.G.Obj(grantor); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the grantor is not on the battlefield: %+v", o)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard || !o.Face().IsPermanent() || o.Face().IsLand() {
		t.Fatalf("precondition: the bear is not a nonland card in the graveyard: %+v", o)
	}
	var params map[string]string
	for _, st := range e.G.Obj(grantor).Face().Statics {
		if st.Params["ValidSA"] == "Spell" {
			params = st.Params
		}
	}
	if params == nil {
		t.Fatal("precondition: the fixture's ValidSA$ static is missing")
	}
	if applies, grants, _, _, _, _ := e.mayPlayStatic(params, bear, 0, grantor); applies || grants {
		t.Fatalf("ValidSA$+Secondary$ static granted through mayPlayStatic: applies=%v grants=%v", applies, grants)
	}
	// Positive control: the SAME static without ValidSA$ does grant, so the
	// refusal above is the gate and not a broken fixture.
	open := maps.Clone(params)
	delete(open, "ValidSA")
	if applies, grants, _, _, _, _ := e.mayPlayStatic(open, bear, 0, grantor); !applies || !grants {
		t.Fatalf("positive control: static without ValidSA$ did not grant: applies=%v grants=%v", applies, grants)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision, got %+v", d)
	}
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == bear {
			t.Fatalf("ValidSA$-gated Secondary$ static offered the bear: %+v", d.Options)
		}
	}
	replayCheck(t, e, cfg)
}

// TestMayPlaySecondaryEffectRouteUnchanged pins the route that already
// carried Secondary$: Glamorous Outlaw's hand ability exiles it and the
// registered Effect grants the cast from exile.
func TestMayPlaySecondaryEffectRouteUnchanged(t *testing.T) {
	t.Parallel()
	e, cfg, ids := mpRiderGame(t, []string{"Glamorous Outlaw"}, nil, []string{"Island"}, nil)
	outlaw := ids[0]["Glamorous Outlaw"][0]
	land := ids[0]["Island"][0]
	addMana(t, e, 0, "CC")

	opt := abilityOption(t, e, outlaw, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Glamorous Outlaw's activation did not ask for its land target: %+v", d)
	}
	target := -1
	for _, o := range d.Options {
		if o.Obj == land {
			target = o.Index
		}
	}
	if target < 0 {
		t.Fatalf("the activation's target options do not name the Island: %+v", d.Options)
	}
	submitChoices(t, e, target)
	passUntilStackEmpty(t, e, 60)

	if o := e.G.Obj(outlaw); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: the hand ability did not exile Glamorous Outlaw: %+v", o)
	}
	if !e.mayPlayEffectGrantsCast(0, e.G.Obj(outlaw)) {
		t.Fatal("the Effect-delivered may-play grant no longer covers the exiled Glamorous Outlaw")
	}
	addMana(t, e, 0, "CCCBUR")
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision, got %+v", d)
	}
	found := false
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == outlaw {
			found = true
		}
	}
	if !found {
		t.Fatalf("Glamorous Outlaw is not offered for casting from exile: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}
