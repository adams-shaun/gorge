package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A bounce land's one ability adds two colours at once. With nothing else to
// tap, a {U}{U} cast takes two of them (each gives {W}{U}, so the white
// floats); the kernel's autopay plan must find the same payment gorge does.
var kchanceryDecks = [2][]cardset.Entry{
	{
		{Name: "Azorius Chancery", Count: 8}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Plated Seastrider", Count: 8},
		{Name: "Mahamoti Djinn", Count: 4}, {Name: "Phantom Monster", Count: 8}, {Name: "Counterspell", Count: 2},
	},
	{
		{Name: "Azorius Chancery", Count: 8}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Plated Seastrider", Count: 8},
		{Name: "Phantom Monster", Count: 8}, {Name: "Air Elemental", Count: 3}, {Name: "Plated Seastrider", Count: 3},
	},
}

func TestBounceLandPaysDoubleColourLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "chancery", kchanceryDecks, 200, "KCHANCERY_SEEDS")
}
