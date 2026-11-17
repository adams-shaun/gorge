package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An until-end-of-turn Effect's LifeGained Triggers$ (kgen emblemTimedMode,
// kernel fireEmblemsAmt and dropEOTEmblems): False Cure makes every player
// lose twice the life they gain this turn, Vizkopa Guildmage makes each
// opponent lose the life you gain this turn. Soul Warden, Healing Salve,
// Angel's Mercy and Knight of Meadowgrain gain the life, on both sides.
var effLifeGainedDecks = [2][]cardset.Entry{
	{
		{Name: "False Cure", Count: 4}, {Name: "Vizkopa Guildmage", Count: 6}, {Name: "Soul Warden", Count: 6}, {Name: "Healing Salve", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Soul Warden", Count: 6}, {Name: "Knight of Meadowgrain", Count: 6}, {Name: "Angel's Mercy", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 8},
	},
}

func TestEffectTriggersDurationLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "efflifegained", effLifeGainedDecks, 80, true, nil)
}

// TestEffectTriggersDurationLockstepMutation: with the emblems outliving the
// turn (UntilEOT dropped), some seed diverges.
func TestEffectTriggersDurationLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "efflifegained", effLifeGainedDecks, 80, false, func(tbl *kop.Table) {
		ems := append([]kop.Emblem(nil), tbl.Emblems...)
		for i := range ems {
			ems[i].UntilEOT = false
		}
		tbl.Emblems = ems
	})
	if n == 0 {
		t.Fatal("no seed diverged with the life-gain emblems outliving the turn")
	}
}
