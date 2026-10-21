package rules

// Blight<N> (CR 701.68a) is a cost: "put N -1/-1 counters on a creature you
// control" as a payment. Doubling Season's EffectOnly$ AddCounter replacement
// ("If an EFFECT would put one or more counters ...") must leave it alone,
// exactly like the [+N] loyalty cost (cr_g72_cost_counter_test.go), on all
// three payment sites: an activation, a spell cast and a triggered-cost window.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// blightCreature is the 9/9 whose toughness never matters to the assertions.
const blightCreatureSrc = "Name:Blight Victim\nTypes:Creature Bear\nPT:9/9\nOracle:x\n"

func TestDoublingSeasonLeavesBlightActivationCostUndoubled(t *testing.T) {
	t.Parallel()
	idol := card(t, "Name:Blight Idol\nTypes:Artifact\n"+
		"A:AB$ GainLife | Cost$ Blight<2> | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n")
	victim := card(t, blightCreatureSrc)
	ds := tokenReplCorpusCard(t, "Doubling Season")
	e, cfg := tokenReplGame(t, 301, idol, victim, ds)
	id := moveSeededCard(t, e, 0, idol, state.ZBattlefield)
	cr := moveSeededCard(t, e, 0, victim, state.ZBattlefield)
	dsID := moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	if o := e.G.Obj(dsID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Doubling Season is not on the battlefield: %v", o)
	}
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, id, 0).Index)
	if n := len(e.G.Stack); n != 1 {
		t.Fatalf("precondition: stack depth %d after activation, want 1 (the ability is on the stack while its cost is paid)", n)
	}
	if got := e.G.Obj(cr).Counter("M1M1"); got != 2 {
		t.Fatalf("Doubling Season doubled a Blight<2> activation cost: %d M1M1 counters, want 2", got)
	}
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}

func TestDoublingSeasonLeavesBlightSpellCostUndoubled(t *testing.T) {
	t.Parallel()
	spell := card(t, "Name:Blight Rite\nTypes:Sorcery\nManaCost:R\n"+
		"A:SP$ GainLife | Cost$ Blight<2> | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n")
	victim := card(t, blightCreatureSrc)
	ds := tokenReplCorpusCard(t, "Doubling Season")
	e, cfg := tokenReplGame(t, 302, spell, victim, ds)
	sp := moveSeededCard(t, e, 0, spell, state.ZHand)
	cr := moveSeededCard(t, e, 0, victim, state.ZBattlefield)
	dsID := moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	if o := e.G.Obj(dsID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Doubling Season is not on the battlefield: %v", o)
	}
	addMana(t, e, 0, "R")
	submitChoices(t, e, castOptionFor(t, e, sp).Index)
	if n := len(e.G.Stack); n != 1 {
		t.Fatalf("precondition: stack depth %d after the cast, want 1 (the spell is on the stack while its cost is paid)", n)
	}
	if got := e.G.Obj(cr).Counter("M1M1"); got != 2 {
		t.Fatalf("Doubling Season doubled a Blight<2> spell cost: %d M1M1 counters, want 2", got)
	}
	if got := countCounterChanges(e, cr, "M1M1", 2); got != 1 {
		t.Fatalf("got %d CounterChange(M1M1,+2) events, want exactly 1", got)
	}
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}

func TestDoublingSeasonLeavesTriggeredBlightCostUndoubled(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Blighted Blackthorn", "Grizzly Bears", "Doubling Season")
	toBattlefield := func(name string) state.ObjID {
		for _, zone := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(zone, 0) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: zone, To: state.ZBattlefield})
					return id
				}
			}
		}
		t.Fatalf("corpus fixture %q absent from hand/library", name)
		return 0
	}
	toBattlefield("Blighted Blackthorn")
	creature := toBattlefield("Grizzly Bears")
	dsID := toBattlefield("Doubling Season")
	if o := e.G.Obj(dsID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Doubling Season is not on the battlefield: %v", o)
	}
	e.pending = nil
	e.priorityRound()
	d := passUntilNonPriority(t, e, 20)
	if d.Kind == decision.KTriggerOptional {
		submitChoices(t, e, 0)
	}
	payIndex, _ := triggerCostWindowAsk(t, e)
	if payIndex < 0 {
		t.Fatalf("Blight<2> was not offered as payable: %+v", e.Pending())
	}
	submitChoices(t, e, payIndex)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_blight" {
		t.Fatalf("expected triggered blight creature choice, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == creature {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("candidate creature %d not offered: %+v", creature, d.Options)
	}
	submitChoices(t, e, pick)
	// The 2/2 bear dies to its own counters, so count the placements.
	if got := countCounterChanges(e, creature, "M1M1", 4); got != 0 {
		t.Fatalf("Doubling Season doubled a triggered Blight<2> cost: %d CounterChange(M1M1,+4) events, want 0", got)
	}
	if got := countCounterChanges(e, creature, "M1M1", 2); got != 1 {
		t.Fatalf("got %d CounterChange(M1M1,+2) events, want exactly 1", got)
	}
}

// TestBlightCostStillModifiedByNonEffectOnlyReplacement is the control: a
// plain "that many plus one" M1M1 replacement (no EffectOnly$) still applies
// to a Blight cost, so the cost bracket refuses only EffectOnly$ True.
func TestBlightCostStillModifiedByNonEffectOnlyReplacement(t *testing.T) {
	t.Parallel()
	idol := card(t, "Name:Blight Idol\nTypes:Artifact\n"+
		"A:AB$ GainLife | Cost$ Blight<2> | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n")
	victim := card(t, blightCreatureSrc)
	repl := card(t, "Name:Minus Scales\nTypes:Enchantment\n"+
		"R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Permanent.YouCtrl+inZoneBattlefield | ValidCounterType$ M1M1 | ReplaceWith$ AddOneMore | Description$ If one or more -1/-1 counters would be put on a permanent you control, that many plus one are put on it instead.\n"+
		"SVar:AddOneMore:DB$ ReplaceCounter | ValidCounterType$ M1M1 | ChooseCounter$ True | Amount$ X\n"+
		"SVar:X:ReplaceCount$CounterNum/Plus.1\n"+
		"Oracle:x\n")
	e, cfg := tokenReplGame(t, 303, idol, victim, repl)
	id := moveSeededCard(t, e, 0, idol, state.ZBattlefield)
	cr := moveSeededCard(t, e, 0, victim, state.ZBattlefield)
	moveSeededCard(t, e, 0, repl, state.ZBattlefield)
	// Precondition: the replacement is live for an ordinary placement.
	e.emit(events.Event{Kind: events.CounterChange, Obj: cr, Counter: "M1M1", Amount: 1})
	if got := e.G.Obj(cr).Counter("M1M1"); got != 2 {
		t.Fatalf("precondition: authored replacement turned 1 M1M1 into %d, want 2", got)
	}
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, id, 0).Index)
	if got := e.G.Obj(cr).Counter("M1M1"); got != 5 {
		t.Fatalf("Blight<2> under a plus-one replacement: %d M1M1 counters, want 5 (2 + 3)", got)
	}
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}
