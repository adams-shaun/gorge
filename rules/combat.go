// Task 21: combat. CR 508-510 in miniature -- declare attackers, declare
// blockers, then damage -- plus the CR 514.2 cleanup that combat depends on
// and that nothing in this codebase implemented before now. The lethal-
// damage and Deathtouch state-based actions this file's own damage marking
// depends on were also added here by Task 21, as destroyLethalDamage, but
// moved to sba.go by Task 22 -- they are general state-based-action logic,
// not combat-specific, and now live alongside the rest of CR 704.
//
// M1's own simplifications, matching the brief this task was built from:
//   - Only the active player attacks. The defending player is a real, per-
//     attacker choice since Task m34: each attacking creature may be declared
//     against ANY one living opponent, independently (CR 506.2 / CR 903.14),
//     and the KAttackers decision offers one option per (attacker, defender)
//     pair -- decision.Option.Player carries the pair's defender, so widening
//     M1's single-fixed-defender simplification into the true choice was
//     additive, exactly as this comment always promised. The engine rejects
//     an intent that declares one creature against two defenders
//     (validateAttackers), and with a single opponent the pair list is one
//     option per attacker at that one defender, so a two-player game
//     observes exactly the M1 surface.
//   - No priority is offered mid-combat: declare attackers, declare blockers
//     and combat damage each resolve in one automatic step, same as every
//     other engine-only step (untap, cleanup).
//   - A blocking creature is not prevented from being declared against more
//     than one attacker (CR 509's one-attacker-per-blocker default, absent
//     Menace-like exceptions this build doesn't model) -- not reachable from
//     any legal option offered here, since askBlockers's own set of block
//     options for one blocker only ever repeats the same attacker within one
//     combat, but a hand-built decision.Intent could still name the same
//     blocker in two separate options.
package rules

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// canAttack reports whether id may be declared as an attacker (CR 508.1a):
// a creature under the active player's control, untapped, and either not
// summoning sick or hasty.
func (e *Engine) canAttack(id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != e.G.Active {
		return false
	}
	f := o.Face()
	if f == nil || !f.IsCreature() {
		return false
	}
	if o.Tapped {
		return false
	}
	if o.SummonSick && !e.HasKeyword(id, "Haste") {
		return false
	}
	return true
}

// canBlock reports whether blocker may be declared against attacker (CR
// 509.1a): an untapped creature controlled by the defending player, gated by
// Flying/Reach (CR 702.9b) and by any CantBlock/CantBlockBy static
// (blockRestricted, statics.go).
func (e *Engine) canBlock(blocker, attacker state.ObjID) bool {
	b, a := e.G.Obj(blocker), e.G.Obj(attacker)
	if b == nil || a == nil || !a.IsAttacking {
		return false
	}
	if b.Zone != state.ZBattlefield || a.Zone != state.ZBattlefield {
		return false
	}
	bf := b.Face()
	if bf == nil || !bf.IsCreature() {
		return false
	}
	if b.Tapped || b.Controller != a.Attacking {
		return false
	}
	// CR 509.1a / 702.16j: a creature that the attacker is protected from
	// cannot block it.
	if e.protectedFrom(attacker, blocker) {
		return false
	}
	if e.HasKeyword(attacker, "Flying") && !e.HasKeyword(blocker, "Flying") && !e.HasKeyword(blocker, "Reach") {
		return false
	}
	if e.blockRestricted(blocker, attacker) {
		return false
	}
	return true
}

