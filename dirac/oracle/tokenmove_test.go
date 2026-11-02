package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// CR 111.8 a token that left the battlefield can't come
// back. Lilysplash Mentor and Splash Portal exile a target creature you
// control and return it in one resolution, so no state-based action runs
// between the moves; Dragon Fodder and Raise the Alarm give them token targets.
var ktokenmoveDeck = []cardset.Entry{
	{Name: "Lilysplash Mentor", Count: 6}, {Name: "Splash Portal", Count: 6}, {Name: "Dragon Fodder", Count: 5},
	{Name: "Raise the Alarm", Count: 5}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 2}, {Name: "Plains", Count: 2},
}

var ktokenmoveMatchups = [][2][]cardset.Entry{{ktokenmoveDeck, ktokenmoveDeck}}

func TestTokenmoveLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	sonRun(t, "tokenmove", ktokenmoveMatchups, 300, true, nil, nil)
}

// TestTokenmoveMutation: a token that moves again must diverge, which also
// proves the pool flickers tokens.
func TestTokenmoveMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugTokenMoves = true
	defer func() { dirac.DebugTokenMoves = false }()
	if n := sonRun(t, "tokenmove", ktokenmoveMatchups, 128, false, nil, nil); n == 0 {
		t.Errorf("mutation: no seed in 128 diverged")
	}
}
