package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The has-all-abilities-of static class (Forge's GainsAbilitiesOf$ /
// GainsTriggerAbsOf$ on a Mode$ Continuous static) pinned on the real corpus
// carrier Idris, Soul of the TARDIS, whose static is
//
//	S:Mode$ Continuous | Affected$ Card.Self | EffectZone$ Battlefield |
//	  GainsAbilitiesOf$ Card.ExiledWithSource |
//	  GainsTriggerAbsOf$ Card.ExiledWithSource | GainsAbilitiesOfZones$ Exile |
//	  AddPower$ X | AddToughness$ X
//	SVar:X:Count$ValidExile Card.ExiledWithSource$CardManaCost
//
// The exiled card is an AUTHORED fixture artifact (never a corpus .txt, per
// the licensing rule) carrying one activated ability and one triggered
// ability, so every assertion runs through the engine's own offer, activation
// and trigger paths against Idris's compiled static.

// gainsArtifactSrc is the exiled artifact: "{T}: Draw a card." plus "At the
// beginning of your upkeep, you lose 2 life." -- one activated and one
// triggered ability, each independently observable.
func gainsArtifactSrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Gains Artifact\nManaCost:2\nTypes:Artifact\n"+
		"A:AB$ Draw | Cost$ T | NumCards$ 1 | SpellDescription$ Draw a card.\n"+
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | Execute$ TrigBleed | TriggerDescription$ At the beginning of your upkeep, you lose 2 life.\n"+
		"SVar:TrigBleed:DB$ LoseLife | LifeAmount$ 2 | Defined$ You\n"+
		"Oracle:x\n")
}

// gainsBoard seeds Idris (the real corpus carrier) and the authored artifact
// onto seat 0's battlefield, then exiles the artifact with Idris as the
// exiling source through a logged MoveZone carrying the provenance in IDs
// (events' moveZoneEvent shape), and returns (engine, cfg, idris, artifact).
func gainsBoard(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	idris := tokenReplCorpusCard(t, "Idris, Soul of the TARDIS")
	artifact := gainsArtifactSrc(t)
	e, cfg := tokenReplGame(t, 9107, idris, artifact)
	idrisID := moveSeededCard(t, e, 0, idris, state.ZBattlefield)
	artifactID := moveSeededCard(t, e, 0, artifact, state.ZBattlefield)
	// Exile the artifact with Idris as the exiling source: the MoveZone IDs
	// payload is the exact provenance carrier effects' moveZoneEvent uses,
	// so ExiledWith == Idris afterwards and Card.ExiledWithSource matches.
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{idrisID}})
	e.pending = nil
	if o := e.G.Obj(artifactID); o == nil || o.Zone != state.ZExile || o.ExiledWith != idrisID {
		t.Fatalf("artifact = %+v, want exiled with %d", o, idrisID)
	}
	e.priorityRound()
	return e, cfg, idrisID, artifactID
}

