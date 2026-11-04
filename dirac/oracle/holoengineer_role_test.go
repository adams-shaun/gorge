package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A Role maker beside (a singleton Holo-Engineer, so no order ask is posed) Holo-Engineer: gorge turns the Role mint into a plain
// 3/3 Illusion script mint (the Illusion is no Aura, so nothing is withheld),
// where the kernel's TokAttach path would look for a host for a non-Aura card.
// The kernel does not model it: it stalls (StallTokRewrite) before the mint, so
// no game diverges.
var holoRoleDecks = [2][]cardset.Entry{
	{
		{Name: "Holo-Engineer", Count: 1}, {Name: "Monstrous Rage", Count: 4}, {Name: "Embereth Veteran", Count: 4},
		{Name: "Dragon Fodder", Count: 3}, {Name: "Thraben Inspector", Count: 3}, {Name: "Raging Goblin", Count: 3},
		{Name: "Elite Vanguard", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
	},
	holoDecks[1],
}

func TestHoloEngineerRoleStall(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, _ := runHolo(t, holoRoleDecks, 96, false)
	t.Logf("%d games, %d divergences, stalls %v", games, divs, stalls)
	for r := range stalls {
		if r != "tokrewrite" && r != "board" {
			t.Errorf("stalled on %q, want only tokrewrite", r)
		}
	}
	if stalls["tokrewrite"] < 5 {
		t.Errorf("%d tokrewrite stalls in %d games, want at least 5", stalls["tokrewrite"], games)
	}
}
