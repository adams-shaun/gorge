package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC combat-1 spelling family, lowered in lockstep with gorge:
//
//   - K:Bushido:N (Hand of Honor, Kitsune Blademaster, Samurai of the Pale
//     Curtain, Konda): gorge expands it into a Blocks and an AttackerBlocked
//     trigger on Card.Self, each pumping the creature +N/+N until end of turn
//     (CR 702.45a); kgen lowers those two lines.
//   - A granted landwalk from an Equipment (Trailblazer's Boots, "Landwalk:
//     Land.nonBasic"): the static's filter is the equipped creature and its
//     Landwalk filter reads the defender's lands (kop.Static.Landwalk).
//   - A Pump's landwalk (Piracy Charm, "Landwalk:Island" until end of turn):
//     an end-of-turn override record carrying the land filter
//     (kop.Override.Landwalk).
//
// The fixture pool is dirac/cardset/pools/fixtures/mbccombat1. Without the
// lowerings kgen refuses these cards and the pool has no table for them, so the
// test fails to build its matchups.

var mbcCombat1Decks = []fixtureDecks{{
	{
		{Name: "Hand of Honor", Count: 3}, {Name: "Kitsune Blademaster", Count: 3}, {Name: "Samurai of the Pale Curtain", Count: 3},
		{Name: "Konda, Lord of Eiganjo", Count: 2}, {Name: "Trailblazer's Boots", Count: 4}, {Name: "Piracy Charm", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Trailblazer's Boots", Count: 3},
		{Name: "Piracy Charm", Count: 3}, {Name: "Kitsune Blademaster", Count: 2}, {Name: "Hallowed Fountain", Count: 4},
		{Name: "Tranquil Cove", Count: 4}, {Name: "Island", Count: 7}, {Name: "Plains", Count: 7},
	},
}, {
	{
		{Name: "Trailblazer's Boots", Count: 4}, {Name: "Piracy Charm", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hallowed Fountain", Count: 3}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Hand of Honor", Count: 4}, {Name: "Samurai of the Pale Curtain", Count: 4}, {Name: "Konda, Lord of Eiganjo", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Tranquil Cove", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Island", Count: 10},
	},
}}

var mbcCombat1Watch = map[string]bool{
	"Hand of Honor": true, "Kitsune Blademaster": true, "Samurai of the Pale Curtain": true, "Konda, Lord of Eiganjo": true,
	"Trailblazer's Boots": true, "Piracy Charm": true,
}

func TestMBCCombat1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "mbccombat1", mbcCombat1Decks, mbcCombat1Watch, 200, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	for name := range mbcCombat1Watch {
		if run.casts[name] == 0 {
			t.Errorf("%s was never cast: the lockstep did not exercise it", name)
		}
	}
}

// TestMBCCombat1LowersMutation: each rewrite of the lowered table must make the
// lockstep diverge, so the games above do exercise the lowering. 1 drops the
// Bushido pump (the +N/+N a block gives), 2 drops the Equipment's granted
// landwalk filter, 3 drops the Pump's landwalk filter.
func TestMBCCombat1LowersMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	bushido := []string{"Hand of Honor", "Kitsune Blademaster", "Samurai of the Pale Curtain", "Konda, Lord of Eiganjo"}
	for _, m := range []struct {
		what   string
		mutate func(t *testing.T, tbl *kop.Table)
	}{
		{"Bushido pumps nothing", func(t *testing.T, tbl *kop.Table) {
			for _, n := range bushido {
				if !slices.ContainsFunc(tbl.Cards, func(c kop.CardDef) bool { return c.Name == n }) {
					continue // this matchup's decks do not hold it
				}
				mutateCard(t, tbl, n, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpPumpUntilEOT {
						in.A, in.B = kop.Const(0), kop.Const(0)
					}
				})
			}
		}},
		{"the Equipment's landwalk is dropped", func(t *testing.T, tbl *kop.Table) {
			dropped := false
			for i := range tbl.Cards {
				cd := &tbl.Cards[i]
				if cd.Name != "Trailblazer's Boots" {
					continue
				}
				cd.Abilities = slices.Clone(cd.Abilities)
				for j := range cd.Abilities {
					if a := &cd.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Landwalk != 0 {
						a.Static.Landwalk, dropped = 0, true
					}
				}
			}
			if !dropped {
				t.Fatal("Trailblazer's Boots has no landwalk static")
			}
		}},
		{"the Pump's landwalk is dropped", func(t *testing.T, tbl *kop.Table) {
			tbl.Overrides = slices.Clone(tbl.Overrides)
			dropped := false
			for i := range tbl.Overrides {
				if tbl.Overrides[i].Landwalk != 0 {
					tbl.Overrides[i].Landwalk, dropped = 0, true
				}
			}
			if !dropped {
				t.Fatal("no override carries a landwalk")
			}
		}},
	} {
		t.Run(m.what, func(t *testing.T) {
			run := runFixtureLockstep(t, "mbccombat1", mbcCombat1Decks, mbcCombat1Watch, 300, false, func(tbl *kop.Table) { m.mutate(t, tbl) })
			run.requireDivergence(t, m.what)
		})
	}
}
