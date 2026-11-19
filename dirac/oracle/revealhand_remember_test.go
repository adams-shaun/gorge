package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
)

// RevealHand with RememberRevealed$ True (Inquisition: "Target player reveals
// their hand. Inquisition deals damage to that player equal to the number of
// white cards in their hand"; Brain Pry: the discard of the named card, else a
// draw, reads the revealed hand). gorge effReveal appends each revealed card
// of the named player's hand to the resolution's remembered set, in hand
// order, and the chained links read it; dirac emits OpPeek PeekHand over the
// player ref (peekHand), so in a pod it reads the targeted opponent's hand
// whichever of the three that is. The mutation (DebugPeekHandNextSeat)
// remembers the next live seat's hand, the two-seat reading.
var revealHandRememberPodDeck = []cardset.Entry{
	{Name: "Inquisition", Count: 6}, {Name: "Brain Pry", Count: 6},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4},
	{Name: "Grizzly Bears", Count: 2},
	{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 6},
}

var revealHandRememberCards = map[string]bool{"Inquisition": true, "Brain Pry": true}

// revealHandRememberWatch: gorge asks for the target of a watched hand reveal
// with two or more opponents to choose among.
func revealHandRememberWatch(c *PickCtx) bool {
	g := c.G.E.G
	d := c.G.fr.d
	if d == nil || d.Kind != decision.KTarget || len(d.Options) < 3 {
		return false
	}
	ob := g.Obj(d.Source)
	return ob != nil && ob.Face() != nil && revealHandRememberCards[ob.Face().Name]
}

// TestFourSeatRevealHandRememberLockstep: four-seat games of RevealHand
// RememberRevealed$ play in lockstep, the remembered set the targeted
// player's hand.
func TestFourSeatRevealHandRememberLockstep(t *testing.T) {
	podClassLockstep(t, revealHandRememberPodDeck, 24, 10, revealHandRememberWatch, &dirac.DebugPeekHandNextSeat)
}
