package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The filter-head lockstep: DamageAll ValidPlayers$ Targeted (Chandra's Flame
// Wave, Chandra, Flame's Fury, Chandra, Bold Pyromancer: the sweep scoped to
// the ValidTgts$ Player's permanents, then that player; Cerebral Eruption: the
// PeekAndReveal Opponent slot, unscoped). Both decks carry creatures for the
// sweeps to hit on either side, and burn to thin them. Player.Other (Night
// Dealings) has no lockstep: its THEFT counter does not build yet.
var kfilterHeadsDecks = [2][]cardset.Entry{
	{
		{Name: "Chandra's Flame Wave", Count: 4}, {Name: "Chandra, Flame's Fury", Count: 2},
		{Name: "Chandra, Bold Pyromancer", Count: 2}, {Name: "Cerebral Eruption", Count: 4},
		{Name: "Lightning Bolt", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Mountain", Count: 15}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 14}, {Name: "Swamp", Count: 4},
	},
}

func TestFilterHeadsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "filterheadsref", kfilterHeadsDecks, 400, "KFILTERHEADS_SEEDS")
}

// filterHeadsDamageAllBits rewrites every DamageAll's B.N bits through f.
func filterHeadsDamageAllBits(f func(int16) int16) func(*kop.Table) {
	return func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for k := range prog {
					if prog[k].Op == kop.OpDamageAll && prog[k].B.N&8 != 0 {
						prog[k].B.N = f(prog[k].B.N)
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
}

// TestFilterHeadsLockstepMutationPlayer: with the targeted-player bit dropped
// (the sweep hits the permanents only), some seed diverges.
func TestFilterHeadsLockstepMutationPlayer(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "filterheadsref", kfilterHeadsDecks, 200, false,
		filterHeadsDamageAllBits(func(b int16) int16 { return b &^ 8 }))
	if n == 0 {
		t.Fatal("no seed diverged with the targeted player left undamaged")
	}
}

// TestFilterHeadsLockstepMutationScope: with the scope bit dropped (the sweep
// hits every seat's permanents), some seed diverges, so a ValidTgts$ Player
// sweep really meets a creature of the player it does not target.
func TestFilterHeadsLockstepMutationScope(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "filterheadsref", kfilterHeadsDecks, 200, false,
		filterHeadsDamageAllBits(func(b int16) int16 { return b &^ 16 }))
	if n == 0 {
		t.Fatal("no seed diverged with the scope bit dropped")
	}
}
