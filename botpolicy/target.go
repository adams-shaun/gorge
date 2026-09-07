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

// faceScore is what pointing a targetable-at-player effect at the opponent's
// face is worth. It is not a constant: a flat 0 made the face a last
// resort, chosen only when the opponent had no targetable creature at all,
// so a Lightning Bolt with the opponent at 2 life went to their 1/1 instead
// of winning the game. Two facts price it, both already on the Board:
//
//   - bestThreat, the largest threat among the opponent's offered creatures
//     that the same decision could remove instead. The face's standing
//     value is a hair below it, so R2's board-over-face preference holds
//     while the face is merely legal -- a removal answers the board rather
//     than throwing damage at a face that can take it.
//   - closeness to the end of either lethal track: life running out (the
//     opponent loses at 0) or commander damage nearing 21 (CR 903.10, the
//     second clock). The policy cannot read the spell's damage, so it prices
//     the face by the opponent's proximity to dead, not by the damage the
//     spell would deal: a player at 2 life is a face worth the win, a
//     player at 18 is not.
//
// The lethal term is a steep bonus gated on the opponent being within
// reach -- never on the face being merely legal -- so a healthy board is
// not ignored: away from lethal the face sits under the best creature and
// the removal answers the board (the "do not suicide into the board"
// guard). players and off-battlefield objects never share a target decision
// the way a burned-out face would need it to, so no tie to a board creature
// is ever left to position order here.
const (
	// faceStand is how much the face's standing value sits below the best
	// creature the same effect could remove, so R2 holds while the face is
	// merely legal.
	faceStand int32 = 1
	// faceLethalNear is how close to the end of a lethal track the opponent
	// must be (life-like points remaining) for the face to become the win.
	faceLethalNear int32 = 3
	// faceLethalStep is how much the face's value climbs per point nearer a
	// track's end once within reach, steep enough that a face in reach
	// outranks any creature the board can offer.
	faceLethalStep int32 = 16
)

func (b Board) faceScore(opponent state.PlayerID, bestThreat int32) int32 {
	life := b.Life[opponent]
	// CR 903.10: the greatest combat damage one commander has dealt this
	// player -- the second, life-independent lethal track.
	var cmdMax int32
	for _, cm := range b.Commanders {
		if d := cm.Damage[opponent]; d > cmdMax {
			cmdMax = d
		}
	}
	// Standing: just under the best creature the effect would otherwise
	// answer, so a merely-legal face loses to the board.
	s := bestThreat - faceStand
	if s < 0 {
		s = 0
	}
	// The opponent is nearest dead on whichever track has the least
	// life-like headroom: life points left, or (21 - cmd) before the
	// commander clock closes.
	headroom := life
	if cmd := int32(21) - cmdMax; cmd < headroom {
		headroom = cmd
	}
	if headroom <= faceLethalNear {
		s += (faceLethalNear - headroom + 1) * faceLethalStep
	}
	return s
}

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
// worth faceScore (priced against that player's best offered creature threat
// and their proximity to a lethal track's end -- see faceScore); an
// opponent's permanent is worth its on-board threat when it is a battlefield
// creature in the census, or 0 when its Obj is in no zone this Board can
// read (a battlefield artifact/enchantment a removal could target, or a
// Graveyard/Hand/Exile object) -- never a crash, just a low rank.
func (b Board) rankOption(o decision.Option, me state.PlayerID, bestThreat map[state.PlayerID]int32) targetRank {
	if o.Kind == "player" {
		if o.Player == me {
			return targetRank{score: 0, idx: o.Index, ours: true}
		}
		return targetRank{score: b.faceScore(o.Player, bestThreat[o.Player]), idx: o.Index}
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
//     preferred over the opponent's face whenever the opponent has one and
//     the face is merely legal -- that is, whenever the opponent is not
//     within reach of a lethal track's end. The policy cannot read the
//     effect, so its proxy for "this is a stop-the-board removal" is that
//     the opponent has a battlefield creature offered at all; away from
//     lethal that creature's threat outranks the face (faceScore is a hair
//     below the best such threat). When the opponent has no battlefield
//     creature worth an option, or is close enough to dead that damage to
//     the face is the win, the face is the pick.
//     R6 (the face is the win): the face outranks the board only when the
//     opponent is within reach of a lethal track's end -- life running
//     out, or commander damage nearing 21 (CR 903.10) -- and never because
//     the face is merely a legal target. The boost scales with how near
//     the end the opponent is, so a player at 1 life prices the face above
//     a player at 4, and a healthy 18-life player's face stays below their
//     best creature: the bot answers the board instead of ignoring it.
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
	// bestThreat: the largest threat among each opposing player's offered
	// battlefield creatures -- what a face hit against THAT player would
	// otherwise trade with. It is computed off the offered options, not the
	// whole board, because only the options this decision allows are legal
	// targets the removal could actually take instead.
	bestThreat := make(map[state.PlayerID]int32)
	for _, o := range d.Options {
		if c, ok := b.Creatures[o.Obj]; ok && c.Controller != me {
			if t := c.threat(); t > bestThreat[c.Controller] {
				bestThreat[c.Controller] = t
			}
		}
	}
	for _, o := range d.Options {
		r := b.rankOption(o, me, bestThreat)
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
