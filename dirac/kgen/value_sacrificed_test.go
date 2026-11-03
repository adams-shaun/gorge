package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestValueLoweringSacrificedThisTurn:
// exactly the three spellings the kernel keeps tallies for lower, with an /Op
// suffix applied to the count (Bonecache Overseer's three per Food); any other
// spec refuses by name.
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
	v, err := lc.svarValue("PlayerCountPropertyYou$SacrificedThisTurn Food/Times.3", 0)
	if err != nil || v.Kind != vSacrificedThisTurn || v.N != 2 || v.Mul != 3 {
		t.Errorf("Food/Times.3: got %s N=%d Mul=%d, %v; want SacrificedThisTurn N=2 Mul=3", v, v.N, v.Mul, err)
	}
}
