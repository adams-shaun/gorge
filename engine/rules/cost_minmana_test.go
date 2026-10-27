package rules

// MinMana$ (Forge's "can't reduce the mana in that cost to less than N")
// caps ONE reduction. It is not a minimum price: a cost already at or below
// the floor is left exactly as it was, so Heartstone and Forensic Gadgeteer
// never turn a {T}-only activation into {1}{T} (Forge
// CostAdjustment.java:500-509 caps maxReduction at cmc-minMana and returns 0,
// never a negative reduction). The fixtures below embed the real corpus
// scripts' parameter shape; no committed .txt.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const tapOnlyPumpSrc = "Name:Tap Pumper\nManaCost:1 G\nTypes:Creature Beast\nPT:1/1\nK:Haste\n" +
	"A:AB$ Pump | Cost$ T | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ CARDNAME gets +1/+0 until end of turn.\n" +
	"Oracle:{T}: CARDNAME gets +1/+0 until end of turn.\n"

const greenOnlyPumpSrc = "Name:Green Pumper\nManaCost:1 G\nTypes:Creature Beast\nPT:1/1\n" +
	"A:AB$ Pump | Cost$ G | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ CARDNAME gets +1/+0 until end of turn.\n" +
	"Oracle:{G}: CARDNAME gets +1/+0 until end of turn.\n"

const threeCostPumpSrc = "Name:Three Costs\nManaCost:3 G\nTypes:Creature Beast\nPT:3/3\n" +
	"A:AB$ Pump | Cost$ 3 | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ CARDNAME gets +1/+0 until end of turn.\n" +
	"A:AB$ Pump | Cost$ 2 | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ CARDNAME gets +1/+0 until end of turn.\n" +
	"A:AB$ Pump | Cost$ 1 | Defined$ Self | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ CARDNAME gets +1/+0 until end of turn.\n" +
	"Oracle:{3}, {2}, or {1}: CARDNAME gets +1/+0 until end of turn.\n"

const clueArtifactSrc = "Name:Test Clue\nManaCost:2\nTypes:Artifact\n" +
	"A:AB$ Draw | Cost$ 2 Sac<1/CARDNAME> | SpellDescription$ Draw a card.\n" +
	"Oracle:{2}, Sacrifice CARDNAME: Draw a card.\n"

const tapArtifactSrc = "Name:Test Tapper\nManaCost:2\nTypes:Artifact\n" +
	"A:AB$ Draw | Cost$ T | SpellDescription$ Draw a card.\n" +
	"Oracle:{T}: Draw a card.\n"

// TestMinManaFloorNeverRaisesTapOnlyCost is the regression: a {T}-only cost
// has zero mana BEFORE Heartstone's reduction, so the floor has nothing to
// cap and the price stays {T} — and, with an empty pool, the ability is still
// offered (it was already payable without mana).
func TestMinManaFloorNeverRaisesTapOnlyCost(t *testing.T) {
	t.Parallel()
	e, _, pumper := newFixtureDeck(t, 6201, tapOnlyPumpSrc, heartstoneSrc)
	putCreature(t, e, 0, tapOnlyPumpSrc)
	putCreature(t, e, 0, heartstoneSrc)
	if got := e.AbilityCosts(0, pumper); len(got) != 1 || got[0] != "T" {
		t.Fatalf("Heartstone's MinMana$ 1 raised a {T}-only ability to %v, want [T]", got)
	}
	e.priorityRound()
	if total := e.G.Players[0].Pool.Total(); total != 0 {
		t.Fatalf("precondition: the pool is not empty (%d mana)", total)
	}
	if got := kinds(e.legalActions(0))["ability"]; got != 1 {
		t.Fatalf("a {T}-only ability with an empty pool offered %d abilities, want 1", got)
	}
}

