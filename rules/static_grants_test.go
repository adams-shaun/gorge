package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// The static-grant family (task inbox-paramcensus-static-grant-misc): one
// leaf per Continuous grant sub-kind the parameter census named, each on a
// real corpus card driven through a real engine. The census labels these
// tests retire: Continuous.CharacteristicDefining (Master of Etherium),
// Continuous.AddStaticAbility (Exploration Broodship), Continuous.AddTrigger
// (Hearthhull, the Worldseed), Continuous.AddSVar (Sword of Fire and Ice),
// Continuous.MayLookAt (Oracle of Mul Daya).

// TestMasterOfEtheriumCDAIsItsArtifactCount is Continuous.Characteristic-
// Defining's leaf: the P/T-setting CDA sets the layer-7a base in EVERY zone
// (CR 604.3/208.2, CR 613.4a) -- the battlefield-only static scan can never
// express that, so cdaSetPT reads it directly off the face in derivedScalar.
// The value is the Count$Valid SVar, priced by the evaluator, NOT the
// statInt("X") zero the old emission degraded to.
func TestMasterOfEtheriumCDAIsItsArtifactCount(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	moe := lookup(t, reg, "Master of Etherium")
	thopter := lookup(t, reg, "Ornithopter")
	sol := lookup(t, reg, "Sol Ring")
	e := corpusEngine(t, reg, []*cards.Card{moe, thopter, sol}, nil)
	moeID := moveByName(t, e, 0, "Master of Etherium", state.ZBattlefield)
	thopterID := moveByName(t, e, 0, "Ornithopter", state.ZBattlefield)
	moveByName(t, e, 0, "Sol Ring", state.ZBattlefield)
	// Three artifacts you control (the CDA's count includes the card itself);
	// the +1/+1 the OTHER static grants reaches Ornithopter (another artifact
	// creature), never the CDA itself.
	if got := e.Power(moeID); got != 3 || e.Toughness(moeID) != 3 {
		t.Fatalf("Master of Etherium on the battlefield = %d/%d, want 3/3", e.Power(moeID), e.Toughness(moeID))
	}
	if got := e.Power(thopterID); got != 1 || e.Toughness(thopterID) != 3 {
		t.Fatalf("Ornithopter = %d/%d, want 1/3 (the other-artifact-creature buff)", e.Power(thopterID), e.Toughness(thopterID))
	}
	// All zones: in the graveyard the CDA still sets P/T to the artifacts you
	// control -- now two (the battlefield ones; a graveyard card controls
	// nothing).
	e.emit(events.Event{Kind: events.MoveZone, Obj: moeID, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.Power(moeID); got != 2 || e.Toughness(moeID) != 2 {
		t.Fatalf("Master of Etherium in the graveyard = %d/%d, want 2/2 (CDA applies in every zone)", e.Power(moeID), e.Toughness(moeID))
	}
}

// TestExplorationBroodshipGrantsItsStationStatic is Continuous.AddStatic-
// Ability's leaf: the STATION 3+ static's AddStaticAbility$ body is itself a
// Mode$ Continuous static (an AdjustLandPlays$ 1 to You), granted for
// exactly as long as the outer static's Affected$ matches the host. Below
// three charge counters the grant is not live.
func TestExplorationBroodshipGrantsItsStationStatic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	brood := lookup(t, reg, "Exploration Broodship")
	beast := card(t, "Name:BigBear\nManaCost:2 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n")
	e := corpusEngine(t, reg, []*cards.Card{brood, beast}, nil)
	broodID := moveByName(t, e, 0, "Exploration Broodship", state.ZBattlefield)
	beastID := moveByName(t, e, 0, "BigBear", state.ZBattlefield)
	if got := e.adjustLandPlays(0); got != 0 {
		t.Fatalf("adjustLandPlays before STATION 3+ = %d, want 0 (no grant below the counters)", got)
	}
	stationTo(t, e, broodID, beastID)
	if got := e.G.Obj(broodID).Counter("CHARGE"); got != 3 {
		t.Fatalf("the spacecraft has %d charge counters, want 3", got)
	}
	// The granted static is live: one additional land drop for its
	// controller, and the ordinary grant emission the printed AdjustLandPlays
	// statics share produced it.
	if got := e.adjustLandPlays(0); got != 1 {
		t.Fatalf("adjustLandPlays at STATION 3+ = %d, want 1 (the granted AdjustLandPlays$ 1)", got)
	}
	if got := e.adjustLandPlays(1); got != 0 {
		t.Fatalf("adjustLandPlays for the opponent = %d, want 0", got)
	}
}

// TestHearthhullGrantSacrificeTriggerFires is Continuous.AddTrigger's leaf:
// the STATION 8+ static's AddTrigger$ body is a T:-shaped trigger, granted
// to the spacecraft itself; sacrificing a land queues it (a GrantTriggerPush
// whose stack ability is the Execute$ SVar body) and it resolves, costing
// each opponent 2 life.
func TestHearthhullGrantSacrificeTriggerFires(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hh := lookup(t, reg, "Hearthhull, the Worldseed")
	beast := card(t, "Name:BigBeast\nManaCost:4 G\nTypes:Creature Beast\nPT:8/8\nOracle:x\n")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{hh, beast}, nil)
	hhID := moveByName(t, e, 0, "Hearthhull, the Worldseed", state.ZBattlefield)
	beastID := moveByName(t, e, 0, "BigBeast", state.ZBattlefield)
	stationTo(t, e, hhID, beastID)
	if got := e.G.Obj(hhID).Counter("CHARGE"); got != 8 {
		t.Fatalf("the spacecraft has %d charge counters, want 8", got)
	}
	if !e.HasKeyword(hhID, "Flying") {
		t.Fatal("the STATION 8+ AddKeyword$ grant is not live")
	}
	// The granted trigger, not a printed one: it queues off a sacrifice.
	land := moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	e.emit(events.Sacrifice(land))
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if n := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.GrantTriggerPush }); n != 1 {
		t.Fatalf("the granted trigger pushed %d GrantTriggerPush event(s), want 1", n)
	}
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("opponent life = %d, want 18 (the granted 'each opponent loses 2 life')", got)
	}
	replayCheck(t, e, cfg)
}

