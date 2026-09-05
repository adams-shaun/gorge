package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// abilityOption finds the "ability" option for (obj, index) in the pending
// priority decision, fatal if it is absent.
func abilityOption(t *testing.T, e *Engine, id state.ObjID, idx int) decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatalf("no decision pending while searching for ability %d of %d", idx, id)
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id && o.Ability == idx {
			return o
		}
	}
	t.Fatalf("no ability option for %d [%d]: %+v", id, idx, d.Options)
	return decision.Option{}
}

// findAbilityOption is abilityOption's non-fatal form: the "ability" option
// for (obj, index), or ok=false when the pending decision does not offer it.
func findAbilityOption(e *Engine, id state.ObjID, idx int) (decision.Option, bool) {
	d := e.Pending()
	if d == nil {
		return decision.Option{}, false
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id && o.Ability == idx {
			return o, true
		}
	}
	return decision.Option{}, false
}

// hasActivateOption reports whether the pending priority decision offers the
// existing "activate" (tap-for-mana) option for obj.
func hasActivateOption(e *Engine, obj state.ObjID) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == obj {
			return true
		}
	}
	return false
}

// indexOfPlayerOption returns the option index (not position) that targets
// the given player, or -1 if the pending decision offers no such option --
// the KTarget shape askTarget produces for a player-targeting subject.
func indexOfPlayerOption(d *decision.Decision, p state.PlayerID) int {
	for _, o := range d.Options {
		if o.Player == p {
			return o.Index
		}
	}
	return -1
}

// TestManaCostAbilityGoesOnTheStackAndResolves: an activated ability whose
// cost is mana-only is offered as an "ability" option, pays into the pool,
// pushes a real ability stack object, and its effect (a Draw) resolves.
func TestManaCostAbilityGoesOnTheStackAndResolves(t *testing.T) {
	src := "Name:Sailor\nManaCost:U\nTypes:Creature Spirit\nPT:1/1\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 31, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "UUUU")
	e.Advance()
	opt := abilityOption(t, e, id, 0) // helper: the "ability" option for (obj, index), fatal if absent
	if opt.Label != "Sailor: Draw a card." {
		t.Fatalf("label %q", opt.Label)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Ability == nil || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("stack %v pool %d", e.G.Stack, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Zone(state.ZHand, 0)) != hand+1 {
		t.Fatal("the ability did not draw")
	}
	replayCheck(t, e, cfg)
}

// TestRemoveCounterCostAndTargetedAbility: a SubCounter cost is required
// before the ability is offered, removes the counter at activation, and the
// ability targets and deals damage.
func TestRemoveCounterCostAndTargetedAbility(t *testing.T) {
	src := "Name:Ballista\nManaCost:X X\nTypes:Artifact Creature Construct\nPT:0/0\n" +
		"A:AB$ PutCounter | Cost$ 4 | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ Put a +1/+1 counter on CARDNAME.\n" +
		"A:AB$ DealDamage | Cost$ SubCounter<1/P1P1> | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ It deals 1 damage to any target.\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 32, src)
	id := putCreature(t, e, 0, src) // drives to Main1 and leaves e.pending nil, so the Advance below re-asks a fresh priority decision
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	e.Advance()
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatal("{4} ability offered with an empty pool")
	}
	opt := abilityOption(t, e, id, 1)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target decision: %+v", d)
	}
	life := e.G.Players[1].Life
	submitChoices(t, e, indexOfPlayerOption(d, 1))
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(id).Counter("P1P1") != 1 || e.G.Players[1].Life != life-1 {
		t.Fatalf("counters %d life %d", e.G.Obj(id).Counter("P1P1"), e.G.Players[1].Life)
	}
	replayCheck(t, e, cfg)
}