// askAttackers builds a KAttackers decision, one option per (attacker,
// defender) pair: every creature passing canAttack, offered once against
// every living opponent of the active player (CR 506.2: each attacking
// creature's controller announces which opponent it is attacking, one
// independent choice per creature). Task m34 replaces M1's single fixed
// defender (the next living seat) with this real choice; each option's
// Player field carries its own defender, and handleAttackers groups the
// chosen options back into one DeclareAttackers event per defender. Options
// are ordered defender-major: for each living opponent in ascending seat
// order, for each legal attacker in battlefield order -- deterministic, and
// the same order the declare-blockers step asks those defenders in.
//
// A creature may be declared attacking at most one opponent (CR 506.2), but
// the pair list offers it once per defender: an intent that names the same
// creature against two defenders is REJECTED by the engine's
// validateAttackers guard, never resolved by the engine picking one defender
// for a client that could not decide.
//
// With no possible attacker, there is nothing this decision could change (an
// empty answer is the only legal one, since Max would be 0), and no legal
// attacker also means the rest of combat -- declare blockers, combat damage
// -- has nothing to do either (CR 508.1's "if no creatures are declared as
// attackers... skip the declare blockers and combat damage steps"), so this
// skips straight to end of combat rather than surfacing a decision no
// possible answer to which matters. TestTurnsRotateThroughEverySeat (turn_
// test.go, predating this task) depends on exactly this: a table of Mountains
// with no creatures ever on it must sail through combat as a sequence of
// ordinary priority rounds, not a non-priority decision passAll doesn't know
// how to answer.
func (e *Engine) askAttackers() {
	p := e.G.Active
	var attackers []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if e.canAttack(id) {
			attackers = append(attackers, id)
		}
	}
	if len(attackers) == 0 {
		e.setStep(state.StepEndCombat)
		return
	}
	var defenders []state.PlayerID
	for _, q := range e.G.AliveFrom(0) {
		if q != p {
			defenders = append(defenders, q)
		}
	}
	var opts []decision.Option
	for _, d := range defenders {
		for _, id := range attackers {
			opts = append(opts, decision.Option{Index: len(opts), Kind: "attacker",
				Label: "Attack with " + e.G.Obj(id).Face().Name + " at " + e.G.Players[d].Name,
				Obj:   id, Player: d})
		}
	}
	e.ask(&decision.Decision{Player: p, Kind: decision.KAttackers, Min: 0, Max: len(opts),
		Prompt: fmt.Sprintf("turn %d — declare attackers", e.G.Turn), Options: opts})
}

// handleAttackers records the chosen attackers (CR 508.1c: this is what
// causes triggered abilities matching "Attacks" to fire, via checkTriggers
// running behind every emit) and taps each one unless it has Vigilance (CR
// 508.1f, 702.20b).
//
// Each chosen option carries its own defender (the KAttackers option list is
// one entry per (attacker, defender) pair, Task m34), so the declaration is
// grouped by defender into one DeclareAttackers event per defending player,
// emitted in ascending seat order -- deterministic, and the same order the
// declare-blockers step asks the defenders in. The chosen set is guaranteed
// to hold distinct attackers (validateAttackers ran before this handler), so
// each creature lands in exactly one group. Within a defender, the event's
// IDs keep the order the client submitted them (what an Attacks trigger's
// Remembered reads, in trigger_match.go).
func (e *Engine) handleAttackers(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	if len(chosen) == 0 {
		e.advanceStep()
		return
	}
	var defenders []state.PlayerID
	byDef := make(map[state.PlayerID][]state.ObjID, len(chosen))
	for _, opt := range chosen {
		if _, ok := byDef[opt.Player]; !ok {
			defenders = append(defenders, opt.Player)
		}
		byDef[opt.Player] = append(byDef[opt.Player], opt.Obj)
	}
	sort.Slice(defenders, func(i, j int) bool { return defenders[i] < defenders[j] })
	for _, d := range defenders {
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: d, IDs: byDef[d]})
	}
	for _, opt := range chosen {
		if !e.HasKeyword(opt.Obj, "Vigilance") {
			e.emit(events.Event{Kind: events.Tap, Obj: opt.Obj})
		}
	}
	e.advanceStep()
}

