package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Ensign Pavel Chekov lockstep: when it enters, the opponent exiles the top
// X cards of their library (X the differently named lands the controller
// controls), the controller chooses a land card the opponent owns among them
// (an ask even for one candidate, none without one) and may play the chosen
// land for as long as it stays exiled. The expected answer at every decision
// is gorge's. The decks differ in land names (X varies from 1 to 3), carry
// lands among the exiled cards at about 37% of the library, and both play
// Chekov, so each side's lands are exiled and chosen too. dirac is a
// two-seat engine: "each opponent" is one opponent, one iteration.
var kchekovMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Ensign Pavel Chekov", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Wall of Omens", Count: 3}, {Name: "Elvish Visionary", Count: 3}, {Name: "Prodigal Sorcerer", Count: 2},
		{Name: "Counterspell", Count: 2}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Ensign Pavel Chekov", Count: 3}, {Name: "Savannah Lions", Count: 4}, {Name: "Phyrexian Rager", Count: 3},
		{Name: "Wall of Omens", Count: 3}, {Name: "Mind Stone", Count: 3}, {Name: "Murder", Count: 3},
		{Name: "Unsummon", Count: 2}, {Name: "Counterspell", Count: 2}, {Name: "Prodigal Sorcerer", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 5},
	},
}, {
	{
		{Name: "Ensign Pavel Chekov", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Wall of Omens", Count: 3}, {Name: "Elvish Visionary", Count: 3}, {Name: "Prodigal Sorcerer", Count: 2},
		{Name: "Counterspell", Count: 2}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Ensign Pavel Chekov", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Wall of Omens", Count: 3}, {Name: "Elvish Visionary", Count: 3}, {Name: "Prodigal Sorcerer", Count: 2},
		{Name: "Counterspell", Count: 2}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4},
	},
}}

type kchekovTally struct {
	exiled, landsPlayed int
}

// kchekovCheck counts, on the gorge side, the cards exiled from a library
// (the Dig) and the cards moving from exile to the battlefield (the only
// exile-to-battlefield move of these decks is a land played under Chekov's
// permission).
func kchekovCheck(tal *kchekovTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			if ev := evs[seen]; ev.Kind == events.MoveZone {
				switch {
				case ev.From == state.ZLibrary && ev.To == state.ZExile:
					tal.exiled++
				case ev.From == state.ZExile && ev.To == state.ZBattlefield:
					tal.landsPlayed++
				}
			}
		}
		return nil
	}
}

func TestChekovLockstep(t *testing.T) {
	t.Parallel()
	var tal kchekovTally
	sonRun(t, "chekov", kchekovMatchups, 250, true, nil, kchekovCheck(&tal))
	t.Logf("500 games: %d cards exiled from a library, %d lands played from exile", tal.exiled, tal.landsPlayed)
	if tal.exiled < 400 || tal.landsPlayed < 100 {
		t.Errorf("exiled %d (want >= 400), lands played from exile %d (want >= 100)", tal.exiled, tal.landsPlayed)
	}
}

// TestChekovMutation: the wrong player's land chosen (the controller's own),
// the choice dropped, or the play permission dropped, makes some seed
// diverge.
func TestChekovMutation(t *testing.T) {
	dirac.DebugExileLandOwn = true
	n := sonRun(t, "chekov", kchekovMatchups, 100, false, nil, nil)
	dirac.DebugExileLandOwn = false
	if n == 0 {
		t.Fatal("DebugExileLandOwn: no seed diverged")
	}
	for _, drop := range []func(kop.Instr) bool{
		func(in kop.Instr) bool { return in.Ext() == kop.ExtExileLandChoose },
		func(in kop.Instr) bool { return in.Op == kop.OpMayPlay },
	} {
		n := sonRun(t, "chekov", kchekovMatchups, 100, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				abs := append([]kop.Ability(nil), cards[i].Abilities...)
				for j := range abs {
					p := append([]kop.Instr(nil), abs[j].Prog...)
					for x := range p {
						if drop(p[x]) {
							p[x] = kop.Instr{Op: kop.OpNop}
						}
					}
					abs[j].Prog = p
				}
				cards[i].Abilities = abs
			}
			tbl.Cards = cards
		}, nil)
		if n == 0 {
			t.Fatal("no seed diverged with an instruction dropped")
		}
	}
}
