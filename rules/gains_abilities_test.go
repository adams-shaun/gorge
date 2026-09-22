package rules

import (
	"strings"
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

// gainsManaArtifactSrc is an exiled artifact carrying only a mana ability:
// "{T}: Add {B}."
func gainsManaArtifactSrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Gains Mana Artifact\nTypes:Artifact\n"+
		"A:AB$ Mana | Cost$ T | Produced$ B | SpellDescription$ Add {B}.\n"+
		"Oracle:x\n")
}

// TestIdrisGainsTheExiledArtifactManaAbility pins the class's mana-ability
// route (a distinct activation path from beginActivation): the exiled
// artifact's mana ability is offered on Idris as an "activate" option and
// produces mana when activated.
func TestIdrisGainsTheExiledArtifactManaAbility(t *testing.T) {
	idris := tokenReplCorpusCard(t, "Idris, Soul of the TARDIS")
	artifact := gainsManaArtifactSrc(t)
	e, cfg := tokenReplGame(t, 9108, idris, artifact)
	idrisID := moveSeededCard(t, e, 0, idris, state.ZBattlefield)
	artifactID := moveSeededCard(t, e, 0, artifact, state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{idrisID}})
	e.pending = nil
	e.priorityRound()

	driveToStep(t, e, 3, 0, state.StepMain1)
	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision")
	}
	var opt decision.Option
	found := false
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == idrisID {
			opt, found = o, true
			break
		}
	}
	if !found {
		t.Fatalf("Idris offers no gained mana ability: %+v", d.Options)
	}
	submitChoices(t, e, opt.Index)
	if got := e.G.Players[0].Pool[state.MB]; got != 1 {
		t.Fatalf("gained mana ability produced %d black, want 1", got)
	}
	if o := e.G.Obj(idrisID); o == nil || !o.Tapped {
		t.Fatalf("Idris = %+v, want tapped by the gained mana ability", o)
	}
	replayCheck(t, e, cfg)
}

// countersOf reads one counter kind's total off an object.
func countersOf(o *state.Object, kind string) int32 {
	for _, c := range o.Counters {
		if c.Kind == kind {
			return c.N
		}
	}
	return 0
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

// gainedAbilitiesOn returns the pending priority decision's gained-ability
// options on obj (the whole options, so a caller can count them and read
// labels).
func gainedAbilitiesOn(e *Engine, obj state.ObjID) []decision.Option {
	d := e.Pending()
	if d == nil {
		return nil
	}
	var out []decision.Option
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == obj && o.GainedSource != 0 {
			out = append(out, o)
		}
	}
	return out
}

// gainsAbilitiesOnlySrc is the REAL corpus carrier Territory Forge, whose
// static names GainsAbilitiesOf$ and NO GainsTriggerAbsOf$ -- the
// Mairsil-the-Pretender class shape the round-2 review broke (a
// GainsAbilitiesOf-only carrier must never fire the foreign card's phase
// triggers, because Forge's parameter grants activated abilities only).
func gainsForgeSrc(t *testing.T) *cards.Card {
	t.Helper()
	return tokenReplCorpusCard(t, "Territory Forge")
}

// TestGainsAbilitiesOnlyNeverFiresForeignTriggers pins the parameter split in
// the TRIGGER direction: Territory Forge gains the exiled card's ACTIVATED
// ability (its parameter means exactly that) but never its upkeep trigger --
// without the GainedTriggerFaces split the foreign trigger fired from the
// gains static alone.
func TestGainsAbilitiesOnlyNeverFiresForeignTriggers(t *testing.T) {
	forge := gainsForgeSrc(t)
	artifact := gainsArtifactSrc(t)
	e, cfg := tokenReplGame(t, 9109, forge, artifact)
	forgeID := moveSeededCard(t, e, 0, forge, state.ZBattlefield)
	artifactID := moveSeededCard(t, e, 0, artifact, state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{forgeID}})
	e.pending = nil
	if o := e.G.Obj(artifactID); o == nil || o.Zone != state.ZExile || o.ExiledWith != forgeID {
		t.Fatalf("artifact = %+v, want exiled with %d", o, forgeID)
	}
	e.priorityRound()

	// Precondition: the grant EXISTS in the activated half -- Territory Forge
	// is an artifact, so the {T} draw is offerable on turn 1 with no
	// summoning-sickness gate in the way. Without this assertion the negative
	// below could pass on a grant that never resolved at all.
	driveToStep(t, e, 1, 0, state.StepMain1)
	if !hasGainedAbility(e, forgeID) {
		t.Fatal("precondition: Territory Forge does not offer the gained activated ability")
	}

	// The foreign card's upkeep trigger never fires: life is unchanged and no
	// GainedTriggerPush was minted.
	lifeBefore := e.G.Players[0].Life
	driveToStep(t, e, 3, 0, state.StepUpkeep)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != lifeBefore {
		t.Fatalf("GainsAbilitiesOf-only fired the foreign upkeep trigger: life %d -> %d, want unchanged", lifeBefore, got)
	}
	if n := countKind(e.L.Events, events.GainedTriggerPush, forgeID); n != 0 {
		t.Fatalf("GainedTriggerPush count = %d, want 0 on a GainsAbilitiesOf-only static", n)
	}
	replayCheck(t, e, cfg)
}

