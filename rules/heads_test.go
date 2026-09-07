package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// acceptanceHeads pins the chain head of the deterministic acceptance game
// at each seat count (R-14). A change here is a change to what the 12 repo
// decks do; the commit that makes it must name the card behaviour that
// moved it. The seed, bot and deck assignment are TestRepoDecksPlayAtEverySeatCount's
// own (rules/acceptance_test.go's playAcceptance), so the two tests always agree.
// M2d moved all four heads, and owns exactly ONE regeneration -- this one,
// at the milestone merge gate, not once per sub-task. Build-bisected causes
// (FL-76), measured per seat count rather than assumed:
//
//   - M2d-1, the London mulligan (R-M1): moves ALL FOUR. playAcceptance now
//     sets Mulligans: 1, so every acceptance game runs a keep/mulligan round
//     and, for any seat that mulligans, a re-shuffle, a fresh seven and a
//     bottoming round -- all of it emitted, so every head shifts.
//     Intermediate heads after M2d-1 alone: 2 45e0671d07b60d9e,
//     4 dcee545be139ca21, 6 642a239a24d3a1fe, 8 acf4ad4bafda267f.
//
//   - M2d-2, KModes and the mid-resolution ask (R-8): moves 4, 6 and 8 only.
//     It emits the new ModeChosen kind, and counting that kind in each
//     acceptance log gives 2 seats: 0, and 4/6/8 seats: 1 apiece. So the
//     2-seat golden below is M2d-1's value, unchanged by M2d-2.
//
//     An earlier version of this comment explained the 2-seat zero by saying
//     dimir-tempo's Spell Pierce carries UnlessCost$ 2 and is "served through
//     this same kind", just never cast under this seed. That is wrong, and
//     the correction matters more than the head does: effCounter
//     (effects/misc.go) never reads UnlessCost$ at all. Spell Pierce, Mana
//     Leak, Daze and Mausoleum Wanderer counter unconditionally, so casting
//     one would emit no ModeChosen either. M2d-2 closed R-8 for
//     CopySpellAbility only -- see AGENTS.md's Known approximations. The
//     per-seat COUNT above was measured and stands; only the explanation of
//     the zero was wrong.
//
//   - M2d-3, concede (R-M3): moves NOTHING. It only adds an option to every
//     priority decision, and neither bot mirror ever picks it; an offered
//     but untaken option is not an event.
//
// M3's OWN regeneration (Ruling FL-83 allows exactly one per milestone,
// and m34 is that one). The previous golden set (2 45e0671d07b60d9e,
// 4 04950e3969039a7b, 6 d70bc7e30c0fccdd, 8 496784e7fbcf37be) is from the
// B2 era; everything M3 merged on top was verified UNMOVED at every merge
// (m30, m31, m32, m33, m35, m37), so the whole gap between those goldens
// and the pre-m34 computed heads is B3 and B4, and m34 supplies the third
// cause (see the commit body for the regression's full naming). The four
// pre-m34 computed heads, verified at 11fc153 before m34's own change:
//
//	2 1d5dc4e2727cf2d8, 4 1c581d6dfe425410, 6 09345bb7f2a73286,
//	8 1f5644eaaa0fbe40.
//
//	- B3 (e13a3ee / merge 60b8981): the botpolicy targeting heuristic --
//	  never own, board over face, rank by threat, honour Min/Max. Spells
//	  and abilities now pick different targets, so different permanents
//	  die and different life totals remain. Moves ALL FOUR seats.
//	- B4 (d2d4a6e / merge 36be378): cast and play_land ranking in the
//	  priority policy -- which spells are cast and in what order, which
//	  lands are played, hence different boards from the first main phase
//	  on. Moves ALL FOUR seats (including the 2-seat head, which had been
//	  fixed since M2d-1: the 2-seat game now casts differently, so
//	  45e0671d07b60d9e finally moves to 1d5dc4e2727cf2d8).
//	- m34 (fcea152): the free-for-all defender choice (CR 506.2 / CR
//	  903.14). Every KAttackers option is now one (attacker, defender)
//	  pair, so the recorded attacker intents -- and therefore the
//	  DeclareAttackers events, combat damage, blocks and deaths -- change
//	  shape from the first combat on, and an attack can split across two
//	  defenders (one combat in the 4-seat game emits two DeclareAttackers
//	  events, 14 total instead of 13). Also asks one KBlockers decision
//	  per defender instead of one for the whole declaration. Measured, 4
//	  seats: 8 of the game's 13 attacker choices target a defender the
//	  fixed-defender build could not express. Moves 4, 6 and 8 ONLY: the
//	  2-seat game still sees one option per creature at its sole defender,
//	  so m34's own chain contribution is a blank at 2 seats and the golden
//	  below stays 1d5dc4e2727cf2d8, unchanged from the pre-m34 measured
//	  value.
//
// Task D1's OWN single regeneration (the task is this milestone's ONE golden
// regeneration, FL-83; attributed to the commit as a whole per FL-76). D1
// implements CR 514.1 -- the cleanup step discards the active player down to
// seven cards (a new KChoose "discard" decision). Measured per seat count
// (counted hand->grave MoveZone events in each acceptance log plus the moved
// heads themselves), the cleanup discard fires ONCE in the 2-seat game and
// ONCE in the 6-seat game, and never in the 4-seat or 8-seat games -- those
// games end before any player's hand ever exceeds seven (fewer turns per
// seat; the lone hand->grave move in the 8-seat log is a card-driven discard,
// pre-existing and unchanged). So only the 2-seat (1d5dc4e2727cf2d8 ->
// 630573758e2019a8) and 6-seat (9207c51fd1e4ed6a -> 2563530049855042) heads
// move; 4 and 8 stay byte-identical to the golden. This contravenes the
// brief's expectation that every acceptance trajectory changes, and the
// measurement is reported as the honest cause.
//
// The opponents milestone's OWN single regeneration (FL-83; attributed per
// FL-76). ONE cause, and it moves ONE seat count: botpolicy now declines an
// Equip whose equipment is already attached to something, instead of
// re-activating it forever. The no-op detector reads the attachment state
// (Card.AttachedTo != 0) rather than predicting which creature the target
// branch will elect -- the previous attempt re-derived the target with the
// policy's own threat ranking, the two rankings disagreed on the stuck board
// (obj=41 attachedTo=9 bestOwn=8), and the loop survived.
//
// Measured by instrumented replay, not inferred: the 6-seat game's
// death-n-taxes pilot reaches 21 priority decisions offering an Equip on an
// already-attached Sword of Fire and Ice, and the new rule declines every
// one where the position-first block re-activated it. The 2-, 4- and 8-seat
// games reach ZERO attached-equip decisions, so their heads cannot move and
// do not: replaying this build with the old position-first block restores
// all four goldens byte-for-byte. Only the 6-seat head moves
// (2563530049855042 -> 7a043ce3e84f6f39).
//
// This is the second recorded case of a botpolicy change moving a strict
// subset of the heads, and the subset is exactly the seat counts whose games
// reach the changed code path -- the same shape D1 found above, arrived at
// from the opposite direction.
//
// The opponents milestone's SECOND regeneration -- an explicit exception to
// FL-83's one-per-milestone, taken deliberately because the alternative was
// worse: op6 and op7 were both gated and both move heads, so landing them
// separately would have cost TWO further regenerations instead of this one.
// Recorded as an exception rather than dressed up as a milestone boundary.
//
// TWO causes, and the seat counts separate them cleanly:
//
//   - op6, the tap-for-mana gate: the policy now taps a permanent for mana
//     only while some castable card exists whose cost the pool cannot yet
//     pay, instead of tapping whichever permanent was offered first. That
//     changes the ManaAdd event stream in every game, so it moves ALL FOUR
//     heads.
//   - op7, the attack-target tiebreak: equal-tier attack options now prefer
//     the lowest-life defender instead of the lowest seat index. A two-player
//     game offers one defender and therefore has no tie to break, so this
//     cause CANNOT move the 2-seat head, and does not.
//
// Measured, not inferred -- each cause was benched alone before stacking:
//
//	seats   op6 alone           op7 alone           both (this commit)
//	2       0876361619998e2a    unchanged           0876361619998e2a
//	4       355ae7ceb40e9ac9    c4d8a9de11cca37d    d74b8a889f09be48
//	6       9940f5be8b29e4d6    f8d7e167ab30c173    ea3d87a74c4c954d
//	8       25ce1c7603a1bd0f    224fd2dda5a59ff4    5e573c76021a419f
//
// The 2-seat column is the attribution proof: the combined head is
// BYTE-IDENTICAL to op6's, so op7 contributes nothing there, exactly as its
// mechanism predicts. On 4/6/8 both causes compound and neither alone
// reproduces the combined head.
var acceptanceHeads = map[int]string{
	2: "0876361619998e2a",
	4: "d74b8a889f09be48",
	6: "ea3d87a74c4c954d",
	8: "5e573c76021a419f",
}

func TestHeads(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	for _, seats := range []int{2, 4, 6, 8} {
		got := acceptanceHead(t, reg, seats)
		if want := acceptanceHeads[seats]; got != want {
			t.Errorf("%d seats: chain head %s, golden %s — if this move is intended, update acceptanceHeads and name the cause in the commit body", seats, got, want)
		}
	}
}
