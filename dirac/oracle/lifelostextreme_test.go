package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The life-lost extremes (gorge playerCountExtreme's LifeLostThisTurn arm):
// Sygg's end-step draw (PlayerCountRegisteredOpponents$HighestLifeLostThisTurn
// at 3), Bloodchief Ascension (the same head at 2), Knight of the Ebon Legion
// and Y'shtola (PlayerCountDefinedRegistered$, every living player, at 4),
// beside burn and Sign in Blood, which makes the caster lose life too.
var klifelostextremeDecks = [2][]cardset.Entry{{
	{Name: "Sygg, River Cutthroat", Count: 3}, {Name: "Bloodchief Ascension", Count: 3}, {Name: "Knight of the Ebon Legion", Count: 4},
	{Name: "Y'shtola, Night's Blessed", Count: 1}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
	{Name: "Sign in Blood", Count: 3}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 2}, {Name: "Plains", Count: 1},
}, {
	{Name: "Sygg, River Cutthroat", Count: 2}, {Name: "Bloodchief Ascension", Count: 2}, {Name: "Knight of the Ebon Legion", Count: 4},
	{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Sign in Blood", Count: 4},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 2},
}}

func TestLifeLostExtremeLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "lifelostextreme", klifelostextremeDecks, 300, "KLIFELOSTEXTREME_SEEDS")
}

// TestLifeLostExtremeMutation: a kernel that reads no life lost makes the
// gates differ from gorge's, so the pool must reach them.
func TestLifeLostExtremeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugOppLifeLostZero = true
	defer func() { dirac.DebugOppLifeLostZero = false }()
	if !lockstepDiverges(t, "lifelostextreme", klifelostextremeDecks, 64) {
		t.Fatal("DebugOppLifeLostZero: no divergence in 64 seeds")
	}
}