// validateAttackers is the KAttackers legality guard behind Option A's
// cross-product option list (Task m34). One creature may be declared
// attacking at most one opponent (CR 506.2, CR 903.14), but the option list
// offers every (attacker, defender) pair, so an intent naming the same
// creature twice -- against two different defenders -- passes decision.
// Decision.Validate's per-index checks (two distinct, in-range options)
// while declaring one creature attacking two players. Rejecting here, before
// the intent is recorded and the decision consumed, is the enforcement
// boundary: the legality of an attacker set is a property of the DECLARATION
// as a whole, not of any single option, so it cannot live in Validate's
// option-shape checks, and it must not be trusted to a client to avoid (a
// rules-ignorant client only knows it may pick any subset of the pairs it
// was offered). With a single opponent the pair list has exactly one option
// per creature, so this guard is inert in two-player games.
func validateAttackers(d *decision.Decision, in decision.Intent) error {
	seen := make(map[state.ObjID]bool, len(in.Choices))
	for _, o := range d.Chosen(in) {
		if seen[o.Obj] {
			return fmt.Errorf("attacker %d declared against more than one defender", o.Obj)
		}
		seen[o.Obj] = true
	}
	return nil
}

// blockerRound is the declare-blockers step's plain-value cursor, the same
// one-decision-at-a-time pattern as the London mulligan round (rules/
// mulligan.go): Task m34 lets one attack split across several defending
// players (CR 506.2), and each defender declares its own blocks (CR
// 509.1c). order lists the defenders that have at least one attacking
// creature, in ascending seat order -- the same order handleAttackers
// emitted their DeclareAttackers events in; cursor is the next defender to
// ask. askBlockers builds the list on the step's first entry, asks one
// defender per call, and hands the step to combat damage once every
// defender has declared. Plain data (a slice plus an index), never a
// closure, so Engine.Clone copies it like the mulligan round.
//
// zero value: order == nil means "not yet built", the step's first-entry
// state; an empty-but-built round (order with len 0) means "nothing to
// block", which askBlockers resolves by moving straight to combat damage.
type blockerRound struct {
	order  []state.PlayerID
	cursor int
}

// askBlockers runs the declare-blockers step one defending player at a
// time. The first entry to the step builds the defender list; each call
// then asks the next defender with at least one legal block option, and the
// last one exhausted moves the step to combat damage. A defender with
// attackers but zero legal options for them is skipped without a decision:
// its only legal answer would be "block with nothing", and asking would
// change nothing (the same skip askAttackers applies to its own no-option
// case). Between two defenders' answers the Advance loop stays on
// StepDeclareBlockers (handleBlockers only advances the round cursor), so a
// split attack on two opponents produces two KBlockers decisions, one per
// defender, in seat order.
//
// With no attackers this combat (declared 0, or all already gone), there is
// nothing to block: the round is empty and the step skips straight to
// combat damage. With attackers but zero legal blockers for them (every
// candidate fails canBlock, e.g. a lone ground creature against a flier),
// the same reasoning skips each such defender.
func (e *Engine) askBlockers() {
	if e.blockerRound.order == nil {
		order := make([]state.PlayerID, 0, len(e.G.Players))
		for _, q := range e.G.AliveFrom(0) {
			if q == e.G.Active || len(e.blockAttackers(q)) == 0 {
				continue
			}
			order = append(order, q)
		}
		e.blockerRound = blockerRound{order: order}
	}
	br := &e.blockerRound
	for br.cursor < len(br.order) {
		defender := br.order[br.cursor]
		var opts []decision.Option
		for _, bid := range e.G.Zone(state.ZBattlefield, defender) {
			for _, aid := range e.blockAttackers(defender) {
				if !e.canBlock(bid, aid) {
					continue
				}
				opts = append(opts, decision.Option{Index: len(opts), Kind: "block",
					Label: e.G.Obj(bid).Face().Name + " blocks " + e.G.Obj(aid).Face().Name,
					Obj:   bid, Attacker: aid, Player: defender})
			}
		}
		if len(opts) == 0 {
			br.cursor++
			continue
		}
		e.ask(&decision.Decision{Player: defender, Kind: decision.KBlockers, Min: 0, Max: len(opts),
			Prompt: fmt.Sprintf("turn %d — declare blockers", e.G.Turn), Options: opts})
		return
	}
	e.blockerRound = blockerRound{}
	e.setStep(state.StepCombatDamage)
}

