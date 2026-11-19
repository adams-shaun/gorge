package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Dig and DigUntil with RememberRevealed$ True, read by the next link of the
// same resolution (Remembered$Amount, ConditionPresent over Remembered).
// gorge's effDig appends the whole Reveal$ window (the top DigNum$ cards) to
// the resolution's remembered set before it filters or moves a card
// (Stomping Slabs: "if a card named Stomping Slabs was revealed", Track Down:
// "if it's a creature or land card", Dihada's Treasure count); effDigUntil
// appends every scanned card, found and revealed, in scan order (Spoils of
// the Vault: life lost for each exiled card). dirac remembers the Dig window
// with a public Peek just before the Dig (lower_digwindow.go) and the
// DigUntil scan with DuRememberRevealed. The mutations remember the top card
// only (DebugPeekWindowOne) and the last scanned card only
// (DebugDigRememberLastOnly). The found-zBattlefield DigUntil carriers
// (Madcap Experiment, Audacious Reshapers) are refused: the found card
// entering the battlefield is never read by the lowering. (The once-
// suspected sac-source prefix spin in resolvePrefixStack is fixed --
// sacsource_prefixspin_test.go -- but the carriers still have no lowering
// to lockstep against.)

// remRevealWatch counts, per carrier name, the decisions posed with a carrier
// on the stack or as the source (the carrier resolved or was about to).
func remRevealWatch(names map[string]bool, hits map[string]int) func(c *PickCtx) bool {
	return func(c *PickCtx) bool {
		g := c.G.E.G
		seen := false
		if d := c.G.fr.d; d != nil {
			if ob := g.Obj(d.Source); ob != nil && ob.Face() != nil && names[ob.Face().Name] {
				hits[ob.Face().Name]++
				seen = true
			}
		}
		for _, id := range g.Stack {
			if ob := g.Obj(id); ob != nil && ob.Face() != nil && names[ob.Face().Name] {
				hits[ob.Face().Name]++
				seen = true
			}
		}
		return seen
	}
}

var digRememberPodDeck = []cardset.Entry{
	{Name: "Stomping Slabs", Count: 6}, {Name: "Track Down", Count: 6},
	{Name: "Dihada, Binder of Wills", Count: 3},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 2},
	{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 5},
	{Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3},
}

// TestFourSeatDigRememberRevealedLockstep: four-seat Dig RememberRevealed$
// games play in lockstep, the window the Dig revealed remembered.
func TestFourSeatDigRememberRevealedLockstep(t *testing.T) {
	corpusDirOrSkip(t) // the watch assertions below need the pod to have played
	names := map[string]bool{"Stomping Slabs": true, "Track Down": true, "Dihada, Binder of Wills": true}
	hits := map[string]int{}
	podClassLockstep(t, digRememberPodDeck, 24, 10, remRevealWatch(names, hits), &dirac.DebugPeekWindowOne)
	t.Logf("decisions with a carrier on the stack, by card: %v", hits)
	for n := range names {
		if hits[n] == 0 {
			t.Errorf("no decision with %s on the stack", n)
		}
	}
}

var digUntilRememberPodDeck = []cardset.Entry{
	{Name: "Spoils of the Vault", Count: 8},
	{Name: "Ornithopter", Count: 6},
	{Name: "Grizzly Bears", Count: 2},
	{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 7},
}

// TestFourSeatDigUntilRememberRevealedLockstep: four-seat DigUntil
// RememberRevealed$ games play in lockstep, the scan window remembered. The
// found-zBattlefield carriers (Madcap Experiment, Audacious Reshapers) are
// refused (lower_digimprint.go: found-zBattlefield lowering unimplemented),
// so
// only the found-zHand carrier is dealt.
func TestFourSeatDigUntilRememberRevealedLockstep(t *testing.T) {
	corpusDirOrSkip(t) // the watch assertions below need the pod to have played
	names := map[string]bool{"Spoils of the Vault": true}
	hits := map[string]int{}
	podClassLockstep(t, digUntilRememberPodDeck, 24, 10, remRevealWatch(names, hits), &dirac.DebugDigRememberLastOnly)
	t.Logf("decisions with a carrier on the stack, by card: %v", hits)
	for n := range names {
		if hits[n] == 0 {
			t.Errorf("no decision with %s on the stack", n)
		}
	}
}
