package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The X ask's potential-target retry (xTargetPotentialMods) prices an
// announced {X} under a target-dependent reduction the nil-target composition
// pc.mods lacks, but it accepted only a payment the floating pool finishes.
// A CR 601.2g window source (Lotus Blossom's "{T}, Sacrifice this: Add X mana
// of any one color, where X is its petal counters" -- a resolvable counter
// amount behind a deterministic self-sacrifice, already priced by
// castWindowUnits) funds the rest of the cost, so its X values were withheld:
// with {R}{B}{B} floating, two petals and a blue permanent out, Torch Breath
// ({X}{R}, {2} less if it targets a blue permanent) pays {4}{R} at X=6, but
// the menu offered only 0..4 (CR 601.2b, 601.2f, 605.3b).
const (
	cwLotusSrc = "Name:Lotus\nManaCost:2\nTypes:Artifact\n" +
		"A:AB$ Mana | Produced$ Any | Amount$ Count$CardCounters.PETAL | Cost$ T Sac<1/CARDNAME>\n" +
		"Oracle:x\n"
	cwTorchSrc = "Name:Torch\nManaCost:X R\nTypes:Instant\n" +
		"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | ValidTarget$ Permanent.Blue | Amount$ 2 | EffectZone$ All\n" +
		"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ X\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	cwBlueSrc = "Name:Blue\nManaCost:1 U\nTypes:Creature Merfolk\nPT:1/1\nOracle:x\n"
)

func TestCastWindowSelfSacCounterAmountWidensXAsk(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 71, cwTorchSrc, cwLotusSrc, cwBlueSrc)
	lotus := putCreature(t, e, 0, cwLotusSrc)
	blue := putCreature(t, e, 0, cwBlueSrc)
	e.emit(events.Event{Kind: events.CounterChange, Obj: lotus, Counter: "PETAL", Amount: 2})
	// Preconditions: the reduction really is target-dependent (the nil-target
	// composition carries none), the producer really holds two counters of the
	// kind its Amount$ reads, and the pool really is short of {4}{R}.
	if mods := e.costModifiers(0, handCardOf(t, e, 0, "Torch"), spellScope("")); modsReduces(mods) != 0 {
		t.Fatalf("nil-target reduction = %d, want 0 (the {2} is target-dependent)", modsReduces(mods))
	}
	if o := e.G.Obj(lotus); o == nil || o.Zone != state.ZBattlefield || len(o.Counters) != 1 || o.Counters[0].N != 2 {
		t.Fatalf("Lotus fixture %+v, want an untapped battlefield source with 2 PETAL counters", o)
	}
	if o := e.G.Obj(blue); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("blue fixture %+v, want it on the battlefield", o)
	}
	addMana(t, e, 0, "RBB")
	opt := castByName(t, e, 0, "Torch")
	if opt == nil {
		t.Fatal("Torch not castable: the {2} blue-target reduction must make {4}{R} payable from {R}{B}{B} plus the Blossom")
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("want the X decision, got %+v", d)
	}
	last := d.Options[len(d.Options)-1].Amount
	if last != 6 {
		t.Fatalf("X menu tops at %d, want 6 (pool {R}{B}{B} plus the Blossom's two any-colour mana pays {4}{R})", last)
	}
	for _, o := range d.Options {
		if o.Amount == 5 {
			return
		}
	}
	t.Fatal("X=5 missing from the menu although the window pays {3}{R}")
}

func handCardOf(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%s not in seat %d's hand", name, p)
	return 0
}

func modsReduces(m costMods) int32 {
	n := int32(0)
	for _, r := range m.Reduces {
		n += r.Generic + r.Colored.Total()
	}
	return n
}
