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

	// Baseline: without Ground Seal the same graveyard card is a legal target,
	// so the sealed assertion below is not passing for an unrelated reason.
	base, _ := corpusEngineCfg(t, reg, []*cards.Card{raise, bears}, nil)
	baseBear := moveByName(t, base, 0, "Grizzly Bears", state.ZGraveyard)
	baseRaise := moveByName(t, base, 0, "Raise Dead", state.ZHand)
	addMana(t, base, 0, "B")
	base.askPriority(0)
	if !castOffered(base, baseRaise) {
		t.Fatalf("precondition: Raise Dead is not castable without Ground Seal: %+v", base.Pending().Options)
	}
	castFirst(t, base, "cast")
	baseTarget := base.Pending()
	if baseTarget == nil || baseTarget.Kind != decision.KTarget || indexOfObjOption(baseTarget, baseBear) < 0 {
		t.Fatalf("precondition: the graveyard Grizzly Bears is not offered without Ground Seal: %+v", baseTarget)
	}

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

// activateObjAbility submits obj's "ability" option on seat p's priority and
// returns the target decision the activation poses. Cunning Sparkmage has
// Haste, so its {T} activation is offered the turn it enters even though
// moveByName's battlefield move marks the permanent summoning-sick.
func activateObjAbility(t *testing.T, e *Engine, p state.PlayerID, obj state.ObjID) *decision.Decision {
	t.Helper()
	e.askPriority(p)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected seat %d's priority, got %+v", p, d)
	}
	activate := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == obj {
			activate = o.Index
			break
		}
	}
	if activate < 0 {
		t.Fatalf("precondition: object %d's ability was not offered for activation: %+v", obj, d.Options)
	}
	submitChoices(t, e, activate)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision for object %d's ability, got %+v", obj, d)
	}
	return d
}

// TestCantTargetValidSASpellDoesNotBlockActivatedAbility pins ValidSA$ against
// the TARGETING ABILITY, not its host permanent: Dense Foliage's
// `ValidSA$ Spell` withholds a creature from the opponent's Lightning Bolt,
// but not from the opponent's Cunning Sparkmage activation. Before the fix the
// host permanent was classified as a Spell, so the pinger's target was
// withheld too (and the resolution recheck dropped it).
func TestCantTargetValidSASpellDoesNotBlockActivatedAbility(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	foliage := lookup(t, reg, "Dense Foliage")
	bears := lookup(t, reg, "Grizzly Bears")
	bolt := lookup(t, reg, "Lightning Bolt")

	// Spell leg: the restriction bites a spell.
	spellE, spellCfg := corpusEngineCfg(t, reg, []*cards.Card{foliage, bears}, []*cards.Card{bolt})
	spellFoliage := moveByName(t, spellE, 0, "Dense Foliage", state.ZBattlefield)
	spellBear := moveByName(t, spellE, 0, "Grizzly Bears", state.ZBattlefield)
	boltID := moveByName(t, spellE, 1, "Lightning Bolt", state.ZHand)
	cantTargetOnBoard(t, spellE, spellFoliage)
	addMana(t, spellE, 1, "R")
	spellE.askPriority(1)
	if !castOffered(spellE, boltID) {
		t.Fatalf("precondition: Lightning Bolt is not castable: %+v", spellE.Pending().Options)
	}
	castFirst(t, spellE, "cast")
	d := spellE.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target decision, got %+v", d)
	}
	if indexOfObjOption(d, spellBear) >= 0 {
		t.Fatalf("Dense Foliage's ValidSA$ Spell did not withhold the creature from a spell: %+v", d.Options)
	}
	replayCheck(t, spellE, spellCfg)

	// Ability leg: it must not bite an activated ability, and the ability's
	// own CR 608.2b recheck must let the damage land.
	abilityE, abilityCfg := corpusEngineCfg(t, reg, []*cards.Card{foliage, bears}, []*cards.Card{lookup(t, reg, "Cunning Sparkmage")})
	abilityFoliage := moveByName(t, abilityE, 0, "Dense Foliage", state.ZBattlefield)
	abilityBear := moveByName(t, abilityE, 0, "Grizzly Bears", state.ZBattlefield)
	sparkID := moveByName(t, abilityE, 1, "Cunning Sparkmage", state.ZBattlefield)
	cantTargetOnBoard(t, abilityE, abilityFoliage)
	d = activateObjAbility(t, abilityE, 1, sparkID)
	idx := indexOfObjOption(d, abilityBear)
	if idx < 0 {
		t.Fatalf("Dense Foliage's ValidSA$ Spell withheld the creature from an activated ability: %+v", d.Options)
	}
	submitChoices(t, abilityE, idx)
	passUntilStackEmpty(t, abilityE, 12)
	if o := abilityE.G.Obj(abilityBear); o == nil || o.Damage != 1 {
		t.Fatalf("the pinger's damage did not land on the creature the restriction should not have withheld: %+v", o)
	}
	replayCheck(t, abilityE, abilityCfg)
}