// blockAttackers lists the creatures currently declared attacking defender
// (CR 509.1): every battlefield object under the active player's control
// marked IsAttacking with Attacking == defender and a Face().
//
// Ruling T21-c (Task 21 fix round 1): the census additionally requires
// Face() != nil. DeclareAttackers's own events.Apply case sets IsAttacking
// on any existing object with no such check (Player is validated, but
// nothing about the object it names), so a malformed or tampered event -- or
// a nil-Card object such as an ability's own stack object (Ruling F3) --
// reaching IsAttacking used to make it as far as the label build in the
// single-defender askBlockers, which read e.G.Obj(aid).Face().Name
// unconditionally: a nil-pointer panic, and therefore a remote kill of the
// whole match (one goroutine runs it). canAttack already requires this for a
// real attacker, so no legitimate attacker is excluded by requiring it here
// too.
func (e *Engine) blockAttackers(defender state.PlayerID) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		o := e.G.Obj(id)
		if o == nil || !o.IsAttacking || o.Face() == nil || o.Attacking != defender {
			continue
		}
		out = append(out, id)
	}
	return out
}

// handleBlockers records the chosen (attacker, blocker) pairs in one
// DeclareBlockers event, in the order the client submitted them -- that order
// is what BlockedBy preserves (events.Apply's DeclareBlockers case is a plain
// append per pair) and so what dealCombatDamage's damage-assignment loop
// below reads as "blocker order" for CR 510.1c's ordered damage assignment.
//
// Task m34: this advances only the blockers-round cursor, never the step.
// Each defending player declares its own blocks (CR 509.1c -- a split attack
// can involve several), and the Advance loop re-enters askBlockers for the
// next defender, which is what decides when the step moves to combat damage.
func (e *Engine) handleBlockers(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	if len(chosen) > 0 {
		pairs := make([][2]state.ObjID, 0, len(chosen))
		for _, opt := range chosen {
			pairs = append(pairs, [2]state.ObjID{opt.Attacker, opt.Obj})
		}
		e.emit(events.Event{Kind: events.DeclareBlockers, Pairs: pairs})
	}
	e.blockerRound.cursor++
}

// dealCombatDamage runs first-strike damage and then regular damage. Damage
// within a step is simultaneous: every amount is computed against pre-step
// state before any event is emitted, so two creatures that would kill each
// other both die.
func (e *Engine) dealCombatDamage() {
	if e.anyFirstStrike() {
		e.damageStep(true)
		e.checkStateBased()
	}
	e.damageStep(false)
}

// liveBlockers filters a's BlockedBy to blockers still actually on the
// battlefield: one may have left play (destroyed by a trick, sacrificed) in
// the gap between blocks being declared and damage being dealt.
func (e *Engine) liveBlockers(a *state.Object) []state.ObjID {
	var out []state.ObjID
	for _, bid := range a.BlockedBy {
		if b := e.G.Obj(bid); b != nil && b.Zone == state.ZBattlefield {
			out = append(out, bid)
		}
	}
	return out
}

// anyFirstStrike reports whether any attacker or its live blockers has First
// Strike or Double Strike, which is what decides whether dealCombatDamage
// runs a separate first-strike step at all (CR 510.5): with none, only the
// single regular damage step happens. Double Strike is not among the eight
// keywords this task registers as implemented (see the init below) -- a card
// that actually has it is not routed into real decks by the coverage gate --
// but the check costs nothing to leave in exactly as the brief specified it,
// so a Double-Strike creature that reaches combat some other way (a test, a
// future task) still behaves correctly rather than merely "not being asked
// about".
func (e *Engine) anyFirstStrike() bool {
	for _, id := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		a := e.G.Obj(id)
		if a == nil || !a.IsAttacking {
			continue
		}
		if e.HasKeyword(id, "First Strike") || e.HasKeyword(id, "Double Strike") {
			return true
		}
		for _, bid := range e.liveBlockers(a) {
			if e.HasKeyword(bid, "First Strike") || e.HasKeyword(bid, "Double Strike") {
				return true
			}
		}
	}
	return false
}

