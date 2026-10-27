package rules

// cr-g213: a targeted ChangeZone with an explicit single Origin$ takes its
// target census from that Origin$ when no TgtZone$ is declared (Forge
// AbilityFactory.adjustChangeZoneTarget: every listed Origin$ zone is the
// target zone; XMage Sentinel of Lost Lore uses TargetCardInExile). Exile was
// missing from the inferred set, so the battlefield was searched instead and
// Sentinel of Lost Lore's "return target card you own in exile that has an
// Adventure" mode (and Blade of the Swarm's warp mode) had no legal target.
// These tests use the fixture spell + a minimal authored Adventure card; the
// Sentinel case uses the real corpus card.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// originExileSpellScript is the brief's shape: a targeted ChangeZone whose
// only zone declaration is Origin$ Exile.
const originExileSpellScript = "Name:Exile Fetch\nManaCost:G\nTypes:Sorcery\n" +
	"A:SP$ ChangeZone | ValidTgts$ Card.YouOwn | Origin$ Exile | Destination$ Hand | SpellDescription$ Return target card you own in exile to your hand.\nOracle:x\n"

// originExileAdventureFixture is an authored Adventure card (never Forge
// text): the front is a creature, the alternate face is an Instant Adventure,
// which is what Card.AdventureCard matches.
const originExileAdventureFixture = "Name:Bonecrusher Giant\nManaCost:2 R\nTypes:Creature Giant\nPT:4/3\n" +
	"AlternateMode:Adventure\nOracle:front\nALTERNATE\nName:Stomp\nManaCost:1 R\n" +
	"Types:Instant Adventure\nOracle:back\n"

// TestOriginExileTargetsExiledCard pins the defect's plain-spell shape end to
// end: with Bonecrusher Giant in exile the spell is offered, its target ask
// offers exactly that exiled card, and resolution returns it to hand.
func TestOriginExileTargetsExiledCard(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 205, originExileSpellScript, originExileAdventureFixture)
	giant := moveSeeded(t, e, 0, originExileAdventureFixture, state.ZExile)
	if o := e.G.Obj(giant); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: Bonecrusher zone = %+v, want exile", o)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: fixture spell zone = %+v, want hand", o)
	}
	addMana(t, e, 0, "G")
	if !castOffered(e, id) {
		t.Fatalf("Origin$ Exile spell not castable with a card in exile: %+v", e.Pending().Options)
	}

	submitChoices(t, e, castOptionFor(t, e, id).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the ChangeZone target ask", d)
	}
	giantOpt := -1
	for _, o := range d.Options {
		if o.Obj == giant {
			giantOpt = o.Index
		}
	}
	if giantOpt < 0 {
		t.Fatalf("target ask did not offer the exiled Bonecrusher: %+v", d.Options)
	}
	submitChoices(t, e, giantOpt)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(giant); o == nil || o.Zone != state.ZHand {
		t.Fatalf("Bonecrusher zone = %+v, want hand after resolution", o)
	}
	replayCheck(t, e, cfg)
}

// TestSentinelOfLostLoreReturnsExiledAdventure uses the real corpus card:
// the "return target card you own in exile that has an Adventure" mode must
// be offered, offer the exiled Adventure card as its target, return it to
// hand, and NOT treat an exiled non-Adventure card as a legal target.
func TestSentinelOfLostLoreReturnsExiledAdventure(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t,
		"s/sentinel_of_lost_lore.txt", "b/bonecrusher_giant_stomp.txt",
		"g/grizzly_bears.txt", "f/forest.txt")
	e, cfg := etbreplEngine(t, reg, "Sentinel of Lost Lore", "Bonecrusher Giant", "Grizzly Bears")
	giant := searchMoveByName(t, e, "Bonecrusher Giant", state.ZExile)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZExile)
	for _, id := range []state.ObjID{giant, bear} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZExile {
			t.Fatalf("precondition: card %d zone = %+v, want exile", id, o)
		}
	}
	sentinel := searchMoveByName(t, e, "Sentinel of Lost Lore", state.ZHand)
	addMana(t, e, 0, "1GG")
	submitChoices(t, e, castOptionFor(t, e, sentinel).Index)

	d := passUntilNonPriority(t, e, 30)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want Sentinel's Charm mode ask", d)
	}
	mode := -1
	for _, o := range d.Options {
		if strings.Contains(o.Label, "Return target card you own in exile") {
			mode = o.Index
		}
	}
	if mode < 0 {
		t.Fatalf("exile-Adventure mode not offered: %+v", d.Options)
	}
	submitChoices(t, e, mode)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the chosen mode's target ask", d)
	}
	giantOpt, bearOffered := -1, false
	for _, o := range d.Options {
		if o.Obj == giant {
			giantOpt = o.Index
		}
		if o.Obj == bear {
			bearOffered = true
		}
	}
	if giantOpt < 0 {
		t.Fatalf("exile-Adventure mode did not offer the exiled Bonecrusher: %+v", d.Options)
	}
	if bearOffered {
		t.Fatalf("exile-Adventure mode offered an exiled card without an Adventure: %+v", d.Options)
	}
	submitChoices(t, e, giantOpt)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(giant); o == nil || o.Zone != state.ZHand {
		t.Fatalf("Bonecrusher zone = %+v, want hand after the mode resolved", o)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("non-Adventure card zone = %+v, want it left in exile", o)
	}
	replayCheck(t, e, cfg)
}

// TestOriginExileIgnoresBattlefield pins the other half: with nothing in
// exile and a creature on the battlefield, the Origin$ Exile spell has no
// legal target and must not be offered (it used to search the battlefield).
func TestOriginExileIgnoresBattlefield(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 205, originExileSpellScript, vanillaCreatureScript)
	bear := putCreature(t, e, 0, vanillaCreatureScript)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: creature zone = %+v, want battlefield", o)
	}
	for p := state.PlayerID(0); p < 2; p++ {
		if zs := e.G.Zone(state.ZExile, p); len(zs) != 0 {
			t.Fatalf("precondition: seat %d exile = %v, want empty", p, zs)
		}
	}
	addMana(t, e, 0, "G")
	if castOffered(e, id) {
		t.Fatalf("Origin$ Exile spell offered from the battlefield: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)
}
