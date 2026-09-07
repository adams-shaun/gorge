package botpolicy

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// abilityScore ranks ONE offered "ability" option (a non-mana activated
// ability, offered by rules/legal.go in an "ability" option whose Obj is the
// source permanent and whose Ability field is the index into its face's
// abilities). It is the one function chooseAbility reads, so the A-rules
// below are exactly this arithmetic, and a mutation that takes the first
// "ability" option, or treats every ability the same, changes it:
//
//   - A1 (no no-ops): an ability that provably changes nothing on the current
//     board is never activated. The one case the Board can see (it now
//     carries each permanent's AttachedTo) is a re-attach: an equipment
//     that is ALREADY attached to any permanent is declined, because an
//     already-attached permanent's activated ability is never worth
//     re-activating — an equip can only re-site the equipment, and whether
//     the move is a gain is a value read this policy cannot make (it
//     cannot read the granted abilities that make a different bearer
//     better) (equipNoOp). Where the policy cannot prove a no-op it may
//     still activate — this rule is exactly the set of changes the Board
//     can prove are nil, not a guess. A1 scoring as not-worth-taking is
//     what lets the priority policy fall through to its explicit pass and
//     end a turn it would otherwise loop on.
//   - A2 (free before costly): among abilities worth activating, the cheaper
//     one ranks higher. A free ability forgoes nothing, so it is always at
//     least as good a trial as one that spends mana. The cost is read from
//     the ability option's own display label (legalActions formats it
//     "<Name>: <SpellDescription>", and Forge spells an activated ability's
//     cost — "Equip 2", "3, Sacrifice …" — into that text), so this is a
//     general cost read, not an equipment special case; an ability whose
//     label carries no number is treated as costly (ranks below a
//     provably-free one).
//   - A3 (deterministic tie): two abilities of equal score tie on option
//     index, so the whole function (and the policy through it) stays a pure
//     function of (Board, Decision, rng consumption points). No wall clock
//     and never a map range; both adapters answering the same decision
//     derive the same pick.
//   - A4 (progress always available): when NO ability ranks as worth
//     taking, chooseAbility returns -1 and the priority policy's explicit
//     pass fallthrough takes over, ending the turn. That fallthrough is the
//     thing that closes a loop, so it is pinned by a named test.
//
// An "ability" option whose source is not a battlefield permanent in the
// Board (a graveyard card, a hand card) carries zero board facts — its
// AttachedTo reads 0, so A1 never fires on it and A2's cost read still
// ranks it; it is never a crash. Like the target and combat branches this
// consumes no rng: the pick is a pure function of the offered options and
// the board facts.
func (b Board) abilityScore(o decision.Option, me state.PlayerID) (score int32, worth bool) {
	if b.equipNoOp(o, me) {
		return 0, false // A1: a provable no-op is never activated.
	}
	// A2: cheaper ranks higher (only among worth-taking abilities; A1 above
	// already returned for the no-op case).
	return 1000 - abilityCost(o.Label), true
}

