package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// The real Vivi Ornitier (printed power 0) on the boards the probe boards do
// not reach: beside a granted mana ability while a may-pay window is open, and
// against an opponent's taxes (unless costs, ward, attack tax), where neither
// engine may count Vivi's {0} ability toward the offer to pay.

// shockProbeScript is Vivi's controller's targeted spell, so a ward cost asks
// them to pay.
const shockProbeScript = "Name:Shock Probe\nManaCost:R\nTypes:Instant\n" +
	"A:SP$ DealDamage | NumDmg$ 1 | ValidTgts$ Creature.OppCtrl | SpellDescription$ CARDNAME deals 1 damage to target creature an opponent controls.\n" +
	"Oracle:Shock Probe deals 1 damage to target creature an opponent controls.\n"

// spikeProbeScript is the opponent's counter-unless instant.
func spikeProbeScript(tax string) string {
	return "Name:Spike Probe\nManaCost:U\nTypes:Instant\n" +
		"A:SP$ Counter | ValidTgts$ Card | TargetType$ Spell | UnlessCost$ " + tax + " | SpellDescription$ Counter target spell unless its controller pays {" + tax + "}.\n" +
		"Oracle:Counter target spell unless its controller pays {" + tax + "}.\n"
}

// viviTaxActivated reports whether seat 0 activated a mana ability of a card
// named name.
func viviTaxActivated(g *GorgeSide, name string) bool {
	if g == nil || g.E == nil {
		return false
	}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.ManaActivate || e.Player != 0 {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// defenderTapped reports whether a card named name tapped: for a defender,
// only its mana ability taps it.
func defenderTapped(g *GorgeSide, name string) bool {
	if g == nil || g.E == nil {
		return false
	}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.Tap {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// viviTaxBoards are the boards. Each diverged (or panicked) on some seed
// before the fix it covers.
func viviTaxBoards() []manaShapeBoard {
	vivi := func(g *GorgeSide) bool { return viviTaxActivated(g, "Vivi Ornitier") }
	grant := func(name, giver string) manaShapeBoard {
		return manaShapeBoard{name: name, first: "Vivi Ornitier",
			fixtures: map[string]string{"Pay Probe": payProbeScript("U", "G U")},
			deck:     concatDecks(deckOf(10, "Vivi Ornitier"), deckOf(7, "Island"), deckOf(7, "Mountain"), deckOf(6, "Forest"), deckOf(5, giver), deckOf(5, "Pay Probe")),
			real:     []string{"Island", "Mountain", "Forest", "Vivi Ornitier", giver},
			seeds:    seedRange(1, 40), reach: vivi}
	}
	taxed := func(name, tax string) manaShapeBoard {
		return manaShapeBoard{name: name, first: "Vivi Ornitier",
			fixtures: map[string]string{"Vivi Spell": viviSpellScript},
			deck:     concatDecks(deckOf(10, "Vivi Ornitier"), deckOf(11, "Island"), deckOf(11, "Mountain"), deckOf(8, "Vivi Spell")),
			opp:      concatDecks(deckOf(26, "Island"), deckOf(14, tax)),
			real:     []string{"Island", "Mountain", "Vivi Ornitier", tax},
			seeds:    seedRange(1, 40), reach: vivi}
	}
	// the tax beside a grant: gorge's unless census prices the granted {T}
	// ability and not Vivi's {0}
	taxedGrant := taxed("rhystic_study_mantle", "Rhystic Study")
	taxedGrant.deck = concatDecks(deckOf(10, "Vivi Ornitier"), deckOf(9, "Island"), deckOf(9, "Mountain"), deckOf(6, "Paradise Mantle"), deckOf(6, "Vivi Spell"))
	taxedGrant.real = append(taxedGrant.real, "Paradise Mantle")
	spike := func(name, tax string, lands int) manaShapeBoard {
		c := taxed(name, "Island")
		c.fixtures = map[string]string{"Vivi Spell": viviSpellScript, "Spike Probe": spikeProbeScript(tax)}
		c.deck = concatDecks(deckOf(14, "Vivi Ornitier"), deckOf(lands, "Island"), deckOf(lands, "Mountain"), deckOf(26-2*lands, "Vivi Spell"))
		c.opp = concatDecks(deckOf(24, "Island"), deckOf(16, "Spike Probe"))
		c.real = []string{"Island", "Mountain", "Vivi Ornitier"}
		return c
	}
	// a ward {2} the payer elects and cannot cover: the failed payment taps
	// what the window lists, the Combo source among it
	ward := manaShapeBoard{name: "ward", first: "Vivi Ornitier",
		fixtures: map[string]string{"Shock Probe": shockProbeScript, "Ward Probe": strings.Replace(wardProbeScript, "Ward:1", "Ward:2", 1)},
		deck:     concatDecks(deckOf(14, "Vivi Ornitier"), deckOf(8, "Island"), deckOf(8, "Mountain"), deckOf(10, "Shock Probe")),
		opp:      concatDecks(deckOf(26, "Forest"), deckOf(14, "Ward Probe")),
		real:     []string{"Island", "Mountain", "Forest", "Vivi Ornitier"},
		seeds:    seedRange(1, 40), reach: vivi}
	// the same beside a grant: the flattened {0}, then the granted {T}
	wardGrant := ward
	wardGrant.name = "ward_mantle"
	wardGrant.fixtures = map[string]string{"Shock Probe": shockProbeScript, "Ward Probe": strings.Replace(wardProbeScript, "Ward:1", "Ward:3", 1)}
	wardGrant.deck = concatDecks(deckOf(10, "Vivi Ornitier"), deckOf(8, "Island"), deckOf(8, "Mountain"), deckOf(4, "Paradise Mantle"), deckOf(10, "Shock Probe"))
	wardGrant.real = []string{"Island", "Mountain", "Forest", "Vivi Ornitier", "Paradise Mantle"}
	wardGrant.seeds = seedRange(81, 120)
	// the any-colour sibling of the failed payment: a {T} Combo Any source
	wardAny := ward
	wardAny.name, wardAny.first = "ward_axebane", "Axebane Guardian"
	wardAny.fixtures = map[string]string{"Shock Probe": shockProbeScript, "Ward Probe": strings.Replace(wardProbeScript, "Ward:1", "Ward:4", 1)}
	wardAny.deck = concatDecks(deckOf(14, "Axebane Guardian"), deckOf(9, "Forest"), deckOf(7, "Mountain"), deckOf(10, "Shock Probe"))
	wardAny.real = []string{"Forest", "Mountain", "Axebane Guardian"}
	wardAny.reach = func(g *GorgeSide) bool { return defenderTapped(g, "Axebane Guardian") }
	return []manaShapeBoard{
		grant("mantle", "Paradise Mantle"),
		grant("hierophants", "Citanul Hierophants"),
		taxed("rhystic_study", "Rhystic Study"),
		taxedGrant,
		spike("counter_unless_one", "1", 5),
		spike("counter_unless_three", "3", 8),
		ward,
		wardGrant,
		wardAny,
	}
}

// TestHostedCmdrViviTaxedBoardsLockstep: the real Vivi Ornitier plays
// divergence-free at power 0 beside a granted ability and under an opponent's
// unless tax, and is activated on at least one seed of each board.
func TestHostedCmdrViviTaxedBoardsLockstep(t *testing.T) {
	for _, c := range viviTaxBoards() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			manaShapeBoardLockstep(t, c)
		})
	}
}