// assignment is one pending damage event, computed against pre-step state so
// a whole damage step applies simultaneously (CR 510.2/510.4).
type assignment struct {
	toPlayer   state.PlayerID
	toObj      state.ObjID
	amount     int32
	lifelink   state.PlayerID
	hasLink    bool
	deathtouch bool
	// from is the creature dealing this assignment (the attacker for its own
	// assignments, each blocker for its hit-back), kept so the damage emit
	// loop can set e.damaging (engine.go) and let protection prevent damage
	// from a protected source (CR 702.16d).
	from state.ObjID
}

// actsThisDamageStep reports whether id deals damage during this pass of
// dealCombatDamage (CR 510.5): Double Strike acts in both the first-strike
// and the regular step; First Strike (without Double Strike) acts only in
// the first-strike step; everything else acts only in the regular step.
func (e *Engine) actsThisDamageStep(id state.ObjID, firstStrike bool) bool {
	if e.HasKeyword(id, "Double Strike") {
		return true
	}
	if e.HasKeyword(id, "First Strike") {
		return firstStrike
	}
	return !firstStrike
}

// tallyCmdDamage records that the commander object from dealt amount combat
// damage to the player p (CR 903.10), by emitting the CmdDamage event that
// events.Apply folds into that player's cumulative commander-damage tally
// (state.Player.CmdDamage). It must only be called from the combat damage
// step, for a Player-targeted assignment whose damage actually landed (not
// prevented or replaced), in a Commander-format game -- the caller in
// damageStep arms all three, and the Apply case derives the commander's
// match-wide dense index (the same slot m30's genesis sizes and New's
// Commanders bookkeeping names) from g.Players[].Commanders, which is what
// keeps a commander keyed to the same slot for the whole match.
//
// The tally is carried in an event of its own rather than written directly:
// the existing Damage event does not record which commander the source was
// (events.Event's fields are append-only, so its field set is frozen), so a
// reconstruction starting from the log alone cannot re-derive the per-
// commander tally from the Damage events it already has. Recording the tally
// in a CmdDamage event -- appended after every earlier Kind, so no ordinal,
// hash chain or golden replay is affected -- makes that same log-only replay
// fold the tally back exactly, which is the deciding question the brief poses
// (and answers "carried in an event of its own"). It also keeps every state
// mutation on the events.Apply path, the build's standing invariant.
func (e *Engine) tallyCmdDamage(p state.PlayerID, from state.ObjID, amount int32) {
	e.emit(events.Event{Kind: events.CmdDamage, Player: p, Obj: from, Amount: amount})
}

