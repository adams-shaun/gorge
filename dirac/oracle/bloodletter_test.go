package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorgeLifeReducedFixTime is gorge cr-lifereduced-twice-not-damage
// (d86d838e0, 2026-10-26 10:36:59 UTC): before it Bloodletter's doubled
// life loss doubled the damage itself.
const gorgeLifeReducedFixTime = "20261026103659"

// The Bloodletter lockstep: Bloodletter of Aclazotz doubles the
// life an opponent would lose during its controller's turn (ReplLifeLossDouble
// in loseLife): combat damage, Shock, Sovereign's Bite's life loss; not on
// the other turn (Shock at instant speed, Night's Whisper). Two Bloodletters
// commute (four times). The expected answer at every decision is gorge's.
var kbloodletterDecks = [2][]cardset.Entry{
	{
		{Name: "Bloodletter of Aclazotz", Count: 8}, {Name: "Sovereign's Bite", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Night's Whisper", Count: 4},
		{Name: "Bloodletter of Aclazotz", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Swamp", Count: 4},
	},
}

func TestBloodletterLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeLifeReducedFixTime) {
		t.Skip("gorge before cr-lifereduced-twice-not-damage (d86d838e0)")
	}
	lockstepSet(t, "bloodletter", kbloodletterDecks, 600, "KBLOODLETTER_SEEDS")
}

// TestBloodletterLockstepMutation: with the doubler gone, some seed diverges.
func TestBloodletterLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeLifeReducedFixTime) {
		t.Skip("gorge before cr-lifereduced-twice-not-damage (d86d838e0)")
	}
	n := lockstepRun(t, "bloodletter", kbloodletterDecks, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbReplacement && abs[j].Repl.Kind == kop.ReplLifeLossDouble {
					abs[j].Repl.Kind = kop.ReplNone
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the life-loss doubler dropped")
	}
}
