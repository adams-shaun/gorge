package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC zone-moves-2 lockstep. Abolish: "You may discard a Plains card
// rather than pay this spell's mana cost" (an AlternativeCost of
// Discard<1/Plains>, CR 118.9, 601.2h: gorge offers the AltCostIndex 1 cast
// while a Plains other than the cast card is in hand). Magmatic Insight:
// "As an additional cost to cast this spell, discard a land card" (a spell
// Cost$ of R Discard<1/Land>). Campfire: "{2}, {T}, Exile Campfire: Put all
// commanders you own from the command zone and from your graveyard into your
// hand. Then shuffle your graveyard into your library." (no seat has a
// commander, so the sweep moves nothing and the graveyard shuffles). The
// expected answer at every decision is gorge's.
var kmbcZM2Decks = [2][]cardset.Entry{
	{
		{Name: "Abolish", Count: 6}, {Name: "Magmatic Insight", Count: 6}, {Name: "Campfire", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Abolish", Count: 4}, {Name: "Campfire", Count: 4}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Forest", Count: 12}, {Name: "Plains", Count: 10},
	},
}

// TestMBCZoneMoves2Lowers plays the pool in lockstep, then shows the pool
// plays each spelling: with Abolish's Plains filter dropped (any card pays),
// with Magmatic Insight's land filter dropped (any card is discarded) or with
// the alternative cost gone, some seed diverges.
func TestMBCZoneMoves2Lowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mbczonemoves2", kmbcZM2Decks, 400, "KMBCZM2_SEEDS")
	for _, m := range []struct {
		name string
		fn   func(*kop.CardDef)
	}{
		{"Abolish's discard filter dropped", func(c *kop.CardDef) {
			for j := range c.Abilities {
				if a := &c.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
					if a.Cost.Flags&kop.CostDiscardFiltered == 0 || a.Cost.N != 1 {
						t.Fatalf("Abolish's alternative cost is not a one-card filtered discard: %+v", a.Cost)
					}
					a.Cost.Flags &^= kop.CostDiscardFiltered
				}
			}
		}},
		{"no alternative cost", func(c *kop.CardDef) {
			for j := range c.Abilities {
				if a := &c.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
					a.Static.Kind = kop.StatNone
				}
			}
		}},
		{"Magmatic Insight's discard filter dropped", func(c *kop.CardDef) {
			for j := range c.Abilities {
				if a := &c.Abilities[j]; a.Kind == kop.AbSpell && a.Cost.Flags&kop.CostDiscard != 0 {
					if a.Cost.Flags&kop.CostDiscardFiltered == 0 {
						t.Fatalf("Magmatic Insight's cost is not a filtered discard: %+v", a.Cost)
					}
					a.Cost.Flags &^= kop.CostDiscardFiltered
				}
			}
		}},
		{"Campfire's graveyard shuffle dropped", func(c *kop.CardDef) {
			for j := range c.Abilities {
				a := &c.Abilities[j]
				if a.Kind != kop.AbActivated || a.Cost.Flags&kop.CostExileSelf == 0 {
					continue
				}
				var keep []kop.Instr
				for _, in := range a.Prog {
					if in.Op != kop.OpShuffle {
						keep = append(keep, in)
					}
				}
				if len(keep) == len(a.Prog) {
					t.Fatalf("Campfire's exile-self ability has no shuffle: %+v", a.Prog)
				}
				a.Prog = keep
			}
		}},
	} {
		n := lockstepRun(t, "mbczonemoves2", kmbcZM2Decks, 200, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				cards[i].Abilities = append([]kop.Ability(nil), cards[i].Abilities...)
				m.fn(&cards[i])
			}
			tbl.Cards = cards
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged: the pool never plays this spelling, or the kernel does not price it", m.name)
		}
	}
}
