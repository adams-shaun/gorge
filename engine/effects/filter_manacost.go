package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// maxManaCostWord is the largest N the ManaCost<N> family registers: the
// corpus carries ManaCost0 and ManaCost1 (Urza's Saga's Chapter III), and a
// printed cost past 15 does not exist.
const maxManaCostWord = 15

// ManaCost<N> is the card property "has a printed mana cost whose mana value
// is N" (Urza's Saga: "an artifact card with mana cost {0} or {1}"). Unlike
// cmcEQN it needs a mana cost at all (CR 202.1b: a land prints none, so
// Seat of the Synod has mana value 0 but no mana cost {0}); a printed "no
// cost" or empty cost therefore never matches. A face-down object has no
// mana cost (CR 708.2). The mana value is state.ManaValue, the one object
// read every tier shares.
func init() {
	for n := 0; n <= maxManaCostWord; n++ {
		want := n
		predicates["ManaCost"+strconv.Itoa(n)] = func(g *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool {
			return printsManaCost(o) && int(state.ManaValue(g, o)) == want
		}
	}
}

// printsManaCost reports whether o's current face has a mana cost: not face
// down, and a printed cost that is neither empty nor "no cost".
func printsManaCost(o *state.Object) bool {
	if o == nil || o.FaceDown || o.Face() == nil {
		return false
	}
	mc := strings.TrimSpace(o.Face().ManaCost)
	return mc != "" && !strings.EqualFold(mc, "no cost")
}
