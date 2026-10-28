package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cantTargetOnBoard asserts the precondition every CantTarget test needs: id
// is on the battlefield and its printed/AffectedZone$-scoped static is in the
// live collection the matcher walks.
func cantTargetOnBoard(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: object %d is not on the battlefield: %+v", id, o)
	}
	for _, sv := range e.activeStatics("CantTarget") {
		if sv.Source == id {
			return
		}
	}
	t.Fatalf("precondition: object %d has no live CantTarget static: %+v", id, o.Face())
}

// TestThrunCantBeTargetedByOpponentsNongreenSpell pins the printed-static
// delivery path (stat:CantTarget was never registered before this change):
// Thrun, Breaker of Silence is not a legal target of an opponent's Lightning
// Bolt, while the opponent may still target Thrun's controller.
func TestThrunCantBeTargetedByOpponentsNongreenSpell(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	thrun := lookup(t, reg, "Thrun, Breaker of Silence")
	bolt := lookup(t, reg, "Lightning Bolt")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{thrun}, []*cards.Card{bolt})

	thrunID := moveByName(t, e, 0, "Thrun, Breaker of Silence", state.ZBattlefield)
	boltID := moveByName(t, e, 1, "Lightning Bolt", state.ZHand)
	cantTargetOnBoard(t, e, thrunID)
	if o := e.G.Obj(boltID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Lightning Bolt %d is not in seat 1's hand: %+v", boltID, o)
	}

	addMana(t, e, 1, "R")
	e.askPriority(1)
	if !castOffered(e, boltID) {
		t.Fatalf("precondition: Lightning Bolt is not castable: %+v", e.Pending().Options)
	}
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target decision, got %+v", d)
	}
	sawOpponent := false
	for _, o := range d.Options {
		if o.Obj == thrunID {
			t.Fatalf("Thrun was offered as an opponent's nongreen spell target: %+v", d.Options)
		}
		if o.Kind == "player" && o.Player == 0 {
			sawOpponent = true
		}
	}
	if !sawOpponent {
		t.Fatalf("the opponent's own player target was withheld alongside Thrun: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestThrunCanBeTargetedByGreenOrOwnSpells is the permissive side: Thrun's
// ValidSource$ Card.nonGreen+OppCtrl admits a green spell from the opponent
// and any spell from Thrun's own controller.
func TestThrunCanBeTargetedByGreenOrOwnSpells(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	thrun := lookup(t, reg, "Thrun, Breaker of Silence")
	growth := lookup(t, reg, "Giant Growth")
	bolt := lookup(t, reg, "Lightning Bolt")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{thrun, bolt}, []*cards.Card{growth})

	thrunID := moveByName(t, e, 0, "Thrun, Breaker of Silence", state.ZBattlefield)
	growthID := moveByName(t, e, 1, "Giant Growth", state.ZHand)
	cantTargetOnBoard(t, e, thrunID)

	addMana(t, e, 1, "G")
	e.askPriority(1)
	if !castOffered(e, growthID) {
		t.Fatalf("precondition: Giant Growth is not castable: %+v", e.Pending().Options)
	}
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Giant Growth's target decision, got %+v", d)
	}
	if indexOfObjOption(d, thrunID) < 0 {
		t.Fatalf("the opponent's green spell may not target Thrun: %+v", d.Options)
	}
	submitChoices(t, e, indexOfObjOption(d, thrunID))
	passUntilStackEmpty(t, e, 12)

	boltID := moveByName(t, e, 0, "Lightning Bolt", state.ZHand)
	addMana(t, e, 0, "R")
	e.askPriority(0)
	if !castOffered(e, boltID) {
		t.Fatalf("precondition: Thrun's controller's Lightning Bolt is not castable: %+v", e.Pending().Options)
	}
	castFirst(t, e, "cast")
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the controller's Lightning Bolt target decision, got %+v", d)
	}
	if indexOfObjOption(d, thrunID) < 0 {
		t.Fatalf("Thrun's controller may not target it with its own spell: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestThrunCantBeTargetedByOpponentsNongreenAbility pins the ability arm:
// a red activated damage ability from an opponent cannot target Thrun.
func TestThrunCantBeTargetedByOpponentsNongreenAbility(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	thrun := lookup(t, reg, "Thrun, Breaker of Silence")
	sparkmage := lookup(t, reg, "Cunning Sparkmage")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{thrun}, []*cards.Card{sparkmage})

	thrunID := moveByName(t, e, 0, "Thrun, Breaker of Silence", state.ZBattlefield)
	sparkID := moveByName(t, e, 1, "Cunning Sparkmage", state.ZBattlefield)
	cantTargetOnBoard(t, e, thrunID)
	if o := e.G.Obj(sparkID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Cunning Sparkmage %d is not on the battlefield: %+v", sparkID, o)
	}

	e.askPriority(1)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected seat 1's priority, got %+v", d)
	}
	activate := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == sparkID {
			activate = o.Index
		}
	}
	if activate < 0 {
		t.Fatalf("precondition: Cunning Sparkmage's pinger was not offered for activation: %+v", d.Options)
	}
	submitChoices(t, e, activate)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the pinger's target decision, got %+v", d)
	}
	sawOpponent := false
	for _, o := range d.Options {
		if o.Obj == thrunID {
			t.Fatalf("Thrun was offered as an opponent's nongreen ability target: %+v", d.Options)
		}
		if o.Kind == "player" && o.Player == 0 {
			sawOpponent = true
		}
	}
	if !sawOpponent {
		t.Fatalf("the pinger's player target was withheld alongside Thrun: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// autumnVeilTargetOptions casts Autumn's Veil for seat 0, then has caster cast
// spell with the given mana and returns the resulting target decision. The
// engine also carries a second, seat-1 Grizzly Bears so every spell stays
// castable when the protected creature is correctly withheld.
func autumnVeilTargetOptions(t *testing.T, reg *cards.Registry, spell *cards.Card, caster state.PlayerID, symbols string) (*Engine, Config, state.ObjID, state.ObjID, *decision.Decision) {
	t.Helper()
	veil := lookup(t, reg, "Autumn's Veil")
	bears := lookup(t, reg, "Grizzly Bears")
	extras0 := []*cards.Card{veil, bears}
	if caster == 0 {
		// The controller's-own-spell leg needs the spell in seat 0's deck.
		extras0 = append(extras0, spell)
	}
	e, cfg := corpusEngineCfg(t, reg, extras0, []*cards.Card{spell, bears})

	protected := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	alternate := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	veilID := moveByName(t, e, 0, "Autumn's Veil", state.ZHand)
	if o := e.G.Obj(protected); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the protected Grizzly Bears %d is not on the battlefield: %+v", protected, o)
	}
	if o := e.G.Obj(alternate); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the alternate Grizzly Bears %d is not on the battlefield: %+v", alternate, o)
	}

	addMana(t, e, 0, "G")
	e.askPriority(0)
	if !castOffered(e, veilID) {
		t.Fatalf("precondition: Autumn's Veil is not castable: %+v", e.Pending().Options)
	}
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 12)

	registered := false
	for ceI, ceL := 0, e.active(); ceI < len(ceL); ceI++ {
		ce := &ceL[ceI]
		if ce.Restriction == "CantTarget" && ce.RestrictParam(cards.PKValidSource) == "Card.Blue,Card.Black" {
			registered = true
		}
	}
	if !registered {
		t.Fatal("precondition: Autumn's Veil did not register its CantTarget effect")
	}

	spellID := moveByName(t, e, caster, spell.Faces[0].Name, state.ZHand)
	addMana(t, e, caster, symbols)
	e.askPriority(caster)
	if !castOffered(e, spellID) {
		t.Fatalf("%s is not castable with the alternate target present: %+v", spell.Faces[0].Name, e.Pending().Options)
	}
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected %s's target decision, got %+v", spell.Faces[0].Name, d)
	}
	return e, cfg, protected, alternate, d
}