// gainsTriggerOnlySrc is an AUTHORED carrier (never a corpus .txt, per the
// licensing rule) whose static names GainsTriggerAbsOf$ and NO
// GainsAbilitiesOf$ -- the mirror-image parameter split.
func gainsTriggerOnlySrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Gains Triggers Only\nManaCost:3\nTypes:Artifact\n"+
		"S:Mode$ Continuous | Affected$ Card.Self | EffectZone$ Battlefield | GainsTriggerAbsOf$ Card.ExiledWithSource | GainsAbilitiesOfZones$ Exile\n"+
		"Oracle:x\n")
}

// TestGainsTriggerAbsOnlyNeverGrantsForeignActivatedAbilities pins the
// parameter split in the ACTIVATED direction: a GainsTriggerAbsOf-only static
// fires the foreign card's upkeep trigger (precondition, so the grant is
// live) but never offers its activated ability.
func TestGainsTriggerAbsOnlyNeverGrantsForeignActivatedAbilities(t *testing.T) {
	carrier := gainsTriggerOnlySrc(t)
	artifact := gainsArtifactSrc(t)
	e, cfg := tokenReplGame(t, 9110, carrier, artifact)
	carrierID := moveSeededCard(t, e, 0, carrier, state.ZBattlefield)
	artifactID := moveSeededCard(t, e, 0, artifact, state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{carrierID}})
	e.pending = nil
	e.priorityRound()

	// Precondition: the TRIGGERED grant is live -- the foreign upkeep trigger
	// fires for seat 0 at its turn-3 upkeep.
	lifeBefore := e.G.Players[0].Life
	driveToStep(t, e, 3, 0, state.StepUpkeep)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != lifeBefore-2 {
		t.Fatalf("gained upkeep trigger: life %d -> %d, want -2 (the grant itself must be live)", lifeBefore, got)
	}
	if n := countKind(e.L.Events, events.GainedTriggerPush, carrierID); n != 1 {
		t.Fatalf("GainedTriggerPush count = %d, want 1", n)
	}

	// The negative: the foreign card's ACTIVATED ability is never offered.
	driveToStep(t, e, 3, 0, state.StepMain1)
	if hasGainedAbility(e, carrierID) {
		t.Fatal("GainsTriggerAbsOf-only offered the foreign card's activated ability")
	}
	replayCheck(t, e, cfg)
}

// gainsRivalLandSrc is the AUTHORED opponent land the Sharkey test gains
// from: one mana ability ("{T}: Add {G}.") and one non-mana activated
// ability ("{T}: Draw a card.") -- Sharkey's GainsValidAbilities$
// Activated.!ManaAbility must admit exactly the second.
func gainsRivalLandSrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Rival Land\nTypes:Land\n"+
		"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.\n"+
		"A:AB$ Draw | Cost$ T | NumCards$ 1 | SpellDescription$ Draw a card.\n"+
		"Oracle:x\n")
}

