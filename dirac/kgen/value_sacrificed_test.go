package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestValueLoweringSacrificedThisTurn:
// exactly the three spellings the kernel keeps tallies for lower; any other
// spec, and an /Op suffix on the head (Bonecache Overseer, held by
// design), refuses by name.
func TestValueLoweringSacrificedThisTurn(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}}
	for _, c := range []struct {
		body string
		n    int
	}{
		{"PlayerCountPropertyYou$SacrificedThisTurn Permanent", 0},
		{"PlayerCountPropertyYou$SacrificedThisTurn Artifact", 1},
		{"PlayerCountPropertyYou$SacrificedThisTurn Food", 2},
	} {
		v, err := lc.svarValue(c.body, 0)
		if err != nil || v.Kind != vSacrificedThisTurn || v.N != c.n {
			t.Errorf("%q: got %s N=%d, %v; want SacrificedThisTurn N=%d", c.body, v, v.N, err, c.n)
		}
	}
	for _, body := range []string{
		"PlayerCountPropertyYou$SacrificedThisTurn Food/Times.3", // Bonecache Overseer: the left-graveyard hold
		"PlayerCountPropertyYou$SacrificedThisTurn Permanent/Times.2",
		"PlayerCountPropertyYou$SacrificedThisTurn Clue",
		"PlayerCountPropertyYou$SacrificedThisTurn Permanent.!token",
		"PlayerCountPropertyYou$SacrificedThisTurn Creature",
		"PlayerCountPropertyYou$SacrificedThisTurn",
		"PlayerCountPropertyYou$SacrificedThisTurn ",
		"PlayerCountPropertyYou$SacrificedThisTurn Artifact.YouCtrl",
		"PlayerCountPlayers$SacrificedThisTurn Permanent",
		"PlayerCountOpponents$SacrificedThisTurn Permanent",
		"PlayerCount$SacrificedThisTurn Permanent",
	} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", body, v)
		}
	}
	// makes the suffix lowerable in general; Bonecache Overseer is a
	// held card, so its refusal must name that hold.
	if _, err := lc.svarValue("PlayerCountPropertyYou$SacrificedThisTurn Food/Times.3", 0); err == nil || !strings.Contains(err.Error(), "Bonecache Overseer's LeftGraveyardThisTurn gate") {
		t.Errorf("Bonecache Overseer suffix: got %v; want the named hold", err)
	}
}
