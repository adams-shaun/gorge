package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The widened DamageDone doubler locksteps: the doubler whose ValidSource$
// names no controller (Card,Emblem — any source's, any controller's), its own
// Thrice (City on Fire, Fiery Emancipation), and the
// ValidTarget$ You / Card.Self words (Goldnight Castigator). Each deck plays
// at most two co-matching doubler lines, so the competitions the widened
// scan collects pose gorge's CR 616.1 order ask with the same candidates in
// the same seat-and-entry order both engines scan with (repl_multipliers.go's
// dblDmgMatch reads a foreign controller's source only through the
// controllerless alternative, and dealAsked multiplies each pick by its own
// line's word). Each pool has a mutation: the any-ctrl alternative narrowed
// to its controller, or a Thrice line flattened to Twice, makes some seed
// diverge — the lockstep tests the widened reading, not the cards around it.

var kdblAnySrcDecks = [2][]cardset.Entry{
	{
		{Name: "Dictate of the Twin Gods", Count: 1},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 11},
	},
	{
		{Name: "City on Fire", Count: 1},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 11},
	},
}

var kdblMultDecks = [2][]cardset.Entry{
	{
		{Name: "Furnace of Rath", Count: 1},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 11},
	},
	{
		{Name: "Fiery Emancipation", Count: 1},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Lava Axe", Count: 6}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 11},
	},
}

// kdblPlusDecks pair the Thrice with a lowered +N damage replacement
// (Torbran, Thane of Red Fell): the picks do not commute, so the doubler
// applies its own multiplier when it is picked (replOrderPick's roPlusPend
// arm / ncPlusPickNC), not a hard-coded 2.
var kdblPlusDecks = [2][]cardset.Entry{
	{
		{Name: "City on Fire", Count: 1},
		{Name: "Torbran, Thane of Red Fell", Count: 1},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Fiery Emancipation", Count: 1},
		{Name: "Torbran, Thane of Red Fell", Count: 1},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

var kdblYouSelfDecks = [2][]cardset.Entry{
	{
		{Name: "Goldnight Castigator", Count: 2},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Fiendish Duo", Count: 2},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

func TestDblAnySrcLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "dbl-anysrc", kdblAnySrcDecks, 400, "KDBLANYSRC_SEEDS")
	lockstepSet(t, "dbl-anysrc", kdblMultDecks, 400, "KDBLTHRICE_SEEDS")
	lockstepSet(t, "dbl-anysrc", kdblYouSelfDecks, 400, "KDBLYOUSELF_SEEDS")
	lockstepSet(t, "dbl-anysrc", kdblPlusDecks, 200, "KDBLPLUS_SEEDS")
}

// TestDblAnySrcLockstepMutation: narrowing the controllerless ValidSource$
// alternative to its controller (a foreign source stops matching), or
// flattening a Thrice line to Twice, diverges some seed.
func TestDblAnySrcLockstepMutation(t *testing.T) {
	t.Parallel()
	narrow := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				a := &tbl.Cards[i].Abilities[j]
				if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDoubleDamageToOpps || a.Repl.Filter == 0 {
					continue
				}
				for f := a.Repl.Filter; f != 0; f = tbl.Filters[f].Or {
					if tbl.Filters[f].Ctrl == kop.CtrlAny {
						tbl.Filters[f].Ctrl = kop.CtrlYou
					}
				}
			}
		}
	}
	if n := lockstepRun(t, "dbl-anysrc", kdblAnySrcDecks, 100, false, narrow); n == 0 {
		t.Fatal("narrowing the controllerless ValidSource$ alternative diverged no seed")
	}
	flat := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				a := &tbl.Cards[i].Abilities[j]
				if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDoubleDamageToOpps && a.Repl.A > 2 {
					a.Repl.A = 0
				}
			}
		}
	}
	if n := lockstepRun(t, "dbl-anysrc", kdblMultDecks, 100, false, flat); n == 0 {
		t.Fatal("flattening the Thrice doubler diverged no seed")
	}
	if n := lockstepRun(t, "dbl-anysrc", kdblPlusDecks, 100, false, flat); n == 0 {
		t.Fatal("flattening the Thrice beside the +N replacement diverged no seed")
	}
}