// TestGainsValidAbilitiesExcludesForeignManaAbilities pins the grant's
// GainsValidAbilities$ filter on the real corpus carrier Sharkey, Tyrant of
// the Shire (`GainsAbilitiesOf$ Land.OppCtrl |
// GainsValidAbilities$ Activated.!ManaAbility`): the opponent land's
// non-mana activated ability IS gained and offered, its mana ability is NOT --
// neither on the ability offer nor in the mana-ability collector the payment
// window reads.
func TestGainsValidAbilitiesExcludesForeignManaAbilities(t *testing.T) {
	sharkey := tokenReplCorpusCard(t, "Sharkey, Tyrant of the Shire")
	land := gainsRivalLandSrc(t)
	e, cfg := tokenReplGameSeats(t, 9111, []*cards.Card{sharkey}, []*cards.Card{land})
	sharkeyID := moveSeededCard(t, e, 0, sharkey, state.ZBattlefield)
	landID := moveSeededCard(t, e, 1, land, state.ZBattlefield)
	e.priorityRound()

	// Sharkey is a creature: drive past summoning sickness so the {T} gained
	// ability is offerable at all.
	driveToStep(t, e, 3, 0, state.StepMain1)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want priority", d)
	}
	gained := gainedAbilitiesOn(e, sharkeyID)
	if len(gained) != 1 {
		t.Fatalf("Sharkey gained options = %d (%+v), want exactly the non-mana draw", len(gained), gained)
	}
	if o := e.G.Obj(gained[0].GainedSource); o == nil || o.ID != landID || o.Zone != state.ZBattlefield {
		t.Fatalf("gained anchor = %d, want the opponent's land on the battlefield", gained[0].GainedSource)
	}
	if got := e.availableManaAbilities(0, sharkeyID); len(got) != 0 {
		t.Fatalf("Sharkey's mana abilities include the foreign land's: %v, want none", got)
	}
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == sharkeyID {
			t.Fatalf("Sharkey offered a mana activation of the foreign land's ability: %+v", o)
		}
	}
	replayCheck(t, e, cfg)
}

// gainsRivalWalkerSrc is the AUTHORED other planeswalker the Nicol Bolas test
// gains from: one loyalty ability (+1: draw) and one NON-loyalty activated
// ability ({2}: draw two) -- Nicol Bolas Dragon-God's
// GainsValidAbilities$ Activated.Loyalty must admit only the first.
func gainsRivalWalkerSrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Rival Walker\nManaCost:3\nTypes:Planeswalker\nLoyalty:4\n"+
		"A:AB$ Draw | Cost$ AddCounter<1/LOYALTY> | NumCards$ 1 | Planeswalker$ True | SpellDescription$ +1: Draw a card.\n"+
		"A:AB$ Draw | Cost$ T | NumCards$ 2 | SpellDescription$ {T}: Draw two cards.\n"+
		"Oracle:x\n")
}

// TestGainsValidAbilitiesLoyaltyOnlyAdmitsOnlyLoyalty pins the other filter
// direction on the real corpus carrier Nicol Bolas, Dragon-God
// (`GainsAbilitiesOf$ Planeswalker.Other |
// GainsValidAbilities$ Activated.Loyalty`): of the foreign walker's two
// activated abilities exactly the loyalty one is gained.
func TestGainsValidAbilitiesLoyaltyOnlyAdmitsOnlyLoyalty(t *testing.T) {
	bolas := tokenReplCorpusCard(t, "Nicol Bolas, Dragon-God")
	walker := gainsRivalWalkerSrc(t)
	e, cfg := tokenReplGame(t, 9112, bolas, walker)
	bolasID := moveSeededCard(t, e, 0, bolas, state.ZBattlefield)
	walkerID := moveSeededCard(t, e, 0, walker, state.ZBattlefield)
	e.priorityRound()

	// Precondition: both permanents are on the battlefield and the foreign
	// walker entered with its printed loyalty (so it is a live planeswalker,
	// not a 0-loyalty shell).
	if o := e.G.Obj(walkerID); o == nil || o.Zone != state.ZBattlefield || countersOf(o, "LOYALTY") != 4 {
		t.Fatalf("Rival Walker = %+v, want on the battlefield with 4 loyalty", o)
	}
	driveToStep(t, e, 1, 0, state.StepMain1)
	gained := gainedAbilitiesOn(e, bolasID)
	if len(gained) != 1 {
		t.Fatalf("Bolas gained options = %d (%+v), want exactly the foreign loyalty ability", len(gained), gained)
	}
	if gained[0].GainedSource != walkerID {
		t.Fatalf("gained anchor = %d, want the rival walker", gained[0].GainedSource)
	}
	// The admitted ability is the +1 loyalty one, not the {2} draw-two: the
	// option label carries the foreign ability's SpellDescription.
	if !strings.Contains(gained[0].Label, "+1: Draw a card.") {
		t.Fatalf("gained option label = %q, want the loyalty ability's text", gained[0].Label)
	}
	replayCheck(t, e, cfg)
}

// gainsMairsilFixtureSrc is an AUTHORED carrier carrying Mairsil the
// Pretender's exact static line (GainsAbilitiesOf$ Card.YouOwn+counters_GE1_CAGE
// | GainsAbilitiesOfZones$ Exile | GainsAbilitiesLimitPerTurn$ 1) and nothing
// else -- no ETB trigger, no own abilities, so the per-turn cap is the only
// thing under test.
func gainsMairsilFixtureSrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Cage Warden\nManaCost:2\nTypes:Creature\nPT:2/2\n"+
		"S:Mode$ Continuous | Affected$ Card.Self | EffectZone$ Battlefield | GainsAbilitiesOf$ Card.YouOwn+counters_GE1_CAGE | GainsAbilitiesOfZones$ Exile | GainsAbilitiesLimitPerTurn$ 1\n"+
		"Oracle:x\n")
}

