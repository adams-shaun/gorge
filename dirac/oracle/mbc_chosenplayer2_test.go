package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The as-enters chosen opponent (dirac/kgen/lower_chosenopp.go): The Rack
// and Black Vise choose an opponent as they enter (ETB replacement
// ChoosePlayer, kop.ExtChosenOpp), trigger at the chosen player's upkeep
// (ValidPlayer$ Player.Chosen, kop.WhoseChosenPl) and deal damage to that
// player (Defined$ ChosenPlayer, kop.RefSrcChosenPl) counted from their hand
// (Count$ValidHand Card.ChosenCtrl). Entropic Specter and Nyxathid read the
// chosen player's hand size as P/T. The fixture pool is
// dirac/cardset/pools/fixtures/chosenopp.

var chosenOppDecks = []fixtureDecks{{
	{
		{Name: "The Rack", Count: 5}, {Name: "Black Vise", Count: 5}, {Name: "Entropic Specter", Count: 3},
		{Name: "Nyxathid", Count: 3}, {Name: "Hill Giant", Count: 6}, {Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "The Rack", Count: 4}, {Name: "Black Vise", Count: 4}, {Name: "Entropic Specter", Count: 2},
		{Name: "Nyxathid", Count: 2}, {Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 9}, {Name: "Forest", Count: 9},
	},
}}

var chosenOppWatch = map[string]bool{"The Rack": true, "Black Vise": true, "Entropic Specter": true, "Nyxathid": true}

// chosenOppLowered checks the lowered shape the lockstep exercises: each
// chooser opens its enters program with ExtChosenOpp, and the two artifacts
// trigger on WhoseChosenPl and damage RefSrcChosenPl.
func chosenOppLowered(t *testing.T, tbl *kop.Table) {
	t.Helper()
	for _, name := range []string{"The Rack", "Black Vise", "Entropic Specter", "Nyxathid"} {
		enters, whose, dmg := false, false, false
		for i := range tbl.Cards {
			cd := &tbl.Cards[i]
			if cd.Name != name {
				continue
			}
			if !cd.TwoSeat {
				t.Errorf("%s: lowered without the two-seat marker", name)
			}
			for j := range cd.Abilities {
				a := &cd.Abilities[j]
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram && len(a.Prog) == 1 &&
					a.Prog[0].Op == kop.OpExt && a.Prog[0].Ext() == kop.ExtChosenOpp {
					enters = true
				}
				if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigPhase && a.Trig.Whose == kop.WhoseChosenPl {
					whose = true
				}
				for x := range a.Prog {
					if a.Prog[x].Op == kop.OpDealDamage && a.Prog[x].Obj == kop.RefSrcChosenPl {
						dmg = true
					}
				}
			}
		}
		if !enters {
			t.Errorf("%s: no ExtChosenOpp enters program", name)
		}
		if artifact := name == "The Rack" || name == "Black Vise"; artifact && (!whose || !dmg) {
			t.Errorf("%s: WhoseChosenPl trigger %v, RefSrcChosenPl damage %v", name, whose, dmg)
		}
	}
}

func TestMBCChosenPlayerCard2Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "chosenopp", chosenOppDecks, chosenOppWatch, 160, true, func(tbl *kop.Table) { chosenOppLowered(t, tbl) })
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	for name := range chosenOppWatch {
		if run.casts[name] < 20 {
			t.Errorf("only %d %s casts", run.casts[name], name)
		}
	}
}

// TestMBCChosenPlayerCard2WhoseMutation fires the artifacts' trigger at
// their controller's upkeep instead of the chosen player's: the lockstep
// must diverge.
func TestMBCChosenPlayerCard2WhoseMutation(t *testing.T) {
	run := runFixtureLockstep(t, "chosenopp", chosenOppDecks, chosenOppWatch, 48, false, func(tbl *kop.Table) {
		n := 0
		for i := range tbl.Cards {
			cd := &tbl.Cards[i]
			if cd.Name != "The Rack" && cd.Name != "Black Vise" {
				continue
			}
			cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
			for j := range cd.Abilities {
				if a := &cd.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Whose == kop.WhoseChosenPl {
					a.Trig.Whose = kop.WhoseYou
					n++
				}
			}
		}
		if n != 2 {
			t.Fatalf("%d WhoseChosenPl triggers, want 2", n)
		}
	})
	run.requireDivergence(t, "the chosen player's upkeep read as the controller's")
}

// TestMBCChosenPlayerCard2DamageMutation deals the damage to the controller
// instead of the chosen player: the lockstep must diverge.
func TestMBCChosenPlayerCard2DamageMutation(t *testing.T) {
	run := runFixtureLockstep(t, "chosenopp", chosenOppDecks, chosenOppWatch, 48, false, func(tbl *kop.Table) {
		n := 0
		for _, name := range []string{"The Rack", "Black Vise"} {
			mutateCard(t, tbl, name, func(a *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpDealDamage && in.Obj == kop.RefSrcChosenPl {
					in.Obj = kop.RefController
					n++
				}
			})
		}
		if n != 2 {
			t.Fatalf("%d RefSrcChosenPl damage instructions, want 2", n)
		}
	})
	run.requireDivergence(t, "the chosen player's damage dealt to the controller")
}