// TestCantTargetValidSAActivatedTriggeredReadsAbility is the reverse direction:
// Shanna's `ValidSA$ Activated,Triggered | Activator$ Player.Opponent` must
// withhold her from an opponent's activated ability, while a spell (not in
// the list) and her controller's own ability still target her. Before the fix
// the host permanent classified as a Spell, so the restriction never applied.
func TestCantTargetValidSAActivatedTriggeredReadsAbility(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	shanna := lookup(t, reg, "Shanna, Sisay's Legacy")
	spark := lookup(t, reg, "Cunning Sparkmage")
	bolt := lookup(t, reg, "Lightning Bolt")

	// Spell leg: a Spell is not in ValidSA$, so Bolt keeps Shanna.
	spellE, spellCfg := corpusEngineCfg(t, reg, []*cards.Card{shanna}, []*cards.Card{bolt})
	spellShanna := moveByName(t, spellE, 0, "Shanna, Sisay's Legacy", state.ZBattlefield)
	boltID := moveByName(t, spellE, 1, "Lightning Bolt", state.ZHand)
	cantTargetOnBoard(t, spellE, spellShanna)
	addMana(t, spellE, 1, "R")
	spellE.askPriority(1)
	if !castOffered(spellE, boltID) {
		t.Fatalf("precondition: Lightning Bolt is not castable: %+v", spellE.Pending().Options)
	}
	castFirst(t, spellE, "cast")
	d := spellE.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target decision, got %+v", d)
	}
	if indexOfObjOption(d, spellShanna) < 0 {
		t.Fatalf("Shanna's ValidSA$ Activated,Triggered withheld her from a spell: %+v", d.Options)
	}
	replayCheck(t, spellE, spellCfg)

	// Opponent's activated ability: blocked.
	oppE, oppCfg := corpusEngineCfg(t, reg, []*cards.Card{shanna}, []*cards.Card{spark})
	oppShanna := moveByName(t, oppE, 0, "Shanna, Sisay's Legacy", state.ZBattlefield)
	oppSpark := moveByName(t, oppE, 1, "Cunning Sparkmage", state.ZBattlefield)
	cantTargetOnBoard(t, oppE, oppShanna)
	d = activateObjAbility(t, oppE, 1, oppSpark)
	if indexOfObjOption(d, oppShanna) >= 0 {
		t.Fatalf("Shanna's ValidSA$ Activated,Triggered did not withhold her from an opponent's activated ability: %+v", d.Options)
	}
	replayCheck(t, oppE, oppCfg)

	// Her controller's own activated ability: Activator$ Player.Opponent does
	// not name seat 0, so Shanna is a legal target.
	ownE, ownCfg := corpusEngineCfg(t, reg, []*cards.Card{shanna, spark}, nil)
	ownShanna := moveByName(t, ownE, 0, "Shanna, Sisay's Legacy", state.ZBattlefield)
	ownSpark := moveByName(t, ownE, 0, "Cunning Sparkmage", state.ZBattlefield)
	cantTargetOnBoard(t, ownE, ownShanna)
	d = activateObjAbility(t, ownE, 0, ownSpark)
	if indexOfObjOption(d, ownShanna) < 0 {
		t.Fatalf("Shanna's controller could not target her with its own activated ability: %+v", d.Options)
	}
	replayCheck(t, ownE, ownCfg)
}

// TestCantTargetEffectValidSAIsReadForAbility covers the Effect-delivered path
// (an e.active() registration, not a printed static) with an ability target:
// the body's ValidSA$ Spell must withhold the creature from a spell but not
// from an activated ability. TestCantTargetEffectValidSourceIsRead covers the
// spell side only.
func TestCantTargetEffectValidSAIsReadForAbility(t *testing.T) {
	const bearSrc = "Name:Test Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	const pingerSrc = "Name:Test Pinger\nManaCost:R\nTypes:Creature Human Wizard\nPT:0/1\nK:Haste\n" +
		"A:AB$ DealDamage | Cost$ T | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
	body := ContinuousEffect{
		Restriction:    "CantTarget",
		RestrictParams: map[string]string{"ValidTarget": "Creature", "ValidSA": "Spell"},
		Duration:       "UntilTheEndOfYourNextTurn",
	}

	// Spell leg: Shock's only other legal target is the player, so the cast
	// stays offered and the decision shows the creature withheld.
	spellE := handEngine(t)
	spellBear := onBoard(t, spellE, 0, bearSrc)
	shockID := handCard(spellE, card(t, "Name:Shock\nManaCost:R\nTypes:Instant\n"+
		"A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2\nOracle:x\n"), 1)
	ce := body
	ce.Source, ce.Controller = spellBear, 0
	spellE.AddContinuous(ce)
	addMana(t, spellE, 1, "R")
	spellE.askPriority(1)
	if !castOffered(spellE, shockID) {
		t.Fatalf("precondition: Shock is not castable with the player target available: %+v", spellE.Pending().Options)
	}
	castFirst(t, spellE, "cast")
	d := spellE.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Shock's target decision, got %+v", d)
	}
	if indexOfObjOption(d, spellBear) >= 0 {
		t.Fatalf("the Effect body's ValidSA$ Spell did not withhold the creature from a spell: %+v", d.Options)
	}

	// Ability leg: the same body must not withhold it from the pinger.
	abilityE := handEngine(t)
	abilityBear := onBoard(t, abilityE, 0, bearSrc)
	pingerID := onBoardReady(t, abilityE, 1, pingerSrc)
	ce = body
	ce.Source, ce.Controller = abilityBear, 0
	abilityE.AddContinuous(ce)
	d = activateObjAbility(t, abilityE, 1, pingerID)
	if indexOfObjOption(d, abilityBear) < 0 {
		t.Fatalf("the Effect body's ValidSA$ Spell withheld the creature from an activated ability: %+v", d.Options)
	}
}