// TestMinManaFloorLeavesColouredOnlyCost keeps the colour arm: {G} has one
// mana and no generic for the reduction to take, so Heartstone leaves it at
// {G} (the floor is not a reason to add a generic).
func TestMinManaFloorLeavesColouredOnlyCost(t *testing.T) {
	t.Parallel()
	e, _, pumper := newFixtureDeck(t, 6202, greenOnlyPumpSrc, heartstoneSrc)
	putCreature(t, e, 0, greenOnlyPumpSrc)
	if got := e.AbilityCosts(0, pumper); len(got) != 1 || got[0] != "G" {
		t.Fatalf("precondition: the printed ability cost is %v, want [G]", got)
	}
	putCreature(t, e, 0, heartstoneSrc)
	if got := e.AbilityCosts(0, pumper); len(got) != 1 || got[0] != "G" {
		t.Fatalf("Heartstone with MinMana$ 1 changed a {G}-only ability to %v, want [G]", got)
	}
}

// TestMinManaFloorCapsBiomancer pins the cap direction: Biomancer's Familiar
// takes {2} off, but MinMana$ 1 stops the total at one mana — {3} becomes
// {1}, {2} becomes {1}, and {1} is already at the floor so it stays {1}.
func TestMinManaFloorCapsBiomancer(t *testing.T) {
	t.Parallel()
	e, _, pumper := newFixtureDeck(t, 6203, threeCostPumpSrc, biomancersFamiliarSrc)
	putCreature(t, e, 0, threeCostPumpSrc)
	if got := e.AbilityCosts(0, pumper); len(got) != 3 || got[0] != "3" || got[1] != "2" || got[2] != "1" {
		t.Fatalf("precondition: unmodified ability costs are %v, want [3 2 1]", got)
	}
	putCreature(t, e, 0, biomancersFamiliarSrc)
	if got := e.AbilityCosts(0, pumper); len(got) != 3 || got[0] != "1" || got[1] != "1" || got[2] != "1" {
		t.Fatalf("Biomancer's Familiar's {2} reduction with MinMana$ 1 produced %v, want [1 1 1]", got)
	}
}

// TestForensicGadgeteerClue is the reported card: activated abilities of the
// controller's artifacts cost {1} less but never below one mana. A Clue's
// {2}, Sacrifice cost becomes {1} plus the sacrifice; a {T}-only artifact
// ability stays {T} alone.
func TestForensicGadgeteerClue(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	gadgeteer := lookup(t, reg, "Forensic Gadgeteer")
	e := corpusEngine(t, reg, []*cards.Card{gadgeteer, card(t, clueArtifactSrc), card(t, tapArtifactSrc)}, nil)
	clue := moveByName(t, e, 0, "Test Clue", state.ZBattlefield)
	tapper := moveByName(t, e, 0, "Test Tapper", state.ZBattlefield)
	if o := e.G.Obj(clue); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Test Clue %d is not on the battlefield", clue)
	}
	if o := e.G.Obj(tapper); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Test Tapper %d is not on the battlefield", tapper)
	}
	if got := e.AbilityCosts(0, clue); len(got) != 1 || got[0] != "2 Sac<1/CARDNAME>" {
		t.Fatalf("precondition: the unprotected Clue costs %v, want [2 Sac<1/CARDNAME>]", got)
	}
	if got := e.AbilityCosts(0, tapper); len(got) != 1 || got[0] != "T" {
		t.Fatalf("precondition: the unprotected tap ability costs %v, want [T]", got)
	}

	gadgeteerID := moveByName(t, e, 0, "Forensic Gadgeteer", state.ZBattlefield)
	if o := e.G.Obj(gadgeteerID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Forensic Gadgeteer %d is not on the battlefield", gadgeteerID)
	}
	if got := e.AbilityCosts(0, clue); len(got) != 1 || got[0] != "1 Sac<1/CARDNAME>" {
		t.Fatalf("Forensic Gadgeteer's reduction with MinMana$ 1 priced the Clue at %v, want [1 Sac<1/CARDNAME>]", got)
	}
	if got := e.AbilityCosts(0, tapper); len(got) != 1 || got[0] != "T" {
		t.Fatalf("Forensic Gadgeteer's MinMana$ 1 raised a {T}-only ability to %v, want [T]", got)
	}
}