// TestSwordOfFireAndIceGrantsSVarToEquipped is Continuous.AddSVar's leaf:
// the AddSVar$ value is Forge's "SVar:<Name>:<Value>" grant shape; the
// equipped creature carries the granted variable while equipped and carries
// nothing once the Equipment detaches. The corpus's granted SVars are
// AI-evaluation hints, so the engine-side surface is Engine.GrantedSVar.
func TestSwordOfFireAndIceGrantsSVarToEquipped(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sword := lookup(t, reg, "Sword of Fire and Ice")
	bears := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{sword, bears}, nil)
	sw := moveByName(t, e, 0, "Sword of Fire and Ice", state.ZBattlefield)
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	if _, ok := e.GrantedSVar(bearID, "MustBeBlocked"); ok {
		t.Fatal("the unequipped creature already carries the granted SVar")
	}
	addMana(t, e, 0, "RR")
	e.Advance()
	submitChoices(t, e, abilityOption(t, e, sw, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) == 0 || d.Options[0].Obj != bearID {
		t.Fatalf("equip target %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(sw).AttachedTo != bearID {
		t.Fatalf("sword attached to %d, want %d", e.G.Obj(sw).AttachedTo, bearID)
	}
	v, ok := e.GrantedSVar(bearID, "MustBeBlocked")
	if !ok || v != "AttackingPlayerConservative" {
		t.Fatalf("granted SVar = (%q, %t), want (AttackingPlayerConservative, true)", v, ok)
	}
	// The grant follows the attachment: detach and it is gone.
	e.emit(events.Event{Kind: events.Attach, Obj: sw, IDs: []state.ObjID{}})
	e.checkStateBased()
	if _, ok := e.GrantedSVar(bearID, "MustBeBlocked"); ok {
		t.Fatal("the detached creature still carries the granted SVar")
	}
}

// TestOracleOfMulDayaRevealsTopCard is Continuous.MayLookAt's leaf: the
// grant's controller may look at the top card of their own library -- the
// seat's own projection carries LibraryTop, every other seat's carries
// nothing (CR 400.2).
func TestOracleOfMulDayaRevealsTopCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	oracle := lookup(t, reg, "Oracle of Mul Daya")
	e := corpusEngine(t, reg, []*cards.Card{oracle}, nil)
	if e.MayLookAtLibraryTop(0) {
		t.Fatal("the top card is revealed before the Oracle is on the battlefield")
	}
	moveByName(t, e, 0, "Oracle of Mul Daya", state.ZBattlefield)
	if !e.MayLookAtLibraryTop(0) {
		t.Fatal("Oracle of Mul Daya does not reveal the top card to its controller")
	}
	if e.MayLookAtLibraryTop(1) {
		t.Fatal("the grant leaked to a seat it does not affect")
	}
	top := e.G.Zone(state.ZLibrary, 0)[0]
	own := view.Project(e.G, e, 0, e.Pending())
	if own.Players[0].LibraryTop == nil || own.Players[0].LibraryTop.ID != top {
		t.Fatalf("seat 0's view carries no revealed top card (want %d): %+v", top, own.Players[0].LibraryTop)
	}
	other := view.Project(e.G, e, 1, e.Pending())
	if other.Players[0].LibraryTop != nil {
		t.Fatalf("seat 1's view leaks seat 0's revealed top card: %+v", other.Players[0].LibraryTop)
	}
}

// TestNumericSetMaxHandSize pins the numeric SetMaxHandSize$ shape on the
// read maxHandSizeFor already implements (Reliquary Tower's Unlimited landed
// first): five, not the seven-card default. Necrodominance carries the
// printed card shape.
func TestNumericSetMaxHandSize(t *testing.T) {
	e := layerEngine(t)
	onBoard(t, e, 0, "Name:Necrodominance\nManaCost:B B B\nTypes:Legendary Enchantment\n"+
		"S:Mode$ Continuous | Affected$ You | SetMaxHandSize$ 5 | Description$ Your maximum hand size is five.\nOracle:x\n")
	if got := e.maxHandSizeFor(0); got != 5 {
		t.Fatalf("maxHandSizeFor under SetMaxHandSize$ 5 = %d, want 5", got)
	}
	if got := e.maxHandSizeFor(1); got != 7 {
		t.Fatalf("maxHandSizeFor for the unaffected seat = %d, want the 7 default", got)
	}
}

// stationTo is the kw:Station flow the mass-primitives leaves use: the
// sorcery-window station option for craft, answered with the tap pick over
// bigID, leaving its power on craft as charge counters.
func stationTo(t *testing.T, e *Engine, craft, bigID state.ObjID) {
	t.Helper()
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision (got %+v)", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "station" && o.Obj == craft {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no station option for %d: %+v", craft, d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit station: %v", err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("no tap pick after the station option (got %+v)", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == bigID {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("the station tap pick does not offer %d: %+v", bigID, d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
		t.Fatalf("submit tap pick: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
}
