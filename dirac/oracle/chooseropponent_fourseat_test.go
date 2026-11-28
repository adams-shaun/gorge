package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// chooseropponent_fourseat_test.go: a Chooser$ Opponent pick over the cards
// your own library search remembered (Realms Uncharted's "An opponent chooses
// two of those cards") plays four-seat games in lockstep. gorge's
// chooserPlayer answers the pick as the first living seat of
// AliveFrom(controller) that is not the controller; the kernel's chooserOf
// reads the same seat (RefOpponent, oneOpp's nextAlive), so the pick is posed
// to one seat on both sides and the class is not two-seat only (the
// re-spelled Chooser$ Targeted of lower_tutorpick.go is, and stays so: that
// one must reach the announced target, not the first opponent).
var chooserOpponentPodDeck = []cardset.Entry{
	{Name: "Realms Uncharted", Count: 8},
	{Name: "Llanowar Elves", Count: 4},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Sol Ring", Count: 2},
	{Name: "Forest", Count: 10},
}

// chooserOpponentWatch: gorge poses a card pick to a player who does not own
// the offered cards, so an opponent answers someone else's search. The
// searcher's own pick of the found lands (asked of the owner) does not count.
func chooserOpponentWatch(c *PickCtx) bool {
	if c.G == nil || c.G.E == nil || c.G.fr == nil || c.G.fr.d == nil {
		return false
	}
	g := c.G.E.G
	if g == nil {
		return false
	}
	d := c.G.fr.d
	if d.Kind != decision.KChoose {
		return false
	}
	for _, o := range d.Options {
		if o.Kind != "search" && o.Kind != "card" {
			continue
		}
		ob := g.Obj(o.Obj)
		if ob != nil && ob.Owner != state.PlayerID(d.Player) {
			return true
		}
	}
	return false
}

// TestFourSeatChooserOpponentLockstep: four-seat Realms Uncharted games, one
// deck in every seat, play in lockstep with gorge with several
// opponent-answered picks watched. Restoring the removed lc.twoSeat mark at
// libSearchOwner (lower_zone.go) fails the run here at its TwoSeatOnly check;
// reading the kernel's chooser as any other seat diverges at the ask's player
// (TestDemoTutorPickMutation pins the same read against the searcher at two
// seats).
func TestFourSeatChooserOpponentLockstep(t *testing.T) {
	var unused bool
	st := runPodClass(t, chooserOpponentPodDeck, 24, chooserOpponentWatch, &unused, false)
	t.Logf("%d games, %d divergences, %d stalls, %d watched opponent-answered picks", st.games, st.divs, st.stalls, st.hits)
	if st.divs != 0 {
		t.Errorf("%d divergences", st.divs)
	}
	if st.stalls != 0 {
		t.Errorf("%d stalls", st.stalls)
	}
	if st.hits < 5 {
		t.Errorf("only %d watched opponent-answered picks, want >= 5", st.hits)
	}
}
