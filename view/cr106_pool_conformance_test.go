package view

// Reference: Magic: The Gathering Comprehensive Rules, 2026-08-07.
// CR 106.4a: line 578 ("announces what mana is still there").
// CR 106.4b: line 581 ("announces what mana is there").
// CR 117.3d: line 1641 ("they announce what mana is there").
// CR 118.3a: line 1677 ("the player announces what mana is still there").
// CR 400.1: line 3629 (the seven zones; a mana pool is not one of them).
// CR 400.2: line 3633 (hidden zones: library and hand).
//
// This closes a measured divergence. The projection lives in `view/`, so the
// oracle sits here rather than in `rules/` (whose cr*_conformance_test.go
// files drive the engine). Unlike the rules lane, this asserts the CORRECT
// behaviour and carries no GORGE_CR_CONFORMANCE opt-in: the divergence is now
// fixed, so the ordinary suite must defend it from regressing back to
// owner-only redaction.

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestCR106ManaPoolIsPublicForEveryPlayer pins that a mana pool is public
// information: every seat's pool is projected under every visibility (seat,
// public, omniscient) and for every viewer. The CR basis is that a pool is
// not one of the seven zones in 400.1 and holds no cards, so 400.2's
// hidden-zone framework (which names exactly library and hand) has no
// purchase on it; instead 106.4a, 106.4b, 117.3d and 118.3a all require a
// player to ANNOUNCE what is in their pool, an obligation that is incoherent
// for information meant to be hidden. The old pin "another seat's pool is
// nil" encoded the opposite divergence and is retired.
func TestCR106ManaPoolIsPublicForEveryPlayer(t *testing.T) {
	g := visibilityBoard(t)
	ch := flatChars{g}
	for _, vis := range []Visibility{Seat, Public, Omniscient} {
		for viewer := state.PlayerID(0); viewer < 4; viewer++ {
			v := ProjectFor(g, ch, viewer, vis, nil)
			for _, pv := range v.Players {
				if pv.Pool == nil {
					t.Fatalf("CR 106.4a/106.4b: visibility %s viewer %d sees no pool for seat %d; the pool is public", vis, viewer, pv.ID)
				}
				// Each seat floats 1+seat green, so a missing or wrong pool
				// cannot pass on a shared or zero value.
				if pv.Pool["G"] != int32(1+int(pv.ID)) {
					t.Fatalf("CR 106.4a/106.4b: visibility %s viewer %d reads seat %d pool = %d green, want %d", vis, viewer, pv.ID, pv.Pool["G"], 1+int(pv.ID))
				}
			}
		}
	}

	// The hand is a hidden zone (CR 400.2) and stays gated on the viewer's own
	// seat even now: the pool's publicity is not a side effect of "everything
	// is visible". This keeps the split pinned in the same proof.
	v := ProjectFor(g, ch, 0, Public, nil)
	for _, pv := range v.Players {
		if pv.Hand != nil {
			t.Fatalf("CR 400.2: public viewer reads seat %d's hand; only the pool, not the hand, is public", pv.ID)
		}
	}
}