// gainsCagedCardSrc is the AUTHORED caged card: one activated ability, "{2}:
// Draw a card." -- a NON-tap cost, so the tap gate cannot mask the per-turn
// limit after the first activation taps the recipient; the test floats {2}
// before every offer check instead.
func gainsCagedCardSrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Caged Grimoire\nManaCost:2\nTypes:Artifact\n"+
		"A:AB$ Draw | Cost$ 2 | NumCards$ 1 | SpellDescription$ {2}: Draw a card.\n"+
		"Oracle:x\n")
}

// gainsDriveToStep is driveToStep with the two extra decision kinds this
// file's longer drives cross: an attackers declaration (declined -- no
// attack) and a cleanup discard (first-Max, answerIfDiscard).
func gainsDriveToStep(t *testing.T, e *Engine, turn int32, active state.PlayerID, step state.Step) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		if e.G.Turn == turn && e.G.Active == active && e.G.Step == step {
			return
		}
		if e.G.Over {
			t.Fatalf("game ended before reaching turn %d seat %d step %s", turn, active, step)
		}
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority && d.Kind != decision.KAttackers {
			t.Fatalf("non-priority decision %+v encountered while driving to turn %d seat %d step %s",
				d, turn, active, step)
		}
		if d.Kind == decision.KAttackers {
			submitAttackersOnly(t, e)
			continue
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	t.Fatalf("did not reach turn %d seat %d step %s", turn, active, step)
}

// TestGainsLimitPerTurnCapsEachForeignAbilityPerTurn pins the grant's
// GainsAbilitiesLimitPerTurn$ 1: the caged card's ability is offered, offered
// again the NEXT turn after the reset, and withheld for the rest of the turn
// it was activated in.
func TestGainsLimitPerTurnCapsEachForeignAbilityPerTurn(t *testing.T) {
	warden := gainsMairsilFixtureSrc(t)
	caged := gainsCagedCardSrc(t)
	e, cfg := tokenReplGame(t, 9113, warden, caged)
	wardenID := moveSeededCard(t, e, 0, warden, state.ZBattlefield)
	cagedID := moveSeededCard(t, e, 0, caged, state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: cagedID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{wardenID}})
	e.emit(events.Event{Kind: events.CounterChange, Obj: cagedID, Counter: "CAGE", Amount: 1})
	e.pending = nil
	if o := e.G.Obj(cagedID); o == nil || o.Zone != state.ZExile || o.ExiledWith != wardenID || countersOf(o, "CAGE") != 1 {
		t.Fatalf("caged card = %+v, want in exile with provenance and 1 CAGE counter", o)
	}
	e.priorityRound()

	// The warden is a creature: drive past summoning sickness so the gained
	// ability is offerable at all, then float the {2} its cost needs.
	driveToStep(t, e, 3, 0, state.StepMain1)
	addMana(t, e, 0, "CC")
	gained := gainedAbilitiesOn(e, wardenID)
	if len(gained) != 1 {
		t.Fatalf("precondition: Warden gained options = %d, want the caged draw offered", len(gained))
	}
	submitChoices(t, e, gained[0].Index)
	passUntilStackEmpty(t, e, 20)
	if n := countKind(e.L.Events, events.GainedAbilityPush, wardenID); n != 1 {
		t.Fatalf("GainedAbilityPush count = %d, want 1 (the cap counts the activation itself)", n)
	}

	// Same turn: the cap binds and the ability is withheld. Float {2} again
	// so an affordability gate cannot fake the withholding.
	addMana(t, e, 0, "CC")
	e.priorityRound()
	if hasGainedAbility(e, wardenID) {
		t.Fatal("GainsAbilitiesLimitPerTurn$ 1 did not withhold the ability after its one activation")
	}

	// Seat 0's next turn: the per-turn window reset and the ability is back.
	// The drive crosses two combats (turn 3 seat 0 and turn 4 seat 1), so it
	// needs a drive that can decline an attackers declaration.
	gainsDriveToStep(t, e, 5, 0, state.StepMain1)
	addMana(t, e, 0, "CC")
	if !hasGainedAbility(e, wardenID) {
		t.Fatal("the per-turn cap did not reset at TurnChange -- the ability is gone for good")
	}
	replayCheck(t, e, cfg)
}