// TestIdrisGainsTheExiledArtifactAbilities is the filing card end to end: the
// exiled artifact's triggered ability fires for Idris's controller at the
// next upkeep, and its activated ability is offered on Idris (once Idris is
// no longer summoning-sick) and draws a card when activated.
func TestIdrisGainsTheExiledArtifactAbilities(t *testing.T) {
	e, cfg, idrisID, _ := gainsBoard(t)

	// Precondition: Idris is on the battlefield and owns the gains static.
	if o := e.G.Obj(idrisID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Idris = %+v, want on the battlefield", o)
	}

	// The granted TRIGGERED ability fires for Idris's controller at seat 0's
	// next own upkeep (turn 3; Idris entered on turn 1, so this is not a
	// summoning-sickness-dependent step).
	lifeBefore := e.G.Players[0].Life
	driveToStep(t, e, 3, 0, state.StepUpkeep)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != lifeBefore-2 {
		t.Fatalf("gained upkeep trigger: life %d -> %d, want -2", lifeBefore, got)
	}
	if n := countKind(e.L.Events, events.GainedTriggerPush, idrisID); n != 1 {
		t.Fatalf("GainedTriggerPush count = %d, want 1", n)
	}

	// The granted activated ability is offered on Idris once Idris can tap
	// (turn 3's main phase), anchored on the foreign card. Idris's own
	// AddPower$/AddToughness$ static (X = the exiled card's mana value) is the
	// same Card.ExiledWithSource read the grant uses, so assert it too.
	driveToStep(t, e, 3, 0, state.StepMain1)
	if p, tg := e.Power(idrisID), e.Toughness(idrisID); p != 5 || tg != 5 {
		t.Fatalf("Idris P/T = %d/%d, want 5/5 (3/3 base +2/+2 from the exiled mana value)", p, tg)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want priority", d)
	}
	var opt decision.Option
	found := false
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == idrisID && o.GainedSource != 0 {
			opt, found = o, true
			break
		}
	}
	if !found {
		t.Fatalf("Idris offers no gained ability: %+v", d.Options)
	}
	if o := e.G.Obj(opt.GainedSource); o == nil || o.Zone != state.ZExile {
		t.Fatalf("gained anchor = %d, want the exiled artifact", opt.GainedSource)
	}

	// Activate it: the hand size grows by one from the granted draw.
	handBefore := len(e.G.Zone(state.ZHand, 0))
	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	handAfter := len(e.G.Zone(state.ZHand, 0))
	if handAfter != handBefore+1 || len(e.G.Zone(state.ZLibrary, 0)) != libBefore-1 {
		t.Fatalf("granted draw: hand %d->%d library %d->%d, want +1/-1",
			handBefore, handAfter, libBefore, len(e.G.Zone(state.ZLibrary, 0)))
	}
	if n := countKind(e.L.Events, events.GainedAbilityPush, idrisID); n != 1 {
		t.Fatalf("GainedAbilityPush count = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestIdrisGainedAbilitiesEndWhenTheExiledCardLeaves pins the fail-closed
// liveness: once the exiled card leaves the scoped zone (Exile), Idris stops
// offering its abilities, because the static scan re-derives the named set on
// every event.
func TestIdrisGainedAbilitiesEndWhenTheExiledCardLeaves(t *testing.T) {
	e, cfg, idrisID, artifactID := gainsBoard(t)
	// Advance past summoning sickness so the granted tap ability is offerable
	// (otherwise this precondition would pass vacuously via the tap gate).
	driveToStep(t, e, 3, 0, state.StepMain1)
	if !hasGainedAbility(e, idrisID) {
		t.Fatal("precondition: Idris does not offer the gained ability before the move")
	}
	// Move the artifact out of exile (to the graveyard): no provenance there.
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZExile, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	if hasGainedAbility(e, idrisID) {
		t.Fatalf("Idris still offers a gained ability after the exiled card left: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)
}

// TestIdrisDoesNotGainUnrelatedExiledCards pins the Card.ExiledWithSource
// scoping: a card exiled by something ELSE is not gained.
func TestIdrisDoesNotGainUnrelatedExiledCards(t *testing.T) {
	e, cfg, idrisID, artifactID := gainsBoard(t)
	driveToStep(t, e, 3, 0, state.StepMain1)
	if !hasGainedAbility(e, idrisID) {
		t.Fatal("precondition: Idris does not offer the gained ability before the re-exile")
	}
	// Re-exile the artifact with no exiling source so ExiledWith no longer
	// names Idris.
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZExile, To: state.ZHand})
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZHand, To: state.ZExile})
	e.pending = nil
	e.priorityRound()
	if o := e.G.Obj(artifactID); o == nil || o.ExiledWith != 0 {
		t.Fatalf("artifact ExiledWith = %v, want 0 after a fresh exile with no source", o)
	}
	if hasGainedAbility(e, idrisID) {
		t.Fatal("Idris gained an ability from a card it did not exile")
	}
	replayCheck(t, e, cfg)
}

// hasGainedAbility reports whether the pending priority decision offers a
// gained ability on obj.
func hasGainedAbility(e *Engine, obj state.ObjID) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == obj && o.GainedSource != 0 {
			return true
		}
	}
	return false
}
