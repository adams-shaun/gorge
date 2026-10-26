package rules

// CR 305.2a: a land play is legal when the number of lands the player can
// play this turn exceeds the number already played, "including lands played
// as special actions and lands played during the resolution of spells and
// abilities". CR 305.3: "A player can't play a land, for any reason, if it
// isn't their turn. ... Similarly, a player can't play a land, for any
// reason, if that player has used all of their land plays for that turn."
// beginPlay is the rules hand-off every Play effect (Djinn of Wishes,
// Courser-style reveals, cascade-like plays) reaches; CastProhibited is the
// effects-side election filter that keeps an unplayable card out of the
// Play ask.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR305_3EffectLandPlayNeedsOwnTurn: seat 0 holds a Forest with no land
// played, but it is seat 1's turn. A Play effect cannot play it, and the
// election filter does not offer it.
func TestCR305_3EffectLandPlayNeedsOwnTurn(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "f/forest.txt")))
	forest := e.G.Zone(state.ZHand, 0)[0]
	e.G.Active = 1
	if !e.CastProhibited(0, forest) {
		t.Errorf("CR 305.3: the Play election offers a land on another player's turn")
	}
	e.beginPlay(0, forest, true, "", false, false, "")
	if z := e.G.Obj(forest).Zone; z != state.ZHand {
		t.Fatalf("CR 305.3: a Play effect put the Forest into %s on seat 1's turn, want it left in hand", z)
	}
	if n := e.G.Players[0].LandsPlayed; n != 0 {
		t.Fatalf("CR 305.3: LandsPlayed = %d after a refused off-turn play, want 0", n)
	}
}

// TestCR305_2aEffectLandPlayUsesExtraDrop: seat 0's own turn, one land
// already played, Exploration ("You may play an additional land on each of
// your turns") on the battlefield. A Play effect plays the Forest with the
// second land play; a third is refused.
func TestCR305_2aEffectLandPlayUsesExtraDrop(t *testing.T) {
	t.Parallel()
	forestCard := card(t, corpusCardText(t, "f/forest.txt"))
	e := handEngine(t, forestCard, forestCard)
	first, second := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	onBoardGrant(t, e, 0, corpusCardText(t, "e/exploration.txt"))
	e.G.Players[0].LandsPlayed = 1
	if e.CastProhibited(0, first) {
		t.Errorf("CR 305.2a: the Play election withholds a land although Exploration leaves a land play")
	}
	e.beginPlay(0, first, true, "", false, false, "")
	if z := e.G.Obj(first).Zone; z != state.ZBattlefield {
		t.Fatalf("CR 305.2a: the Play effect left the Forest in %s with Exploration's second land play unused, want the battlefield", z)
	}
	if n := e.G.Players[0].LandsPlayed; n != 2 {
		t.Fatalf("LandsPlayed = %d after the effect play, want 2 (CR 305.2a counts it)", n)
	}
	e.beginPlay(0, second, true, "", false, false, "")
	if z := e.G.Obj(second).Zone; z != state.ZHand {
		t.Fatalf("CR 305.3: a third land was played with two land plays available, zone %s", z)
	}
}