// TestCantTargetEffectValidSourceIsRead is the over-restriction regression:
// before this change the Effect-delivered Autumn's Veil body withheld the
// creature from EVERY source, ignoring ValidSource$ and ValidSA$. Now the
// blue and black spells (by an opponent) are withheld while a red spell and
// the controller's own spell still see it.
func TestCantTargetEffectValidSourceIsRead(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	blue := lookup(t, reg, "Unsummon")
	black := lookup(t, reg, "Vendetta")
	red := lookup(t, reg, "Shock")

	e, cfg, protected, alternate, d := autumnVeilTargetOptions(t, reg, blue, 1, "U")
	if indexOfObjOption(d, protected) >= 0 {
		t.Fatalf("Autumn's Veil let an opponent's blue spell target the protected creature: %+v", d.Options)
	}
	if indexOfObjOption(d, alternate) < 0 {
		t.Fatalf("the alternate creature was withheld too (the restriction over-applies): %+v", d.Options)
	}
	replayCheck(t, e, cfg)

	e, cfg, protected, alternate, d = autumnVeilTargetOptions(t, reg, black, 1, "B")
	if indexOfObjOption(d, protected) >= 0 {
		t.Fatalf("Autumn's Veil let an opponent's black spell target the protected creature: %+v", d.Options)
	}
	if indexOfObjOption(d, alternate) < 0 {
		t.Fatalf("the alternate creature was withheld too (the restriction over-applies): %+v", d.Options)
	}
	replayCheck(t, e, cfg)

	e, cfg, protected, alternate, d = autumnVeilTargetOptions(t, reg, red, 1, "R")
	if indexOfObjOption(d, protected) < 0 {
		t.Fatalf("Autumn's Veil withheld the creature from a red spell, which ValidSource$ does not name: %+v", d.Options)
	}
	if indexOfObjOption(d, alternate) < 0 {
		t.Fatalf("the alternate creature was withheld from a red spell: %+v", d.Options)
	}
	replayCheck(t, e, cfg)

	e, cfg, protected, _, d = autumnVeilTargetOptions(t, reg, red, 0, "R")
	if indexOfObjOption(d, protected) < 0 {
		t.Fatalf("Autumn's Veil withheld the creature from its controller's own red spell: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}

// TestGroundSealGraveyardCardsCantBeTargeted pins the AffectedZone$ Graveyard
// shape on a printed static: Raise Dead has no legal target while Ground Seal
// is on the battlefield, so its cast is not offered at all.
func TestGroundSealGraveyardCardsCantBeTargeted(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	seal := lookup(t, reg, "Ground Seal")
	raise := lookup(t, reg, "Raise Dead")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{seal, raise, bears}, nil)

	sealID := moveByName(t, e, 0, "Ground Seal", state.ZBattlefield)
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	raiseID := moveByName(t, e, 0, "Raise Dead", state.ZHand)
	cantTargetOnBoard(t, e, sealID)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Grizzly Bears %d is not in the graveyard: %+v", bearID, o)
	}

	addMana(t, e, 0, "B")
	e.askPriority(0)
	if castOffered(e, raiseID) {
		t.Fatalf("Raise Dead was castable with Ground Seal on the battlefield: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)
}
