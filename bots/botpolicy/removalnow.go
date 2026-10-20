package botpolicy

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// RemovalRules selects the A1 removal-now rule. The zero value is off, so
// every existing Board and both adapters keep their current cast decisions.
type RemovalRules struct {
	// HoldForBigger holds a one-shot Destroy, DealDamage or bounce spell
	// until the best opposing creature's threat clears removalThreatBar.
	HoldForBigger bool
}

// Enabled reports whether the A1 rule is armed.
func (r RemovalRules) Enabled() bool { return r.HoldForBigger }

// removalThreatBar is the threat a target must reach before the hold rule
// spends removal now. On threat()'s scale (power*3 + toughness*2 + 6 when
// untapped) a 3/3 is 21 and a 4/4 is 26, so the bar holds against a 3/3 and
// fires at a 4/4 (or at a smaller evasive creature whose keywords clear it).
// The face is the zero point: faceScore is neutral on the plain path, so
// this is an absolute board-threat figure rather than a comparison with face
// damage.
const removalThreatBar int32 = 24

// removalNow reports whether the cast option o should fire now. It is true
// when the rule is off, when o is not a castable one-shot removal shape, or
// when the best opposing creature's threat reaches removalThreatBar. A false
// answer means "hold the spell and its mana for a bigger threat"; chooseCast
// then skips the option exactly as C8 skips a dead counter.
//
// The predicate reads only Board facts both adapters fill: Card.RemovalKind
// (the printed spell shape), Card.Castable (the zone gate), Creatures and
// their threat(). A board with no opposing creature never holds: the spell
// may still have a legal player target (burn to the face), and this rule is
// not a targeting rule.
func (b Board) removalNow(d *decision.Decision, o decision.Option) bool {
	if !b.Removal.HoldForBigger {
		return true
	}
	c := b.Cards.Get(o.Obj)
	if !c.Castable || !isOneShotRemoval(c.RemovalKind) {
		return true
	}
	best, ok := b.bestOpponentThreat(d.Player)
	return !ok || best >= b.faceScore()+removalThreatBar
}

// isOneShotRemoval is the set of CastRemovalKind values the A1 rule holds.
// Sacrifice, exile, library and graveyard moves are outside this ticket.
func isOneShotRemoval(kind string) bool {
	switch kind {
	case "destroy", "damage", "bounce":
		return true
	}
	return false
}

// bestOpponentThreat is the highest threat() among creatures not controlled
// by me, and whether any exists. A max fold is order-independent, so no map
// iteration order reaches the answer.
func (b Board) bestOpponentThreat(me state.PlayerID) (int32, bool) {
	best, ok := int32(0), false
	for _, c := range b.Creatures.All() {
		if c.Controller == me {
			continue
		}
		if t := c.threat(); !ok || t > best {
			best, ok = t, true
		}
	}
	return best, ok
}
