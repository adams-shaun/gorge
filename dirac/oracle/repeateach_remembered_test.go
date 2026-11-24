package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatEachRememberedLowers plays the remembered-set RepeatEach in
// lockstep with gorge over the repeatremembered fixture pool
// (dirac/cardset/pools/fixtures/repeatremembered): "for each permanent
// destroyed / creature exiled this way, its controller creates a token"
// (Rampage of the Clans, Curse of the Swine; Will, Scholar of Frost's
// ultimate lowers the same shape but needs seven loyalty). The loop is
// OpRepeatEach over g.rem (kop.RepeatRemembered) minting for RefEach, the
// repeated card's controller: its last known one once it left the battlefield
// (Curse's RememberLKI$ capture, one per exiled creature).
//
// Preconditions: Rampage and Curse are cast; Control Magic is cast so some
// exiled creature has a controller who is not its owner. Mutations: the token
// minted for the caster instead of the card's controller; the kernel's
// last-controller capture ignored (the owner read instead).
func TestRepeatEachRememberedLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstep(t, "repeatremembered", repeatRememberedDecks, repeatRememberedWatch, 200, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		for name, min := range map[string]int{"Rampage of the Clans": 10, "Curse of the Swine": 10, "Control Magic": 10} {
			if run.casts[name] < min {
				t.Errorf("only %d %s casts (want %d)", run.casts[name], name, min)
			}
		}
	})
	t.Run("mutation/TokenForCaster", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeatremembered", repeatRememberedDecks, repeatRememberedWatch, 200, false, func(tbl *kop.Table) {
			for _, name := range []string{"Rampage of the Clans", "Curse of the Swine"} {
				mutateCardIfPresent(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpCreateToken && in.Obj == kop.RefEach {
						in.Obj = kop.RefController
						n++
					}
				})
			}
		})
		if n == 0 {
			t.Fatal("no CreateToken minted for RefEach in the table (the remembered loop is not lowered)")
		}
		run.requireDivergence(t, "the remembered loop's tokens minted for the caster")
	})
}

func TestRepeatEachRememberedLowersMutationCapture(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRemCtlOff = true
	defer func() { dirac.DebugRemCtlOff = false }()
	run := runFixtureLockstep(t, "repeatremembered", repeatRememberedDecks, repeatRememberedWatch, 200, false, nil)
	run.requireDivergence(t, "the exiled creature's last controller read as its owner")
}

var repeatRememberedWatch = map[string]bool{
	"Rampage of the Clans": true, "Curse of the Swine": true, "Control Magic": true,
}

var repeatRememberedDecks = []fixtureDecks{
	{
		// blue-green: the two spells, Control Magic, creatures and a Treasure maker
		{
			{Name: "Rampage of the Clans", Count: 5}, {Name: "Curse of the Swine", Count: 5}, {Name: "Control Magic", Count: 4},
			{Name: "Prosperous Innkeeper", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Bonesplitter", Count: 2},
			{Name: "Forest", Count: 7}, {Name: "Island", Count: 7},
		},
		{
			{Name: "Bonesplitter", Count: 4}, {Name: "Spirit Link", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Prosperous Innkeeper", Count: 2}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
		},
	},
	{
		{
			{Name: "Rampage of the Clans", Count: 4}, {Name: "Curse of the Swine", Count: 6}, {Name: "Control Magic", Count: 6},
			{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Spirit Link", Count: 2},
			{Name: "Forest", Count: 7}, {Name: "Island", Count: 7},
		},
		{
			{Name: "Rampage of the Clans", Count: 4}, {Name: "Curse of the Swine", Count: 4}, {Name: "Control Magic", Count: 2},
			{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Bonesplitter", Count: 2}, {Name: "Spirit Link", Count: 2},
			{Name: "Forest", Count: 7}, {Name: "Island", Count: 7},
		},
	},
}