// damageStep computes and then applies one round of combat damage --
// first-strike creatures only, or everyone else, per firstStrike. Every
// Power/Toughness/HasKeyword read above the emit loop happens before any
// Damage event this step produces is applied, which is what makes two
// creatures that would each kill the other both actually die instead of the
// first one's death sparing the second.
//
// Ruling T21-a (Task 21 fix round 1): a first-strike attacker's own forward
// damage (to its blockers or the defending player) is gated on
// actsThisDamageStep, exactly as before, but the "blockers hit back" section
// below is not -- it used to sit inside the same gate, so a first-strike
// attacker skipped its own regular-step turn (correctly) but that same
// `continue` also skipped ever collecting a surviving, non-first-strike
// blocker's regular-step damage back at it. Each blocker's own hit-back
// entry is now independently gated on that blocker's own
// actsThisDamageStep, which is the only thing CR 510.4 actually conditions
// it on.
func (e *Engine) damageStep(firstStrike bool) {
	var as []assignment
	for _, aid := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		a := e.G.Obj(aid)
		if !a.IsAttacking || a.Zone != state.ZBattlefield {
			continue
		}
		blockers := e.liveBlockers(a)

		if e.actsThisDamageStep(aid, firstStrike) {
			if pw := e.Power(aid); pw > 0 {
				link := e.HasKeyword(aid, "Lifelink")
				dt := e.HasKeyword(aid, "Deathtouch")
				trample := e.HasKeyword(aid, "Trample")
				switch {
				case len(a.BlockedBy) == 0:
					// Genuinely unblocked: full damage to the defending player.
					as = append(as, assignment{toPlayer: a.Attacking, amount: pw,
						lifelink: a.Controller, hasLink: link, from: aid})

				case len(blockers) == 0:
					// Ruling T21-d (CR 509.1h): a creature that was blocked
					// stays blocked for the rest of combat even if every
					// creature blocking it has since left -- it deals no
					// combat damage at all, unless Trample lets the whole
					// amount push through to the player instead (there is no
					// blocker left to owe any of it to).
					if trample {
						as = append(as, assignment{toPlayer: a.Attacking, amount: pw,
							lifelink: a.Controller, hasLink: link, from: aid})
					}

				default:
					// Ruling T21-b (CR 510.1c): lethal damage to each
					// blocker, in declaration order, before any spills to
					// the next. Which blocker(s) receive more than lethal
					// when Trample is absent and power exceeds every
					// blocker's combined toughness is really the attacking
					// player's choice (CR 510.1a); turning that into a real
					// decision is new scope this fix does not take on, so
					// the deterministic approximation is: every blocker
					// except the last is capped at its own need, and the
					// last absorbs whatever remains (Trample instead caps
					// every blocker, spilling any true excess to the
					// defending player below).
					remaining := pw
					for i, bid := range blockers {
						need := e.Toughness(bid)
						if dt {
							need = 1
						}
						give := remaining
						if (trample || i < len(blockers)-1) && give > need {
							give = need
						}
						as = append(as, assignment{toObj: bid, amount: give,
							lifelink: a.Controller, hasLink: link, deathtouch: dt, from: aid})
						remaining -= give
						if remaining <= 0 {
							break
						}
					}
					if remaining > 0 && trample {
						as = append(as, assignment{toPlayer: a.Attacking, amount: remaining,
							lifelink: a.Controller, hasLink: link, from: aid})
					}
				}
			}
		}

		// Blockers hit back -- independent of whether the attacker itself
		// acted this step above (Ruling T21-a).
		for _, bid := range blockers {
			if !e.actsThisDamageStep(bid, firstStrike) {
				continue
			}
			if bp := e.Power(bid); bp > 0 {
				as = append(as, assignment{toObj: aid, amount: bp,
					lifelink: e.G.Obj(bid).Controller, hasLink: e.HasKeyword(bid, "Lifelink"),
					deathtouch: e.HasKeyword(bid, "Deathtouch"), from: bid})
			}
		}
	}
	for _, x := range as {
		// e.damaging names the dealing creature for the whole of this
		// assignment so emit's protection check (Task 15) can prevent the
		// damage when the recipient is protected from it (CR 702.16d); reset
		// before the next assignment.
		e.damaging = x.from
		var prevented bool
		if x.toObj != 0 {
			// Task 15 fix round 1 (Critical C1): the return value of the
			// Damage emit is read here. emit swallows a protected permanent's
			// damage and returns a Note instead of a Damage event, but the
			// FOLLOW-ON emits that ride the damage -- the deathtouch marker and
			// the lifelink life gain -- used to run regardless, so a 2/2 blue
			// Merfolk with Lifelink + Deathtouch blocked by a creature with
			// protection from blue would both slap a Deathtouched counter on
			// it (lethal under CR 704.5g) and gain its controller life (CR
			// 702.15a: lifelink triggers only on damage actually dealt) even
			// though the damage itself was prevented. Checking the emitted
			// event's Kind -- the recipient-armed bet here is that a Note (or
			// any replacement-substituted non-Damage kind) means the damage
			// did NOT land -- skips both riders for a prevented assignment.
			ev := e.emit(events.Event{Kind: events.Damage, Obj: x.toObj, Amount: x.amount})
			prevented = ev.Kind != events.Damage
			if x.deathtouch && !prevented {
				e.emit(events.Event{Kind: events.CounterChange, Obj: x.toObj,
					Counter: "Deathtouched", Amount: 1})
			}
		} else {
			// Combat damage to a player. The existing (Task 15) protection
			// prevention path only armed the Obj branch; the player branch
			// never read the emit's return value. In a Commander-format game
			// (CR 903.10, Task m33) this branch must know whether the damage
			// actually landed before it tallies commander damage -- so it
			// reads the returned kind exactly the way the Obj branch already
			// does, and only tallies when it is still a Damage event (a
			// replacement-substituted Note means prevented/replaced damage,
			// which must not add to the tally). Reusing `prevented` here also
			// makes the shared lifelink rider below skip a prevented
			// player-hit, consistent with the Obj branch. Non-Commander games
			// take the original single-emit path untouched, so this task
			// changes nothing about them.
			if e.format == FormatCommander {
				ev := e.emit(events.Event{Kind: events.Damage, Player: x.toPlayer, Amount: x.amount})
				prevented = ev.Kind != events.Damage
				if !prevented {
					e.tallyCmdDamage(x.toPlayer, x.from, x.amount)
				}
			} else {
				e.emit(events.Event{Kind: events.Damage, Player: x.toPlayer, Amount: x.amount})
			}
		}
		if x.hasLink && !prevented {
			e.emit(events.Event{Kind: events.LifeChange, Player: x.lifelink, Amount: x.amount})
		}
		e.damaging = 0
	}
}