// TestGraveyardActivationWithSacrificeCost: an ActivationZone$ Graveyard
// ability whose cost sacrifices a land is activatable from the graveyard,
// asks the sacrifice as a cost, and returns the card to hand.
func TestGraveyardActivationWithSacrificeCost(t *testing.T) {
	src := "Name:Breaker\nManaCost:6 G\nTypes:Creature Eldrazi\nPT:5/7\nK:Devoid\n" +
		"A:AB$ ChangeZone | Cost$ 2 C Sac<1/Land> | Origin$ Graveyard | Destination$ Hand | ActivationZone$ Graveyard | SpellDescription$ Return CARDNAME from your graveyard to your hand.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 33, src)
	// The sacrificial land is a real deck Mountain moved with a logged
	// MoveZone, not a putLands fixture: replayCheck reconstructs a deck card
	// from the config, but putLands bypasses events (direct AddObject), so
	// such an object has no replay trace and would shift every ID after it.
	land := moveSeeded(t, e, 0, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n", state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	addMana(t, e, 0, "CGG")
	e.Advance()
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != land {
		t.Fatalf("sacrifice choice %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(land).Zone != state.ZGraveyard || e.G.Obj(id).Zone != state.ZHand || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("land %s breaker %s pool %d", e.G.Obj(land).Zone, e.G.Obj(id).Zone, e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

// TestSorcerySpeedAndSummoningSicknessGates: a SorcerySpeed$ True ability is
// only offered free on a sorcery-speed window (empty stack), and a creature's
// {T} cost needs no summoning sickness unless the creature has Haste (CR
// 302.6) -- here it does not, so a summoning-sick Tapper offers nothing.
func TestSorcerySpeedAndSummoningSicknessGates(t *testing.T) {
	src := "Name:Gear\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ GainLife | Cost$ 1 | Defined$ You | LifeAmount$ 1 | SorcerySpeed$ True | SpellDescription$ x\nOracle:x\n"
	boltSrc := "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 34, src, boltSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "G")
	e.Advance()
	abilityOption(t, e, id, 0) // seat 0's own main phase, empty stack: offered
	bolt := addToHand(t, e, 0, boltSrc)
	addMana(t, e, 0, "R")
	// Cast Bolt but do NOT drain the stack (castObj would): the check below
	// needs a spell still on the stack, where a sorcery-speed ability must
	// not be offered even to the active player who owns both.
	idx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == bolt {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Bolt: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget && len(d.Options) > 0 {
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("Bolt did not reach the stack")
	}
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatal("sorcery-speed ability offered with a spell on the stack")
	}
	tapper := "Name:Tapper\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nA:AB$ GainLife | Cost$ T | Defined$ You | LifeAmount$ 1 | SpellDescription$ x\nOracle:x\n"
	e2, _, elf := newFixtureDeck(t, 35, tapper)
	e2.emit(events.Event{Kind: events.MoveZone, Obj: elf, From: state.ZHand, To: state.ZBattlefield})
	e2.Advance()
	if _, ok := findAbilityOption(e2, elf, 0); ok {
		t.Fatal("a summoning-sick creature's {T} ability was offered")
	}
}

// TestRealCorpusSacCostWithAlternationAndDescriptionPays drives a REAL
// corpus activated-ability line end to end -- the return-to-hand template
// shared by the graveyard-recursion creatures -- exercising exactly the
// token shapes Task 20's ParseCost fix (Ruling FL-54) targets: the trailing
// "/artifact or creature" description inside the Sac<...> group (which a
// naive whitespace split would tear apart before MatchesSpec could see the
// spec) and the ";" OR alternation folded into MatchesSpec's "," (so a
// Bear, being a Creature, satisfies Sac<1/Artifact;Creature>). A Sac cost
// that cannot be paid never yields an "ability" option at all; once offered,
// the sacrifice is asked as a cost and the card still returns to hand.
func TestRealCorpusSacCostWithAlternationAndDescriptionPays(t *testing.T) {
	src := "Name:Trawler\nManaCost:1 U\nTypes:Creature Whale\nPT:3/3\n" +
		"A:AB$ ChangeZone | Cost$ 2 Sac<1/Artifact;Creature/artifact or creature> | Origin$ Graveyard | Destination$ Hand | ActivationZone$ Graveyard | SpellDescription$ Return CARDNAME from your graveyard to your hand.\nOracle:x\n"
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 37, src, bearSrc)
	bear := putCreature(t, e, 0, bearSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	addMana(t, e, 0, "GG")
	e.Advance()
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != bear {
		t.Fatalf("sacrifice choice %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(bear).Zone != state.ZGraveyard || e.G.Obj(id).Zone != state.ZHand || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("bear %s trawler %s pool %d", e.G.Obj(bear).Zone, e.G.Obj(id).Zone, e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

// TestCantBeActivatedValidSASparesManaAbilities: a CantBeActivated static
// with ValidCard$ Card.NamedCard and ValidSA$ Activated.!ManaAbility blocks a
// named card's activated (non-mana) abilities but expressly spares its mana
// abilities; once the source is renamed away the ability is activatable again.
func TestCantBeActivatedValidSASparesManaAbilities(t *testing.T) {
	needle := "Name:Needle\nManaCost:1\nTypes:Artifact\nS:Mode$ CantBeActivated | ValidCard$ Card.NamedCard | ValidSA$ Activated.!ManaAbility | Description$ x\nOracle:x\n"
	ballista := "Name:Ballista\nManaCost:X X\nTypes:Artifact Creature Construct\nPT:0/0\nA:AB$ DealDamage | Cost$ SubCounter<1/P1P1> | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n"
	e, _, n := newFixtureDeck(t, 36, needle, ballista)
	e.emit(events.Event{Kind: events.MoveZone, Obj: n, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Choose, Obj: n, Counter: "name", Text: "Ballista"})
	b := putCreature(t, e, 0, ballista)
	e.emit(events.Event{Kind: events.CounterChange, Obj: b, Counter: "P1P1", Amount: 1})
	land := putLands(t, e, 0, 1)[0]
	e.emit(events.Event{Kind: events.Choose, Obj: n, Counter: "name", Text: "Mountain"})
	e.Advance()
	if _, ok := findAbilityOption(e, b, 0); !ok {
		t.Fatal("Ballista (not the named card any more) should be activatable")
	}
	if !hasActivateOption(e, land) {
		t.Fatal("the named land's mana ability must still be offered: ValidSA spares mana abilities")
	}
}
