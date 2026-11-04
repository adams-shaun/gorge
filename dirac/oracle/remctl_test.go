package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The remembered controller: Rules of Acquisition's first mode remembers the
// controller of the opponent's permanent it targets, gains control of that
// permanent, then has the remembered player draw three (CR 608.2c: the
// player is read as the Pump link resolves, before control changes). gorge
// remembers the player on the source; the kernel lowers the read as the one
// opponent (dirac/kgen/lower_remembered.go pumpRememberOppController). The
// second mode gives one of your permanents to the targeted opponent and
// draws only if it moved. The fixture pool is
// dirac/cardset/pools/fixtures/remctl.

var remCtlDecks = []fixtureDecks{{
	{
		{Name: "Rules of Acquisition", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 12}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Rules of Acquisition", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}}

var remCtlWatch = map[string]bool{"Rules of Acquisition": true}

func TestRemCtlLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "remctl", remCtlDecks, remCtlWatch, 200, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Rules of Acquisition"] < 60 {
		t.Errorf("only %d Rules of Acquisition casts", run.casts["Rules of Acquisition"])
	}
}

// TestRemCtlMutation has the caster draw instead of the remembered player,
// as reading the permanent's controller after the control change would: the
// lockstep must diverge.
func TestRemCtlMutation(t *testing.T) {
	run := runFixtureLockstep(t, "remctl", remCtlDecks, remCtlWatch, 64, false, func(tbl *kop.Table) {
		mutateCard(t, tbl, "Rules of Acquisition", func(a *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpDraw && (in.Obj == kop.RefEachOpponent || in.Obj == kop.RefOpponent) {
				in.Obj = kop.RefController
			}
		})
	})
	run.requireDivergence(t, "the caster draws for the remembered player")
}
