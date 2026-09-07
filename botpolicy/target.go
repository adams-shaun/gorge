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
// opponent's face is worth on the PLAIN ranking path (rankOption), the one
// used when the KTarget decision carries no recognisable effect or no
// literal damage figure. It is a neutral constant, because on that path
// the policy cannot read the effect that produced the decision -- an
// absent target_effect, an unknown API, or a null/absent damage amount are
// all "the effect is unreadable", so it cannot tell a burn -- an effect
// that actually reduces a player's life -- from a draw or a removal whose
// effect never touches the face. With that fact absent, any bonus keyed on
// the opponent's life, their proximity to a lethal track's end, or their
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
// So on the plain path the face ranks neutrally, beneath every on-board
// creature threat, and is only chosen when no opposing creature is a legal
// target (a burn at a face, a drain, a life-gain that only targets players)
// or as a later pick in a multi-target decision. R2's board-over-face holds
// by construction: any creature ranks above it, so the face is never picked
// over a creature the policy cannot be sure the effect is better than.
// Ties among equal faces (the multiplayer case) break on option index (R7),
// the deterministic answer.
//
// When the effect IS readable (a known literal damage figure), the face is
// scored by effectRanker instead, and a face the damage reaches becomes the
// tier-2 lethal target -- the one case where the bot proves a burn. The
// unknown stays on the plain path, where the face is neutral.
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

// The three tiers of the ft1 face-aiming policy, as large score offsets so
// a tier strictly outranks everything in the tier below it (a creature's
// within-tier threat() figure is bounded far under the gap):
//
//   - tierThreat (1): an opponent creature that may kill the deciding seat
//     this round, and that the damage can actually kill -- defensive removal;
//   - tierFaceLethal (2): an opponent's face, when the damage is potentially
//     lethal to them -- the win line;
//   - tierValue (3): the most powerful opponent creature the damage can kill,
//     but only when the seat has spare mana.
//
// Anything below those ranks on the ordinary board-over-face scale.
const (
	tierThreat     int32 = 1_000_000
	tierFaceLethal int32 = 800_000
	tierValue      int32 = 600_000
)

// effectDamage returns the known literal damage a KTarget decision's effect
// deals, and whether it is a recognisable direct-damage primitive with a
// finite, nonnegative amount. An absent TargetEffect, an unknown API, or a
// null/absent Amount all report (0, false): the effect is unreadable and,
// per the ft1 contract, an unknown must never be treated as lethal. This is
// the one place an unknown is resolved to "no".
func (b Board) effectDamage(d *decision.Decision) (int32, bool) {
	te := d.TargetEffect
	if te == nil || te.Damage == nil || te.Damage.Amount == nil {
		return 0, false
	}
	return int32(*te.Damage.Amount), true
}

// mayKillMe reports whether an opposing creature c is a threat that may
// kill the deciding seat this round: it can attack (untapped, so combat
// damage could reach the seat this turn) and one swing would deal lethal
// combat damage to the seat's current life total (power no less than the
// seat's life). It reads only facts the bot holds, so a life total that is
// absent answers false -- "not a provable threat", never "assume lethal".
func (b Board) mayKillMe(me state.PlayerID, c Creature) bool {
	if c.Power <= 0 || c.Tapped {
		return false
	}
	life, ok := b.Life[me]
	if !ok {
		return false
	}
	return c.Power >= life
}

// hasSpareMana reports whether the deciding seat can afford to spend a
// removal spell on pure value (tier 3). The one sound, readable fact is
// unspent floating mana in the pool: if the pool already holds mana the
// cast did not spend, that mana is spare. (The brief's other half -- an
// untapped source beyond what the rest of the turn needs -- is not carried
// on the Board: the mana producers in b.Cards carry no Tapped field, so
// the policy cannot prove a source is untapped, and per the unknown-is-no
// rule it therefore never assumes one.)
func (b Board) hasSpareMana() bool {
	return b.Pool.Total() > 0
}

// effectRanker returns the KTarget ranking for a decision whose effect is a
// recognised direct-damage primitive with the known literal amount dmg. It
// orders options by the three ft1 tiers, most important first:
//
//   - tier 1: an opponent's creature that may kill the deciding seat this
//     round AND that dmg can actually kill (remaining toughness no more
//     than dmg) -- the defensive removal;
//   - tier 2: an opponent's face, when dmg is potentially lethal to them
//     (their life no more than dmg) -- the win line, the new capability;
//   - tier 3: the most powerful opponent creature dmg can kill -- but only
//     when the seat has spare mana; without it the removal is not spent on
//     value and these creatures fall back to the board-first ranking below.
//
// Everything that is not clearly one of those -- a face the known damage
// does not reach, a creature the damage cannot kill, a value kill with no
// spare mana -- keeps the ordinary board-over-face ranking (rankOption's
// threat()/neutral scale), so this is strictly more aggressive than the
// old neutral face but never less sound: an option the bot cannot prove
// lethal or a threat is never promoted, and an unknown never becomes
// lethal (the unknown stays on the plain path, whose face is neutral).
func (b Board) effectRanker(me state.PlayerID, dmg int32) func(decision.Option) targetRank {
	spare := b.hasSpareMana()
	return func(o decision.Option) targetRank {
		if o.Kind == "player" {
			if o.Player == me {
				return targetRank{score: 0, idx: o.Index, ours: true}
			}
			if b.Life[o.Player] <= dmg {
				return targetRank{score: tierFaceLethal, idx: o.Index}
			}
			// A face the known damage does not reach is not a good value
			// target; it ranks below every creature (board over face).
			return targetRank{score: 0, idx: o.Index}
		}
		if o.Player == me {
			sc := int32(0)
			if c, ok := b.Creatures[o.Obj]; ok {
				sc = c.threat()
			}
			return targetRank{score: sc, idx: o.Index, ours: true}
		}
		c, ok := b.Creatures[o.Obj]
		if !ok {
			return targetRank{score: 0, idx: o.Index}
		}
		if c.remTough() <= dmg {
			if b.mayKillMe(me, c) {
				return targetRank{score: tierThreat + c.threat(), idx: o.Index}
			}
			if spare {
				return targetRank{score: tierValue + c.threat(), idx: o.Index}
			}
		}
		// Not a provable tier-1/2/3 option: the ordinary board-first rank.
		return targetRank{score: c.threat(), idx: o.Index}
	}
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
//   - R2' (the ft1 face-aiming tiers, when the effect is readable): when
//     the decision carries a known literal damage figure (effectDamage),
//     options are ranked by the three tiers of effectRanker instead of the
//     plain scale: an opponent creature that may kill the seat this round
//     and that the damage kills (tierThreat), then an opponent's face the
//     damage reaches lethally (tierFaceLethal), then the most powerful
//     creature the damage kills when there is spare mana (tierValue), and
//     everything else falls to the board-first ranking below. An unknown
//     effect, API or amount keeps the plain scale, where the face is
//     neutral -- an unknown is never treated as lethal or as a threat.
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

	// The ranker: effect-aware (the three ft1 tiers) when the decision
	// carries a known literal damage figure, otherwise the plain
	// board-over-face ranking. A missing effect, an unknown API or a
	// null/absent amount all take the plain path -- the unknown-is-no rule
	// collapses into one branch, never assumed lethal per call site.
	var rank func(decision.Option) targetRank
	if dmg, ok := b.effectDamage(d); ok {
		rank = b.effectRanker(me, dmg)
	} else {
		rank = func(o decision.Option) targetRank { return b.rankOption(o, me) }
	}

	var own, foreign []targetRank
	for _, o := range d.Options {
		r := rank(o)
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
