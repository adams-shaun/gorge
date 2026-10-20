package botpolicy

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// RemovalRules selects the A1 removal-now rule. The zero value is off, so
// every existing Board and both adapters keep their current cast decisions.
type RemovalRules struct {
	// HoldForBigger holds a one-shot Destroy, DealDamage or bounce spell
	// until the best opposing creature's threat clears removalThreatBar
	// (or the spell is lethal to an opponent's face, or could never aim at
	// a creature at all).
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
// when one of the spell's candidate targets is worth removing now:
//
//   - a player target that the spell's literal damage kills outright (the
//     lethal face is the whole point of holding burn: never sit on the
//     winning bolt); the spell must carry a literal positive NumDmg$ and a
//     scope that names players, and some opponent's life must be at or
//     below the damage. An unknown (dynamic) figure is no candidate at all;
//   - the best opposing creature the spell could legally aim at, whose
//     threat() reaches removalThreatBar. The fold reads the spell's
//     RemovalTargets scope, so a Shatter ("" scope, an artifact filter) and
//     a Stone Rain are never held on creature threat, and a creature that
//     is not a plausible legal target (hexproof, shroud, protection,
//     indestructible against a destroy) is no candidate either -- the bar
//     is about the best TARGET, not the biggest body.
//
// A false answer means "hold the spell and its mana for a bigger threat";
// chooseCast then skips the option exactly as C8 skips a dead counter, and
// the tap gate (removalHeld) refuses to tap toward it.
//
// The predicate reads only Board facts both adapters fill: Card.RemovalKind,
// Card.RemovalTargets and Card.RemovalDamage (the printed spell shape and
// scope), Card.Castable (the zone gate), Creatures and their threat(),
// and Life. A board with no candidate at all never holds: with nothing on
// the table worth the spell, the mana is better spent than parked.
func (b Board) removalNow(d *decision.Decision, o decision.Option) bool {
	if !b.Removal.HoldForBigger {
		return true
	}
	c := b.Cards.Get(o.Obj)
	if !c.Castable || !isOneShotRemoval(c.RemovalKind) {
		return true
	}
	if b.lethalFace(d.Player, c) {
		return true
	}
	best, ok := b.bestOpponentThreat(d.Player, c)
	return !ok || best >= b.faceScore()+removalThreatBar
}

// removalHeld is the tap gate's half of the A1 rule (finding: "hold the
// spell AND ITS MANA"): it reports whether the rule holds the cast of card
// id, so no tap may be aimed at making it payable. tapWants, bestUnpayable
// and AnyCastableNow all skip a held card -- otherwise the seat floats mana
// toward a spell chooseCast will refuse, and the pool empties at step end
// (CR 500.4) having bought nothing. With the rule off removalNow returns
// true before reading anything, so the gate is unchanged.
func (b Board) removalHeld(d *decision.Decision, id state.ObjID) bool {
	return !b.removalNow(d, decision.Option{Kind: "cast", Obj: id})
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

// lethalFace reports whether the removal spell c is lethal to some
// opponent's face right now: its kind is "damage", its scope names players,
// it carries a literal positive NumDmg$, and some OTHER player's life total
// is at or below the damage. The fold is a boolean OR over the Life table in
// its insertion order, so no iteration order reaches the answer.
func (b Board) lethalFace(me state.PlayerID, c Card) bool {
	if c.RemovalKind != "damage" || c.RemovalDamage <= 0 || !removalScopePlayers(c.RemovalTargets) {
		return false
	}
	for id, life := range b.Life.All() {
		if id != me && life <= c.RemovalDamage {
			return true
		}
	}
	return false
}

// removalScopePlayers reports whether a Card.RemovalTargets scope names
// players ("player" or "any").
func removalScopePlayers(scope string) bool {
	return scope == "any" || scope == "player"
}

// removalScopeCreatures reports whether a Card.RemovalTargets scope names
// creatures ("creature" or "any"). A spell with neither scope ("" for an
// artifact or land destruction filter) has no creature candidates, so the
// threat gate never holds it.
func removalScopeCreatures(scope string) bool {
	return scope == "any" || scope == "creature"
}

// bestOpponentThreat is the highest threat() among the opposing creatures
// the spell could plausibly legally aim at, and whether any exists. The
// scope filter first: a spell that cannot name a creature (RemovalTargets
// "" or "player") has no candidates at all. Then the per-creature
// targetability filter legalTargetOf. A max fold is order-independent, so
// no map iteration order reaches the answer.
func (b Board) bestOpponentThreat(me state.PlayerID, c Card) (int32, bool) {
	if !removalScopeCreatures(c.RemovalTargets) {
		return 0, false
	}
	best, ok := int32(0), false
	for _, cr := range b.Creatures.All() {
		if cr.Controller == me || !cr.legalTargetOf(c) {
			continue
		}
		if t := cr.threat(); !ok || t > best {
			best, ok = t, true
		}
	}
	return best, ok
}

// legalTargetOf reports whether this creature is a plausible legal target
// of the removal spell whose Card facts are c. The keyword filter mirrors
// what the engine's own targeting census excludes, at the grain the Board
// carries:
//
//   - Hexproof (the controller cannot be the target of spells an opponent
//     controls, and this bot always is one -- CR 702.11b) and Shroud (no
//     one may target it, CR 702.11a) are hard exclusions for every kind;
//   - Protection (any "Protection from <quality>" keyword, matched by
//     prefix) is excluded for every kind: protection prevents targeting
//     whenever the quality matches the spell, and the Board carries no
//     colour/protection-quality pairing to prove it does not -- the filter
//     errs toward over-excluding, which can only FIRE the removal a turn
//     earlier, never hold a spell that should have been spent (a held spell
//     wastes mana, a fired one at worst spends it);
//   - Indestructible (CR 702.12) is excluded for "destroy" only, where
//     destruction does nothing; damage and bounce are unaffected by it.
//
// Ward is deliberately NOT excluded: a ward is a cost gate, not a
// targeting illegality, and the spell remains a real option. This is an
// approximation at the Board's grain -- a full legality read is the
// engine's job, and the rule holds or fires a spell, it never names the
// target -- and the design note records it as such.
func (cr Creature) legalTargetOf(c Card) bool {
	if cr.hasKeyword("Hexproof") || cr.hasKeyword("Shroud") {
		return false
	}
	for _, k := range cr.Keywords {
		if head := cards.KeywordHead(k); len(head) >= 10 && strings.EqualFold(head[:10], "Protection") {
			return false
		}
	}
	if c.RemovalKind == "destroy" && cr.hasKeyword("Indestructible") {
		return false
	}
	return true
}
