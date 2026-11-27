package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestValueLoweringSacrificedThisTurn: the three spellings the kernel keeps
// tallies for stay tallies (with an /Op suffix applied to the count, Bonecache
// Overseer's three per Food); every other spec is a filter, so Dargo's
// Artifact,Creature and Evendo's Permanent.!token widen the value with no
// State field. An empty spec and the other PlayerCount groups refuse.
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
		if err != nil || v.Kind != vSacrificedThisTurn || v.N != c.n || v.F != nil {
			t.Errorf("%q: got %s N=%d, %v; want SacrificedThisTurn N=%d, no filter", c.body, v, v.N, err, c.n)
		}
	}
	for _, body := range []string{
		"PlayerCountPropertyYou$SacrificedThisTurn Artifact,Creature",
		"PlayerCountPropertyYou$SacrificedThisTurn Permanent.!token",
		"PlayerCountPropertyYou$SacrificedThisTurn Creature",
		"PlayerCountPropertyYou$SacrificedThisTurn Artifact.YouCtrl",
	} {
		v, err := lc.svarValue(body, 0)
		if err != nil || v.Kind != vSacrificedThisTurn || v.F == nil {
			t.Errorf("%q: got %s F=%v, %v; want SacrificedThisTurn with a filter", body, v, v.F, err)
		}
	}
	for _, body := range []string{
		"PlayerCountPropertyYou$SacrificedThisTurn",
		"PlayerCountPropertyYou$SacrificedThisTurn ",
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
