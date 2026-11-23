package rules

// Temporary diagnostic probe (deleted before commit): what funds the priority
// offer of Molten Ravager's {R} activation on an empty pool with an untapped
// Sol Grail (chosen U), an untapped Signpost Scarecrow and a Simian Spirit
// Guide in hand.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func probeOffer(t *testing.T, e *Engine, rav state.ObjID) (found bool, opts string) {
	t.Helper()
	for _, o := range e.legalActions(0) {
		if o.Obj == rav && o.Kind == "activate" {
			found = true
		}
		if o.Kind != "pass" && o.Kind != "concede" {
			t.Logf("option kind=%s obj=%d ability=%d label=%q", o.Kind, o.Obj, o.Ability, o.Label)
			opts += " " + o.Kind
		}
	}
	return found, opts
}

func TestZZMoltenProbe(t *testing.T) {
	build := func(t *testing.T, guide bool) (*Engine, state.ObjID) {
		var e *Engine
		var rav state.ObjID
		if guide {
			e, _, _ = newFixtureDeck(t, 9405, "Name:R Probe\nManaCost:R\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n",
				"Name:Guide Probe\nManaCost:2 R\nTypes:Creature Ape Spirit\nPT:2/2\nA:AB$ Mana | Cost$ ExileFromHand<1/CARDNAME> | Produced$ R | ActivationZone$ Hand | SpellDescription$ Add {R}.\nOracle:x\n")
			rav = onBoard(t, e, 0, "Name:Ravager Probe\nManaCost:2 R\nTypes:Creature Elemental\nPT:0/4\nA:AB$ Pump | Cost$ R | NumAtt$ +1 | NumDef$ +0 | SpellDescription$ CARDNAME gets +1/+0 until end of turn.\nOracle:x\n")
			toHand(t, e, "Name:Guide Probe\nManaCost:2 R\nTypes:Creature Ape Spirit\nPT:2/2\nA:AB$ Mana | Cost$ ExileFromHand<1/CARDNAME> | Produced$ R | ActivationZone$ Hand | SpellDescription$ Add {R}.\nOracle:x\n")
		} else {
			e, _, _ = newFixtureDeck(t, 9405, "Name:R Probe\nManaCost:R\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n")
			rav = onBoard(t, e, 0, "Name:Ravager Probe\nManaCost:2 R\nTypes:Creature Elemental\nPT:0/4\nA:AB$ Pump | Cost$ R | NumAtt$ +1 | NumDef$ +0 | SpellDescription$ CARDNAME gets +1/+0 until end of turn.\nOracle:x\n")
		}
		grail := onBoard(t, e, 0, "Name:Grail Probe\nManaCost:3\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ Chosen | SpellDescription$ Add one mana of the chosen color.\nOracle:x\n")
		e.G.Obj(grail).ChosenColor = "U"
		if o := e.G.Obj(grail); o == nil || o.ChosenColor != "U" {
			t.Fatalf("precondition: grail chosen = %v", o.ChosenColor)
		}
		onBoard(t, e, 0, "Name:Crow Probe\nManaCost:4\nTypes:Artifact Creature Scarecrow\nPT:2/4\nA:AB$ Mana | Cost$ 2 | Produced$ Any | SpellDescription$ Add one mana of any color.\nOracle:x\n")
		if n := e.G.Players[0].Pool.Total(); n != 0 {
			t.Fatalf("precondition: pool = %d, want 0", n)
		}
		return e, rav
	}
	t.Run("no guide", func(t *testing.T) {
		e, rav := build(t, false)
		found, opts := probeOffer(t, e, rav)
		t.Logf("no guide: found=%v options=%s", found, opts)
		if found {
			t.Errorf("offered without a guide")
		}
	})
	t.Run("guide in hand", func(t *testing.T) {
		e, rav := build(t, true)
		found, opts := probeOffer(t, e, rav)
		t.Logf("guide: found=%v options=%s", found, opts)
		if !found {
			t.Errorf("not offered with the guide in hand")
		}
	})
}
