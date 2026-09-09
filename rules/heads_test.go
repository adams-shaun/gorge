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
//
// dp2, the colour-aware tap gate, moves all four again. ONE cause, and this
// time the attribution is a bisect rather than a per-cause bench, because the
// branch carried a performance patch that had to be proven inert:
//
//	commit     what it is                        TestHeads
//	63eb3ab    main, before the branch           PASS -- matches the old goldens
//	12f27b3    the tap feature, merged           all four move, to the values below
//	f46ee97    + the load-time production cache  all four IDENTICAL to 12f27b3
//
// So the feature moved every head and the perf patch moved none. The
// mechanism predicts exactly that: chooseTap now prefers a source whose
// produced colour matches a coloured pip the pool still owes, so the ORDER in
// which permanents are tapped changes, which reorders the ManaAdd event stream
// in every game at every seat count -- while deriving that same production at
// load instead of per projection changes WHEN it is computed and never WHAT it
// is. Verified separately rather than assumed: the 12f27b3 and f46ee97 trees'
// -decision-stats output over 400 commander games is byte-identical (md5
// ad75c845c3002bf866f162ca975bbdc9).
//
// This is the regeneration Ruling FL-90 describes: an attributed move, proven
// against the parent tree, is not a spend against FL-83's budget. An
// unattributed one still is.
// dp1b, the policy determinism pass, moves all four. Attribution here is an
// ABLATION rather than a bisect: the branch changes four decision arms at once,
// so each was reverted to main's behaviour in turn and the heads re-measured.
//
//	arm reverted to main            resulting heads (2/4/6/8)
//	(none -- branch as-is)          b984373b 7d178f22 78c5c443 a5ec8770
//	KMulligan bottoming             8c79889e d212ee6d 223856cf 0c85486d  <- all four
//	KTriggerOrder -> Fisher-Yates   fa993f4e 7d178f22 756d4d9e 8074e8b2  <- 2/6/8
//	KTriggerOptional -> coin        b984373b 7d178f22 78c5c443 a3d8d825  <- 8 only
//	KChoose discard/exile/sacrifice b984373b 7d178f22 78c5c443 a5ec8770  <- NOTHING
//
// So the bottoming reroute is the universal driver, trigger-order and
// trigger-optional contribute at the seat counts whose games reach them, and
// the headline discard ranking -- the change the branch is named for -- does
// not touch the acceptance games at all. That last row is worth keeping: it
// says the head churn is paid for by arms other than the one being advertised.
//
// Ruling FL-90: an attributed regeneration is not a spend against FL-83.
// tg1, the stack-targeting fix, moves all four. Counterspells were INERT: a
// "target spell" ask searched the battlefield, so Mana Leak was offered every
// permanent, resolved, and effCounter no-opped on o.Zone != ZStack. They now
// find spells, and a spell is no longer offered as a target of itself
// (CR 115.5).
//
// Measured per seat count, `countered` MoveZone events and Counter resolves:
//
//	tree                      countered (2/4/6/8)   Counter resolves   self-target
//	587f31d before the line   0/1/2/5               0/2/1/3            0/0/0/0
//	e4bfbb9 round 1           0/4/3/8               0/2/1/3            0/0/1/3  <- self!
//	7f89c50 round 2           0/4/2/5               0/2/0/0            0/0/0/0
//
// Round 1 made counterspells work and simultaneously let three of them counter
// THEMSELVES; round 2's exclusion is why the 6/8 resolves fall back to zero —
// those casts were the self-targeting ones and now correctly fizzle.
//
// The 2-seat head moves too, which it did NOT against 587f31d, and the reason
// is a genuine interaction rather than noise: dp1b (0751802) changed the policy
// enough to put a counterspell into the 2-seat game, where it resolved and
// countered nothing (countered=0, the inert mistarget). tg1 makes that same
// cast bite. So the 2-seat move is dp1b and tg1 compounding, and neither alone
// produces it — the same shape as op6/op7 above, arrived at across two merges
// instead of one.
//
// Ruling FL-90: an attributed regeneration is not a spend against FL-83.
// dt1, delayed triggers, moves all four — and here a partial move was never
// available. `playAcceptance` assigns decks as all[i%12], so seat 0 holds
// death-n-taxes at EVERY seat count, and death-n-taxes runs Flickerwisp. The
// gate counted register/push/return-from-exile at 2/4/6/8 as 1/1/1, 1/1/1,
// 3/3/3, 1/1/1: the card fires everywhere, so every head must move.
//
// Before this, `effDelayedTrigger` emitted a Note and stopped. Flickerwisp
// exiled a permanent and never returned it. It now registers into
// state.Game.Delayed via an appended DelayedRegister event and, on entering the
// registered phase, fires through the ORDINARY trigger drain (APNAP, raising a
// real CR 603.3b trigger_order decision when several are waiting) as a
// DelayedPush minted ability.
//
// The append is the part that had to be right: two kinds were appended at
// ordinals 37/38 AFTER CmdDamage, with no existing kind reordered and no
// events.Event field added or retyped. The gate hashed the Append encoding of
// all 37 pre-existing kinds with populated fields on both trees and got
// identical SHA-256, so no historical log's bytes moved.
//
// Ruling FL-90: an attributed regeneration is not a spend against FL-83.
// dc1, the discard ask, moves ONE head — and the single-seat move is the whole
// attribution, traced to one card in one game.
//
// Only the 8-seat game changes. The gate instrumented it: Duress resolves once,
// Thoughtseize / Cabal Therapy / Faithless Looting never do, and the game's one
// ModeChosen belongs to a Charm, not the Duress. So Duress's ask is never even
// posed — its DiscardValid$ Card.nonCreature+nonLand filter finds nothing
// eligible in that hand — and the branch correctly discards NOTHING where the
// old deterministic stand-in took hand[0]. That one card declining to discard
// is the entire move.
//
// The B1/B2/B3 repairs move nothing: the gate measured `3cfab11 + 69cdced` and
// got this identical 8-seat head, so the mover is the RevealYouChoose feature
// itself, not the suspension fix that made it safe.
//
// Ruling FL-90: an attributed regeneration is not a spend against FL-83.
//
// uc1 (Counter's UnlessCost$) moves 2 and 4 and leaves 6 and 8 alone. Both
// movers are one card in one seat: Daze#95, dimir-tempo seat 1, which now poses
// the "pay {1} or it is countered" ask that never existed before. The two seat
// counts move for DIFFERENT reasons, and the difference is the point:
//
//	2 seats  LOG CHURN ONLY. ModeChosen at seq 1172 is followed immediately by
//	         obj 79 "countered" with no mana_add -- the payer had nothing
//	         floating, so the payment FAILED and the spell is countered exactly
//	         as before. The board is identical (countered 1 -> 1); the head
//	         moves because two new events are in the chain.
//	4 seats  A REAL BOARD DIVERGENCE. Same card, same seat, but here the
//	         payment SUCCEEDS: ModeChosen at seq 1907 is followed by
//	         mana_add(-1, "B") at 1908, and a spell that used to be countered
//	         now resolves.
//	6, 8     UNCHANGED, and this is the check that the attribution is honest:
//	         Daze never resolves at those seat counts, so no unless-pay ask is
//	         posed and the only ModeChosen in either game is Warping Wail's,
//	         byte-identical on both trees.
//
// A PARTIAL move is a stronger result than a full one: if all four had moved,
// nothing would distinguish "the feature fired" from "something unrelated
// perturbed every game". Two moving with named causes and two provably not
// moving is the shape that rules the second explanation out.
//
// The gate measured these on top of 63ee22d (the sc1 merge) and got values
// byte-identical to the measurement on e7fec05, so sc1's sacrifice work does
// not interact with these four games.
//
// REGENERATED at the ft1 merge (the bot may aim at a face, by Ruling FL-108's
// three tiers). ONE seat count moved -- 8, from 700f85d871d35367 -- and 2, 4 and
// 6 are byte-identical to the values above. The mover is ONE decision, found by
// instrumenting every effect-path KTarget in all four acceptance games and
// printing each divergence between the effect-aware pick and the effect-blind
// one:
//
//	8 seats  obj 407 Lightning Bolt (DealDamage 3), asked of p6 at 17 life.
//	         p2 is at 3 life, so 3 <= 3 is provably lethal and the bolt is now
//	         aimed at p2's FACE; the effect-blind policy pointed it at p5's
//	         permanent obj 348. This is TIER 2 (an opponent when the damage is
//	         potentially lethal). spare=false, so tier 3 played no part.
//	2, 4, 6  UNCHANGED, and that is the check that the attribution is honest:
//	         across those three games there is not a single divergent target
//	         decision. Tier 1 (a threat that may kill you this round) and tier 3
//	         (the best creature it can kill, given spare mana) fired ZERO times
//	         anywhere in the acceptance corpus.
//
// Ruling FL-107 governs this regeneration: there is no quota, ATTRIBUTION is the
// only requirement, and agents never write this map -- the orchestrator does, at
// the gate, from values the gate measured on branch+current-main. One seat
// moving with a named card and a named tier, and three provably not moving, is
// the strongest shape available.
//
// REGENERATED at the bl1 merge (the land drop picks the colour the hand needs).
// THREE seat counts moved -- 4, 6 and 8 -- and 2 did NOT. Causes were measured
// per seat count by dumping both event logs, locating the FIRST divergent event,
// and then instrumenting chooseLand to print the ranking inputs (unmet / cover /
// basic / flex) for every offered land at exactly that decision:
//
//	2 seats  UNMOVED, a95fd3b1da972438. The event log is byte-identical to
//	         main's at 1353 events, which is the check that the rest of this
//	         attribution is honest.
//	4 seats  ba76d1bb389a3c2b -> 9bd7c9ea7917f512. LEVEL 3, fewest distinct
//	         colours. At seq 1206 the seat holds Cavern of Souls (obj 134) and
//	         Ancient Tomb (obj 122) with no unmet coloured need, so coverage
//	         ties at 0 and neither is basic; Cavern's Any production reports
//	         five distinct colours against Ancient Tomb's zero, so Ancient Tomb
//	         is played where basic-first-then-lowest-index took Cavern.
//	6 seats  baafe0b87f436bec -> 56cad0755f3f8c78. Same level, same two cards,
//	         same objects, first divergence at seq 1995.
//	8 seats  dbe58b710d39e5ec -> 0964cc00aed0c02c. LEVEL 1, colour coverage. At
//	         seq 425 the unmet need is four blue and one black; Underground Sea
//	         (obj 61) covers 2 of it against a basic Island's (obj 65) 1, so
//	         coverage outranks basic-ness and the dual is played. This seat
//	         count now has TWO STACKED CAUSES: ft1's target ranking moved it
//	         from 700f85d871d35367 (above), and the land drop moves it again --
//	         and the land drop's divergence is the EARLIER of the two in the log.
//
// The branch's other change moves NOTHING: TestHeads passes on b4c04ea (the
// non-literal Amount$ honesty fix) alone with all four heads at their pre-branch
// values, so chooseLand owns the whole movement. The gate build-bisected that
// rather than assuming it, and it corrects the branch report's own claim that
// both commits contributed.
// REGENERATED again at the cn1 merge (a CARDNAME sacrifice cost resolves against
// the source object, CR 201.5). THREE seat counts moved -- 2, 4 and 8 -- and 6
// did NOT. One card owns every move: Polluted Delta (obj 70, seat 1,
// dimir-tempo), whose activation costs `T PayLife<1> Sac<1/CARDNAME>`. Before
// this merge the CARDNAME leg matched no object, so the cost was unpayable and
// the fetch was never offered; now it is. Causes were measured per seat count by
// dumping both full event logs and locating the FIRST divergent event:
//
//	2 seats  a95fd3b1da972438 -> 2af07c65b7cafec2. At seq 658 main has seat 1
//	         simply pass priority; the branch instead activates Polluted Delta --
//	         a "choose" decision at 659 for the library search, the payment at
//	         661, tap of obj 70 at 662, its move_zone "sacrificed" at 663 and
//	         ability_push at 664. The log grows 1353 -> 1371 events, and the +18
//	         is that activation and the land it fetches.
//	4 seats  9bd7c9ea7917f512 -> 2cf5359b632bdbd3. First divergence at seq 1817,
//	         where the same seat's priority answer moves from option [2] to [3]:
//	         the fetch activation is now in the offered list, so every later
//	         index shifts. 4387 -> 4413 events.
//	6 seats  UNMOVED, 56cad0755f3f8c78. Seat 1's Polluted Delta is not drawn
//	         into a position where the bot activates it in this game.
//	8 seats  0964cc00aed0c02c -> adb26a03b5daa734. First divergence at seq 8288,
//	         priority answer [0] -> [1], the same option-list shift. The event
//	         COUNT is unchanged at 15132: the game takes a different decision of
//	         the same length rather than a longer one.
//
// The 6-seat non-move is the check that the rest of this attribution is honest:
// the same code is live at every seat count, so a cause that claimed to be
// unconditional would have moved it too.
// REGENERATED again at the ce1 merge (DB$ Effect registers CantTarget and
// CantRegenerate as real continuous effects). ONLY the 8-seat head moves, and
// the cause is a SINGLE EVENT, measured by dumping both full logs on this exact
// base rather than carried over from the review:
//
//	2, 4, 6  UNMOVED.
//	8 seats  adb26a03b5daa734 -> 505822c760a5f85c. Both logs are 15132 events
//	         and exactly ONE differs. At seq 14414, immediately after seq
//	         14413's stack_resolve of Vines of Vastwood (obj 355, seat 0), main
//	         emits note "registers a continuous effect (STCantTarget) for
//	         Permanent" and this build emits clock_tick from AddContinuous's
//	         Timestamp == 0 branch. NO DECISION DIVERGED: every decision_made in
//	         the two logs is byte-identical, so the CantTarget restriction never
//	         actually withheld a target in that game. The head moves because a
//	         Note was replaced by a real registration, nothing more.
//
// The review measured this same move as e3c5d5c3a411e641 against a main two
// merges older. That value is stale and is NOT what is written here: cn1's
// Polluted Delta activation shifted the log underneath it, and the head was
// re-measured on branch+current-main, which is what FL-107 requires.
// fx2 (F13, CR 602.5a) regenerated all four. The summoning-sickness gate no
// longer withholds a noncreature's tap ability on the turn it entered, so a
// land cracked or activated a turn earlier than before. Measured per seat
// count at the merge gate, first divergent event named:
//
//   - 2 seats: Karakas activated at decision seq 1313 (was Pass), event 1315.
//   - 4 seats: Misty Rainforest cracked at decision seq 1886, event 1888.
//   - 6 seats: Karakas again, decision seq 7440, event 7442.
//   - 8 seats: Polluted Delta inserted at option 0 at event 5611. The bot
//     still PASSES here -- the head moves because DecisionMade hash-chains the
//     numeric choice index, so an option inserted ahead of Pass shifts the
//     recorded index. An offered-but-untaken option is only free of the chain
//     when it is appended LAST, which is why M2d-3's concede moved nothing.
//
// fx1 (F45, CR 103.8a/800.7) regenerated 4, 6 and 8. The starting player's
// first draw is no longer skipped in a multiplayer free-for-all, so every
// multiplayer acceptance game deals one more card on turn 1 and every hidden
// hand downstream of it differs.
//
// The 2-seat head is DELIBERATELY unmoved and is the falsifier for this
// change: CR 103.8a's skip is correct at two players, so the predicate is
// keyed to the constructed seat count. A 2-seat head that moved here would
// have meant the fix was wrong, not that the golden was stale.
//
// fx4 (F31, CR 704.5m/702.16c) moved NO head. The attachment SBA now also
// kills an Aura whose bearer gained protection from it, but no acceptance
// game at any seat count reaches that path -- measured, not assumed, by
// running TestHeads on wt/fx4 and getting all four goldens back unchanged.
//
// fx3 (F21, CR 608.2b) regenerated 8 only. legalTargets now applies the same
// continuous CantTarget predicate askTarget applies at offer time, so a
// restriction that arrives between targeting and resolution fizzles the
// spell instead of being ignored. Measured on wt/fx3 rebased onto fx4, so
// this value is the combined state of both round-2 merges:
//
//   - 2, 4, 6 seats: unmoved. Only the 8-seat game races a restriction
//     against a spell already on the stack.
//   - 8 seats: event 14737. Rancor (obj 359) fizzles for want of a legal
//     target instead of resolving and attaching to Monastery Swiftspear
//     (obj 388, controller 6) at event 14738. The restriction became active
//     after Rancor was cast, which is exactly the case askTarget cannot
//     catch and the resolution recheck now does.
//
// fx13 (F43, CR 802.4) regenerated 6 and 8 only. askBlockers built the
// defender census from AliveFrom(0) -- absolute seat order -- so in a game
// whose active player is not seat 0 the defenders declared blocks in the
// wrong order. It now censuses from AliveFrom(e.G.Active), which is APNAP
// turn order; the scope filters either side of it are unchanged, so the SET
// of defenders asked is identical and only the ORDER of the asks moves.
// Measured at the merge gate by dumping both complete acceptance logs and
// diffing them, not by trusting the head alone -- both logs have the same
// event COUNT at every seat count, which is the signature of a pure
// reordering:
//
//   - 2, 4 seats: unmoved (1726 and 5353 events, identical). These games
//     never reach a declare-blockers step whose active player is a seat
//     whose rotation differs from absolute order, so the two censuses agree.
//   - 6 seats: event 7817, both logs 9230 events. After active player 3
//     takes priority at 7816, the old log asks seat 0 for blocks and the new
//     one asks seat 5. Old 7818 is seat 0 declaring blockers:[0] and 7819 is
//     Vampire Lacerator (obj 209, controller 3) being blocked by Flickerwisp
//     (obj 37, controller 0); new 7818 is seat 5 declaring blockers:[] and
//     seat 0's ask slides to 7819. Same blocks, later.
//   - 8 seats: event 12250, both logs 15523 events. Same shape: active 3
//     takes priority at 12249, the old log asks seat 0 and the new one asks
//     seat 6, and seat 0's blockers:[0] against Vampire Lacerator -- blocked
//     by Palace Jailer (obj 48, controller 0) -- moves back one event.
//
// The equal event counts are load-bearing here. A change to WHICH defenders
// are asked, rather than in what order, would have changed the count, and
// that is the failure this fix had to avoid.
//
// fx11 (F33, CR 800.4a) regenerated ALL FOUR, and this is the one head move
// in the series that should be expected rather than checked for: every
// acceptance game ends with at least one player losing, and what happens to
// a departed player's cards is exactly what changed. A seat count that did
// NOT move would have been the surprising result.
//
// Departure used to move the departing player's BATTLEFIELD permanents to
// exile, which is CR 800.4a approximated by the nearest zone this build had.
// Objects now cease: every card-backed object that player OWNS leaves from
// whatever zone it is in, plus every stack object it controls, to ZCeased --
// a zone with no membership list, so a ceased object is in no zone at all
// rather than sitting in exile pretending to be gone. Three things follow,
// and all three show up in the logs:
//
//   - the sweep now touches library, hand and graveyard cards, not just the
//     battlefield, so it emits far MORE events than the exile sweep did;
//   - it walks the arena in dense index order, so the first card it touches
//     is the lowest-id owned object rather than the first battlefield one;
//   - ownership, not control, decides. An opponent-owned card the departing
//     player happened to control stays; the departing player's own card
//     under someone else's control goes.
//
// Measured at the merge gate on this branch rebased onto current main (so
// these values are the combined state of fx13 and fx11), first divergence
// per seat count. Every one of them is the same signature -- the old log
// exiling a battlefield land, the new log ceasing a lower-id library card:
//
//   - 2 seats: event 1718. Base exiled battlefield Island (obj 65); now
//     Underground Sea (obj 61) ceases from the LIBRARY.
//   - 4 seats: event 4497. Base exiled battlefield Underground Sea (obj 63);
//     now Underground Sea (obj 61) ceases from the library.
//   - 6 seats: event 5626. Base exiled battlefield Swamp (obj 69); now
//     Underground Sea (obj 61) ceases from the library.
//   - 8 seats: event 9275. Base exiled battlefield Island (obj 243); now
//     Island (obj 241) ceases from the library.
//
// The 6- and 8-seat values here are NOT the ones fx11 measured on its own
// branch (f95dceca1d442c5e and a53ae4074f80c2e3). Those were taken against a
// main that predates fx13's APNAP blocker order, which moves those same two
// heads. Re-measured on branch+current-main, which is what FL-107 requires,
// and the 2- and 4-seat values are identical either way because fx13 moved
// neither.
//
// fx15 (F42, CR 800.4e -- creatures attacking a departed player are removed
// from combat) moves the 2-seat head to 996edb2512cdab75 and moves NOTHING
// else. That asymmetry is the evidence, not a puzzle: the defect needs a
// player to leave the game while creatures are attacking THEM specifically,
// and in the 4-, 6- and 8-seat games the eliminations happen to fall
// outside a combat aimed at the departing seat. One golden moving is the
// narrow blast radius the fix was scoped to have.
//
// MEASURED on the merged main by dumping both complete 2-seat logs and
// diffing them, rather than by trusting the head alone:
//
//   - 1719  end_combat_reset obj=31  Thalia, Guardian of Thraben
//   - 1720  end_combat_reset obj=37  Flickerwisp
//   - 1721  end_combat_reset obj=48  Palace Jailer
//   - 1780  damage pl=1 amt=3
//   - 1781  damage pl=1 amt=2
//
// 1783 -> 1784 events; 309 intents, 13 turns and the winner are all
// unchanged. Note the net is +1, NOT +3: three of seat 0's attackers are
// removed from combat when seat 1 departs, and the two damage assignments
// seat 1 was still absorbing afterwards stop happening. A reviewer reading
// only the event delta would see a single event and conclude almost nothing
// changed; the delta is small because an addition and a removal nearly
// cancel, which is exactly the case where the count is the wrong thing to
// read. The removed damage is the defect: it was assigned to a seat that
// had already left the game.
// fx18 (F32, CR 511.3) and fx19 (F44, CR 800.6) move these together, and
// the values below are the COMBINED state measured on merged main -- not
// either branch's own numbers, neither of which survives the other's merge.
//
// fx18 moves all four. Combat is now reset as the end of combat step ENDS
// rather than as it begins, so every game loses one EndCombatReset per
// combat from the position where the step was entered. First divergence is
// the first turn's empty combat in every seat count (2: event 96, 4: 172,
// 6: 232, 8: 292): an EndCombatReset becomes a Priority. Event totals fall
// 1784->1771, 5514->5490, 9492->9466, 15403->15366. With EndCombatReset
// events and sequence numbers removed, the before/after streams are
// byte-for-byte identical, so nothing about card behaviour changed -- only
// the schedule of the reset.
//
// fx19 moves 4, 6 and 8 and DELIBERATELY NOT 2. freeMulligans is 0 below
// three seats, so the two-player path cannot be reached by this change; a
// moved 2-seat head would have meant the multiplayer rule leaked. The
// 2-seat head here is exactly the value fx18 measured alone, which is that
// check passing. In each moved game the dimir-tempo seat takes its one
// permitted mulligan and the now-free first mulligan removes the bottoming
// ask, its answer and one card move (first divergence at event 58, 78 and
// 98 respectively: a seat-1 bottoming DecisionOffered becomes TurnChange).
// fx21 (F35 + F36, CR 508.2 / 509.2 / 508.8) moves all four. Combat now
// contains a priority window in each declare step, after the declaration, so
// every combat in every acceptance game gains decisions and the passes that
// answer them. Measured at the gate on merged main; fx21's own branch values
// were identical, since nothing else merged between.
//
// First divergence is the first turn's empty combat at event 94/167/227/287
// for 2/4/6/8 seats: the base jumped StepDeclareAttackers -> StepEndCombat,
// the fix emits the forced empty DeclareAttackers marker and then the CR
// 508.2 priority decisions before end combat.
//
// Intents 309->349, 1227->1359, 1929->2144, 3398->3790. Events 1771->1928,
// 6283->6795, 9466->10318, 15912->17462. WINNERS AND TURN COUNTS ARE
// UNCHANGED at every seat count, and the coverage ratchet stays at 0 of 436 --
// which is the evidence that this is more decisions in the same games, not
// different games. A head move this large with a changed winner would have
// meant a card primitive moved and would not have been mergeable on this
// reasoning.
var acceptanceHeads = map[int]string{
	// jj-cmb moved all four: combat gained a CR 510.4 priority round INSIDE the
	// combat damage step between the first-strike and regular passes (the round
	// that used to sit at the next step boundary is gone, which is why the
	// intent counts are unchanged at 348/1335/2149/3808), and the log no longer
	// emits a phantom `step` event after `game_over` -- measured by diffing the
	// 2-seat event stream against main, event by event, before regenerating.
	2: "490516cb66bcc061",
	4: "00577379e705ad1e",
	6: "1331f514efb06839",
	8: "89a83d74c174495c",
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