// equipNoOp is A1's provable no-op: does activating the "ability" option o
// leave the board exactly as it is? Every permanent's AttachedTo is on the
// Board (companion to Power/CMC/Basic), filled by both adapter halves from
// the same facts a real seat sees. An equip (an AB$ Attach targeting a
// creature the controller controls, which is what K:Equip expands to in
// cards/keywords.go) leaves the board unchanged in exactly two provable
// ways:
//
//   - already attached: the equipment currently sits on a permanent
//     (AttachedTo != 0), so its Equip can only re-site it. An
//     already-attached permanent's activated ability is never worth
//     re-activating: moving an Equipment to a DIFFERENT own creature is a
//     value read this policy cannot make (it cannot read the granted
//     abilities — haste, shroud — that make a different bearer better), so
//     the re-attach is never taken. The first attach, from AttachedTo == 0,
//     still happens normally.
//   - no legal target: the controller has no battlefield creature at all
//     (hasOwnCreature(me) is false), so an attach-ability cannot land
//     anywhere and any activation fizzles with AttachedTo unchanged — a
//     free equip with nothing to equip onto, which is how a wiped board
//     freezes a game on the very equip the re-attach rule caught while a
//     creature survived.
//
// equipNoOp reads the attachment state itself; it never PREDICTS the target
// an equip would choose, so it cannot drift out of step with the target
// branch (target.go) that actually answers the follow-up target decision.
// AttachedTo == 0 with at least one creature is NOT a no-op — the equip
// really sits the equipment somewhere. A1 therefore only ever fires where
// the activation provably reproduces the current attach state.
//
// Broadness note (an approximation, not a guess): "no own creature at all"
// is judged on the creature census alone, so an "ability" on a NON-attach
// source (a Rishadan Port's tap-a-land, a Karakas bounce) is also declined
// while the bot controls no creatures -- a minor tempo misplay the Board
// cannot distinguish from a no-target equip (it carries no per-ability
// target spec). It never hangs the game: declining an ability is a pass,
// and a pass advances the turn. This is A1's "where the Board cannot tell,
// err toward not churning" boundary, documented so the regression stays
// explicit rather than a guessed behaviour.
func (b Board) equipNoOp(o decision.Option, me state.PlayerID) bool {
	if !b.hasOwnCreature(me) {
		// No battlefield creature to attach to: an equip has no legal target
		// and fizzles unchanged.
		return true
	}
	return b.Cards[o.Obj].AttachedTo != 0
}

// hasOwnCreature is A1's no-legal-target census: does the deciding seat
// control at least one battlefield creature (the target test an Equip's
// "Creature.YouCtrl" ask needs)? It is ONLY the census — the threat-ranked
// "best own creature" projection the fix-round-1 A1 elected an equip
// target from is gone, because a projection is a second copy of the target
// heuristic and is not guaranteed to agree with the target branch that
// actually answers the follow-up decision. No ranking is produced and none
// is read, so a plain boolean is the honest shape (a map range here cannot
// leak order into a choice: the fold's result is the same for any order).
func (b Board) hasOwnCreature(me state.PlayerID) bool {
	for _, c := range b.Creatures {
		if c.Controller == me {
			return true
		}
	}
	return false
}

// abilityCost is A2's cost read for an "ability" option: the first number
// token in its display label, which is where the engine-facing cost lands
// (legalActions labels an ability "<Name>: <SpellDescription>", and Forge
// spells "Equip 2", "3, Sacrifice …" into SpellDescription). It is an
// approximation — the label is display text, not a parsed ManaCost — but it
// is the only cost the Board's offered options expose, it is deterministic
// (no map order, no clock), and it only orders two otherwise-worthy picks.
// An ability whose label carries no number reads as an arbitrarily costly
// ability (20), which ranks below any provably-free one without ever
// suppressing it (every worthy score stays above the -1 "nothing" sentinel,
// so an unnumbered ability is still worth taking).
func abilityCost(label string) int32 {
	var cur int32
	have := false
	for i := 0; i < len(label); i++ {
		c := label[i]
		if c >= '0' && c <= '9' {
			if have {
				cur = cur*10 + int32(c-'0')
			} else {
				cur = int32(c - '0')
				have = true
			}
		} else if have {
			return cur // the first digit run is the cost
		}
	}
	if have {
		return cur
	}
	return 20
}

// chooseAbility is the KPriority ability ranking: it picks ONE of the
// offered "ability" options, or returns -1 when none is worth taking. It
// replaces the position-first "take the first offered ability" that froze
// a whole game (see the package/cast.go docs and A4): every offered ability
// is scored by abilityScore (A1-A3) and the highest-scoring, lowest-index
// winner is chosen. -1 (nothing worth taking) is what lets the priority
// policy reach its explicit pass, which is what actually ends a turn the
// old code spent re-activating a no-op forever (A4). Consumes no rng.
func (b Board) chooseAbility(d *decision.Decision) int {
	best := -1
	var bestScore int32 = -1
	me := d.Player
	for _, o := range d.Options {
		if o.Kind != "ability" {
			continue
		}
		s, worth := b.abilityScore(o, me)
		if !worth {
			continue // A1: a provable no-op is never worth an activation.
		}
		// A3: ties break on the option's own Index (never map order).
		if best == -1 || s > bestScore || (s == bestScore && o.Index < best) {
			best, bestScore = o.Index, s
		}
	}
	return best
}