// maxHandSize is CR 514.1: at the beginning of a player's cleanup step, if
// their hand contains more than this many cards, they discard cards from it
// until it contains exactly this many (normally seven). Task D1 made the
// discard a real decision and verified that no effect in the corpus modifies
// the maximum hand size today -- a grep for maximum-hand-size text across
// cards/ and effects/ found nothing that sets or reads it (see the task
// report) -- so it is a plain package constant, not a game field an effect
// can reach. A card that one day DOES modify it is a separate finding and
// must not silently change this constant.
const maxHandSize = 7

// cleanupStep is the CR 514 cleanup step's turn-based actions, Task D1
// adding CR 514.1 on top of the CR 514.2 body Task 21 owns. Ordering:
// CR 514.1 (discard down to maxHandSize) runs first, then the 514.2 body --
// the two are simultaneous under the rules (nothing about the discard
// affects what the 514.2 body clears and vice versa), so the engine picks
// this order and the discard is offered before any permanent cleanup is
// emitted, keeping the transcript's discard lines ahead of the damage-/
// effect-clear lines it is a turn-based action of the same step. Called from
// turn.go's priorityRound.
//
// When the active player's hand exceeds maxHandSize, this ASKS a KChoose
// "discard" decision -- exactly one option per card in hand, in hand-zone
// order (never built from a map), with Min == Max == the number to discard --
// and returns with the decision pending, suspending the cleanup step until
// the answer arrives (discardCleanup). With a hand of maxHandSize or fewer
// there is nothing to discard and no decision is asked at all -- a zero-option
// or zero-count decision must never reach a seat (brief decision 5). Only the
// ACTIVE player discards (CR 514.1); nobody else is asked during their turn's
// cleanup.
func (e *Engine) cleanupStep() {
	hand := e.G.Zone(state.ZHand, e.G.Active)
	if len(hand) > maxHandSize {
		n := len(hand) - maxHandSize
		opts := make([]decision.Option, 0, len(hand))
		for _, id := range hand {
			name := "a card"
			if o := e.G.Obj(id); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			opts = append(opts, decision.Option{Index: len(opts), Kind: "discard",
				Label: "Discard " + name, Obj: id, Player: e.G.Active})
		}
		e.choosing = chooseCleanup
		e.ask(&decision.Decision{Player: e.G.Active, Kind: decision.KChoose, Min: n, Max: n,
			Prompt:  fmt.Sprintf("turn %d — discard %d card(s) down to the hand-size limit", e.G.Turn, n),
			Options: opts})
		return
	}
	e.cleanupBody()
}

