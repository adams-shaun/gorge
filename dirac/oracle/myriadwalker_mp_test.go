package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Myriad's walker arm (CR 702.116a: "attacking that player or a planeswalker
// they control"): gorge's effMyriad offers, per asked opponent, the player,
// each face-up planeswalker that opponent controls (battlefield order), then
// the decline (myriadAskOptions); a walker yes makes the copy attack that
// walker (myriadCreate). dirac poses the same options (kw_myriad.go
// myriadOptions) and the copy enters attacking the walker. The mutation
// (DebugMyriadNoWalker) offers only the player and the decline.
var myriadWalkerPodDeck = []cardset.Entry{
	{Name: "Genasi Enforcers", Count: 8}, {Name: "Jace Beleren", Count: 6},
	{Name: "Ajani, Caller of the Pride", Count: 4}, {Name: "Savannah Lions", Count: 4},
	{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 6},
}

// myriadWalkerWatch: gorge asks a Myriad trigger's controller about an
// opponent who controls a planeswalker (more than the yes/no pair offered).
func myriadWalkerWatch(c *PickCtx) bool {
	d := c.G.fr.d
	return d != nil && d.ResumeKind == "myriad" && len(d.Options) > 2
}

// TestFourSeatMyriadWalkerLockstep: four-seat Myriad games play in lockstep
// when an asked opponent controls a planeswalker, the walker arm offered and
// attacked.
func TestFourSeatMyriadWalkerLockstep(t *testing.T) {
	podClassLockstep(t, myriadWalkerPodDeck, 24, 3, myriadWalkerWatch, &dirac.DebugMyriadNoWalker)
}
