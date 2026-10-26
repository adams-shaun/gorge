package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestHarmonizeAnnouncementSelectsAtMostOneCreature pins CR 702.180a/b on the
// CR 601.2b announcement: the caster taps up to ONE creature, and the engine
// must offer that single pick rather than one independent option per creature.
func TestHarmonizeAnnouncementSelectsAtMostOneCreature(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Wild Ride"))
	weak := e.G.AddObject(card(t, "Name:Three Power A\nTypes:Creature Human\nPT:3/3\nOracle:x\n"), 0)
	strong := e.G.AddObject(card(t, "Name:Four Power B\nTypes:Creature Human\nPT:4/4\nOracle:x\n"), 0)
	weak.Zone = state.ZBattlefield
	strong.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{weak.ID, strong.ID})
	e.G.Players[0].Pool[state.MR] = 1
	spell := e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "harmonize")
	d := e.Pending()
	if d == nil || len(d.Options) != 2 {
		t.Fatalf("Harmonize did not offer both creatures: %+v", d)
	}
	// Precondition: the two offers differ in power, the stronger one (4)
	// covers Wild Ride's {4} generic while the weaker one (3) does not.
	weakAmount, strongAmount := d.Options[0].Amount, d.Options[1].Amount
	if weakAmount != 3 || strongAmount != 4 {
		t.Fatalf("Harmonize offered powers %d and %d, want 3 and 4", weakAmount, strongAmount)
	}
	generic := int32(4)
	if int32(strongAmount) < generic || int32(weakAmount) >= generic {
		t.Fatalf("test setup: powers %d/%d do not straddle the generic %d", weakAmount, strongAmount, generic)
	}
	if d.Max != 1 {
		t.Fatalf("Harmonize Max = %d, want 1 (CR 702.180a: at most one creature)", d.Max)
	}
	if d.Options[0].Group == "" || d.Options[0].Group != d.Options[1].Group {
		t.Fatalf("harmonize options are not in one shared group: %q and %q", d.Options[0].Group, d.Options[1].Group)
	}
	two := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 1}}
	if err := d.Validate(two); err == nil {
		t.Fatalf("two harmonize picks validated: %+v", two)
	}
	submitChoices(t, e, 1)
	submitChoices(t, e, 0) // target a creature
	if !e.G.Obj(strong.ID).Tapped || e.G.Obj(weak.ID).Tapped {
		t.Fatalf("tapped strong=%v weak=%v, want only the chosen 4/4 tapped",
			e.G.Obj(strong.ID).Tapped, e.G.Obj(weak.ID).Tapped)
	}
	if e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("Wild Ride after harmonize payment = %s, want stack", e.G.Obj(spell).Zone)
	}
}

// TestHarmonizeOfferGateUsesOneCreature pins the legal-action offer gate: the
// harmonize alternative is priced by the single best creature, never by the
// summed power of every creature (CR 702.180a).
func TestHarmonizeOfferGateUsesOneCreature(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	spell := e.G.AddObject(corpusAlternativeCard(t, "Wild Ride"), 0)
	spell.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{spell.ID})
	a := e.G.AddObject(card(t, "Name:Two Power A\nTypes:Creature Human\nPT:2/2\nOracle:x\n"), 0)
	b := e.G.AddObject(card(t, "Name:Two Power B\nTypes:Creature Human\nPT:2/2\nOracle:x\n"), 0)
	a.Zone = state.ZBattlefield
	b.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{a.ID, b.ID})
	e.G.Players[0].Pool[state.MR] = 1
	// Precondition: their sum (4) would pay the generic while no single one
	// (2) can, so the old greedy gate and the new single-pick gate disagree.
	sum := e.Derived(a.ID).Power + e.Derived(b.ID).Power
	best := max(e.Derived(a.ID).Power, e.Derived(b.ID).Power)
	if sum < 4 || best >= 4 {
		t.Fatalf("test setup: powers sum=%d best=%d do not straddle the generic 4", sum, best)
	}
	if n := harmonizeOptionCount(e.legalActions(0)); n != 0 {
		t.Fatalf("harmonize offered with two 2/2s (sum %d, best %d): %d option(s)", sum, best, n)
	}
	big := e.G.AddObject(card(t, "Name:Five Power C\nTypes:Creature Human\nPT:5/5\nOracle:x\n"), 0)
	big.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{a.ID, b.ID, big.ID})
	if n := harmonizeOptionCount(e.legalActions(0)); n != 1 {
		t.Fatalf("harmonize offered %d time(s) with a 5/5 present, want exactly 1", n)
	}
}

func harmonizeOptionCount(opts []decision.Option) int {
	n := 0
	for _, o := range opts {
		if o.Mode == "harmonize" {
			n++
		}
	}
	return n
}