// cleanupBody is the CR 514.2 portion of the cleanup step, run exactly once
// per cleanup step. It removes damage marked on every permanent (combat or
// otherwise), clears this turn's Deathtouched markers (a deathtouch mark lasts
// only as long as the damage it accompanied, CR 702.2c), and drops every
// "until end of turn" continuous effect the layer system is holding
// (Engine.EndOfTurnCleanup, layers.go -- built and tested since Task 19c, but
// nothing ever called it, so a resolved pump effect such as Giant Growth used
// to survive forever instead of expiring at the end of the turn it was cast
// in). Called either directly from cleanupStep when no discard is owed, or
// from discardCleanup after the discard answer is recorded -- never both, so
// the 514.2 actions are never doubled.
func (e *Engine) cleanupBody() {
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			if o.Damage > 0 {
				e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: -o.Damage})
			}
			if n := o.Counter("Deathtouched"); n > 0 {
				e.emit(events.Event{Kind: events.CounterChange, Obj: id,
					Counter: "Deathtouched", Amount: -n})
			}
		}
	}
	e.EndOfTurnCleanup()
}

// chooseCleanup is the chooseFor the pending KChoose discard decision belongs
// to (engine.go): it lets handleChoose route the answer to discardCleanup
// rather than to a cast/etb/miracle flow or the no-flow Note fallback. It
// extends the chooseFor enum in its own file, the same pattern Tasks 12 and
// 18 used for chooseETB and chooseMiracle. iota+4 is pairwise distinct from
// the shared package set chooseCast=1 / chooseETB=2 / chooseMiracle=3 (cast.
// go); the exact numbers only need to differ, never to be adjacent.
const chooseCleanup chooseFor = iota + 4

// discardCleanup applies an answered CR 514.1 discard decision: each chosen
// card moves from the active player's hand to their graveyard (a plain
// MoveZone event per card, in the order the client selected them), then the
// CR 514.2 body runs (cleanupBody), then the turn hands to the next player's
// turn (advanceStep). The move events ride the ordinary emit path, so
// state-based actions and triggered abilities matched by the discard are
// queued exactly as for any other zone change and handled by the same
// machinery the next priority round already drives (see the 514.3 follow-up
// note in the task report: a cleanup-created trigger is placed at the next
// player's first priority rather than in a repeated cleanup step, because
// implementing the CR 514.3 repeated-step control flow -- the no-priority
// turn loop re-entering itself to grant priority and then redoing cleanup --
// was judged not a small, clearly-correct addition at this point of
// priorityRound; an honest recorded gap beats a speculative turn-loop
// change, and the brief directs exactly that). advanceStep is called from
// here only after EVERY part of the cleanup -- the discard and the 514.2
// body -- has run, so the step is never advanced mid-cleanup.
func (e *Engine) discardCleanup(chosen []decision.Option) {
	for _, opt := range chosen {
		e.emit(events.Event{Kind: events.MoveZone, Obj: opt.Obj,
			From: state.ZHand, To: state.ZGraveyard, Player: e.G.Active})
	}
	e.cleanupBody()
	e.advanceStep()
}

// Registered here: exactly the eight keywords this task actually implements
// (Flying/Reach gate blocking in canBlock; Haste and Vigilance gate/modify
// attacking in canAttack/handleAttackers; Deathtouch, Trample, Lifelink and
// First Strike are all read directly in damageStep above and
// destroyLethalDamage, sba.go), plus three the M2r ratchet adds, each with a
// named proof test in keyword_registration_test.go: Flash (legal.go's
// instant-speed gate), Indestructible (destroyLethalDamage and, via
// Host.HasKeyword, Destroy/DestroyAll) and Devoid (effects.ColorsOf). Double
// Strike is still deliberately NOT registered: it is read by
// anyFirstStrike/damageStep but has no proof test of its own, and registering
// a keyword the build only partially or incidentally handles would tell the
// coverage report -- and so the deck-builder gate downstream of it -- that a
// card carrying it is safe to play, which is worse than leaving it reported
// as unsupported.
func init() {
	effects.RegisterNonAPI("kw:Flying", "kw:Reach", "kw:Haste", "kw:Vigilance",
		"kw:Deathtouch", "kw:Trample", "kw:Lifelink", "kw:First Strike",
		"kw:Flash", "kw:Indestructible", "kw:Devoid")
}
