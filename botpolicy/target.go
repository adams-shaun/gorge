package botpolicy

import (
	"sort"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// threat is the policy's ranking of an opposing creature as the thing a
// targeting decision should hit. It deliberately answers "which of the
// opponent's creatures is most worth removing" rather than pitting a
// creature against the opponent's face -- that comparison is the calling
// code's business (faceScore below) -- and it reads only facts already on
// botpolicy.Creature, so neither adapter adds a Board field for it.
//
// The metric prices three things beyond power:
//
//   - persistence: remTough (Toughness minus accumulated damage) is counted,
//     so a healthy creature ranks above an identical one that is one hit
//     away from dying on its own -- removing the creature that will stick
//     around is worth more than one that dies anyway.
//   - availability: an untapped creature can block our incoming attack, so
//     it ranks above a tapped one that has already swung this turn and
//     cannot (CR 509.1).
//   - evasion: a keyword that makes the creature harder to answer with
//     ordinary damage or blocks -- Flying (hard to block, dodges ground
//     removal), Deathtouch (kills anything it trades into), First
//     Strike/Initiative (combat sentences), Vigilance (attacks and still
//     blocks), Reach (keeps the skies honest) -- each adds value a bare
//     power+toughness proxy would miss.
//
// Pure power would order a dying 4/1 above a woken 3/3; threat flips them,
// which is the whole reason the policy does not rank by Power alone.
func (c Creature) threat() int32 {
	v := c.Power*3 + c.remTough()*2
	if !c.Tapped {
		v += 6
	}
	if c.hasKeyword("Flying") {
		v += 16
	}
	if c.hasKeyword("Deathtouch") {
		v += 12
	}
	if c.hasKeyword("First Strike") || c.hasKeyword("Initiative") {
		v += 8
	}
	if c.hasKeyword("Vigilance") {
		v += 6
	}
	if c.hasKeyword("Reach") {
		v += 4
	}
	return v
}

// faceScore is what pointing a targetable-at-player effect at the
// opponent's face is worth, and it is deliberately a neutral constant.
// The policy cannot read the effect that produced the KTarget decision
// (askTarget emits only the source and the legal target identities, and
// Board.Cards carries no stack objects, so the resolving spell is
// unrecoverable), so it cannot tell a burn -- an effect that actually
// reduces a player's life -- from a draw or a removal whose effect never
// touches the face. With that fact absent, any bonus keyed on the
// opponent's life, their proximity to a lethal track's end, or their
// biggest creature would be a kill the policy cannot prove:
//
//   - a player at 2 life is no more a win than one at 18 when the effect
//     may deal no damage at all;
//   - "near" the 21 commander-damage clock is not burn reach: ordinary
//     non-combat spell damage never advances that clock (CR 903.10 counts
//     only the combat damage one commander has dealt), so it must not
//     attract face targeting;
//   - keying the score on the target's biggest creature can prefer the
//     healthier player in a multiplayer game.
//
// So the face ranks neutrally, beneath every on-board creature threat, and
// is only chosen when no opposing creature is a legal target (a burn at a
// face, a drain, a life-gain that only targets players) or as a later pick
// in a multi-target decision. R2's board-over-face holds by construction:
// any creature ranks above it, so the face is never picked over a creature
// the policy cannot be sure the effect is better than. Ties among equal
// faces (the multiplayer case) break on option index (R7), the
// deterministic answer.
func (b Board) faceScore() int32 {
	return faceNeutral
}

// faceNeutral is the standing value of the opponent's face as a target:
// neutral, and below every on-board creature threat.
const faceNeutral int32 = 0

// targetRank is one option's standing for chooseTargets: a score, the
// option's index (the deterministic tiebreak), and whether it belongs to
// the deciding seat.
type targetRank struct {
	score int32
	idx   int
	ours  bool
}

// rankOption scores one KTarget option for seat me. `ours` separates "a
// permanent/player of my own" from everything else, which is the only fact
// a targeting decision's owner-rule needs. An opponent's player option is
// worth faceScore -- a neutral constant (see faceScore); an opponent's
// permanent is worth its on-board threat when it is a battlefield creature
// in the census, or 0 when its Obj is in no zone this Board can read (a
// battlefield artifact/enchantment a removal could target, or a
// Graveyard/Hand/Exile object) -- never a crash, just a low rank.
func (b Board) rankOption(o decision.Option, me state.PlayerID) targetRank {
	if o.Kind == "player" {
		if o.Player == me {
			return targetRank{score: 0, idx: o.Index, ours: true}
		}
		return targetRank{score: b.faceScore(), idx: o.Index}
	}
	if o.Player == me {
		// My own permanent (or a "player" option naming me): a target a
		// removal-shaped effect should not be pointed at, deferred to the
		// every-option-is-mine fallback below -- but scored by the same
		// threat so a forced all-own decision still ranks its own
		// permanents rather than falling back to position order.
		sc := int32(0)
		if c, ok := b.Creatures[o.Obj]; ok {
			sc = c.threat()
		}
		return targetRank{score: sc, idx: o.Index, ours: true}
	}
	if c, ok := b.Creatures[o.Obj]; ok {
		return targetRank{score: c.threat(), idx: o.Index}
	}
	return targetRank{score: 0, idx: o.Index}
}

// chooseTargets is the KTarget policy. It answers the one question the old
// policy got wrong twice -- "which legal target" -- under these rules:
//
//   - R1 (never your own while an opponent exists): an option belonging to
//     the deciding seat is only ever chosen when every other option is also
//     theirs -- a self-target decision (a pump, an aura, a recursion that
//     offers only the seat's own board), which cannot help but point at
//     themselves. When any opposing option exists, the seat's own
//     permanents are never touched.
//   - R2 (board over face): the opponent's best battlefield creature is
//     always preferred over the opponent's face when one is a legal target.
//     The policy cannot read the effect that produced the decision, so it
//     cannot prove it reaches the player at all -- a burn and a draw read
//     identically -- which makes a face target a claim the policy cannot
//     support. So the face ranks neutrally (faceScore is a constant) below
//     every on-board creature threat, and a creature offered alongside it
//     is always the pick. A player target is chosen only when no opposing
//     creature is a legal target (a burn at a face, a drain, a life-gain
//     that only targets players), or as a later pick in a multi-target
//     decision.
//     R7 (deterministic neutral tie): equal faces -- the multiplayer case,
//     or a face beside a zero-fact option -- break on option index, so no
//     map iteration order reaches the answer.
//   - R3 (rank better than pt): opposing creatures rank by threat(), which
//     prices remTough, Tapped and evasion keywords on top of power, not by
//     a bare power or power+toughness figure.
//   - R4 (unreadable options): an option whose Obj is in no zone this Board
//     reads is a 0-score low rank, never a panic; it is only chosen when
//     every other option in its decision also ranks at or below it (the
//     all-off-board graveyard pick, say).
//   - R5 (honour Min/Max): exactly d.Min targets are picked when the
//     decision demands that many, up to d.Max; a may/up-to decision
//     (Min 0) fires its full legal width up to d.Max, so multi-target
//     removal does not idle half-efforts. The count is produced here, and
//     never left for clamp: clamp's top-up fills with lowest-index
//     options -- position order, the very thing this policy replaces -- so
//     an under-filled choice would reintroduce the positional bug by
//     another door.
//
// Like the combat branches, it consumes no rng: the pick is a pure function
// of the offered options and the board facts, and every tie breaks on
// option index, so no map iteration order reaches the answer.
func (b Board) chooseTargets(d *decision.Decision) []int {
	n := len(d.Options)
	if n == 0 {
		return nil
	}
	me := d.Player

	// R5: how many targets to produce, decided here independently of clamp.
	pick := d.Min
	if pick < 1 {
		// A may/up-to decision (Min 0) fires its full legal width: up to N
		// removal needs both targets to do its job.
		pick = d.Max
		if d.Max < 1 {
			pick = 0
		}
	}
	if d.Max >= 0 && pick > d.Max {
		pick = d.Max
	}
	if pick > n {
		pick = n
	}

	var own, foreign []targetRank
	for _, o := range d.Options {
		r := b.rankOption(o, me)
		if r.ours {
			own = append(own, r)
		} else {
			foreign = append(foreign, r)
		}
	}
	byScore := func(s []targetRank) {
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].score != s[j].score {
				return s[i].score > s[j].score
			}
			return s[i].idx < s[j].idx
		})
	}

	// R1: lead with the opposing options; only top up with our own when a
	// decision is all-ours or does not offer enough of theirs to meet Min
	// (totality is preserved either way).
	choices := make([]int, 0, pick)
	byScore(foreign)
	for i := 0; i < len(foreign) && len(choices) < pick; i++ {
		choices = append(choices, foreign[i].idx)
	}
	if len(choices) < pick {
		byScore(own)
		for i := 0; i < len(own) && len(choices) < pick; i++ {
			choices = append(choices, own[i].idx)
		}
	}
	return choices
}
