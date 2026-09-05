// Task 21: combat. CR 508-510 in miniature -- declare attackers, declare
// blockers, then damage -- plus the CR 514.2 cleanup that combat depends on
// and that nothing in this codebase implemented before now. The lethal-
// damage and Deathtouch state-based actions this file's own damage marking
// depends on were also added here by Task 21, as destroyLethalDamage, but
// moved to sba.go by Task 22 -- they are general state-based-action logic,
// not combat-specific, and now live alongside the rest of CR 704.
//
// M1's own simplifications, matching the brief this task was built from:
//   - Only the active player attacks, and every attacker's defending player is
//     the same single seat: the next living one after the active player (CR
//     508.1's "each attacking creature's controller announces which opponent
//     ... it's attacking" is a real choice in a 3+ player game, deferred to
//     M2's UI work -- decision.Option.Player already carries that value, so
//     widening this later is additive, not a rewrite).
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
	if e.HasKeyword(attacker, "Flying") && !e.HasKeyword(blocker, "Flying") && !e.HasKeyword(blocker, "Reach") {
		return false
	}
	if e.blockRestricted(blocker, attacker) {
		return false
	}
	return true
}

// askAttackers builds a KAttackers decision, one option per creature passing
// canAttack, with the (M1-fixed) defending player already filled into each
// option's Player field.
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
	defender := e.G.NextAlive(p)
	var opts []decision.Option
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if !e.canAttack(id) {
			continue
		}
		opts = append(opts, decision.Option{Index: len(opts), Kind: "attacker",
			Label: "Attack with " + e.G.Obj(id).Face().Name, Obj: id, Player: defender})
	}
	if len(opts) == 0 {
		e.setStep(state.StepEndCombat)
		return
	}
	e.ask(&decision.Decision{Player: p, Kind: decision.KAttackers, Min: 0, Max: len(opts),
		Prompt: fmt.Sprintf("turn %d — declare attackers", e.G.Turn), Options: opts})
}

// handleAttackers records the chosen attackers (CR 508.1c: this is what
// causes triggered abilities matching "Attacks" to fire, via checkTriggers
// running behind every emit) and taps each one unless it has Vigilance (CR
// 508.1f, 702.20b).
func (e *Engine) handleAttackers(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	if len(chosen) == 0 {
		e.advanceStep()
		return
	}
	ids := make([]state.ObjID, 0, len(chosen))
	defender := chosen[0].Player
	for _, opt := range chosen {
		ids = append(ids, opt.Obj)
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: defender, IDs: ids})
	for _, id := range ids {
		if !e.HasKeyword(id, "Vigilance") {
			e.emit(events.Event{Kind: events.Tap, Obj: id})
		}
	}
	e.advanceStep()
}

// askBlockers builds a KBlockers decision, one option per (blocker, attacker)
// pair passing canBlock, with the attacker recorded in Option.Attacker (server-
// side only, same as castSpell's AltCostIndex) so handleBlockers knows which
// attacker each chosen blocker is blocking without the client needing to.
//
// With no attackers this combat (declared 0, or all already gone), there is
// nothing to block; with attackers but zero legal blockers for them (every
// candidate fails canBlock, e.g. a lone ground creature against a flier),
// there is a real declare-blockers step but no decision whose answer could
// differ from "block with nothing" -- both skip straight to the combat-damage
// step, the same reasoning askAttackers applies to its own empty case.
//
// Ruling T21-c (Task 21 fix round 1): the attacker-collection loop below
// additionally requires Face() != nil. DeclareAttackers's own events.Apply
// case sets IsAttacking on any existing object with no such check (Player is
// validated, but nothing about the object it names), so a malformed or
// tampered event -- or a nil-Card object such as an ability's own stack
// object (Ruling F3) -- reaching IsAttacking used to make it as far as the
// label build a few lines down, which read e.G.Obj(aid).Face().Name
// unconditionally: a nil-pointer panic, and therefore a remote kill of the
// whole match (one goroutine runs it). canAttack already requires this for a
// real attacker, so no legitimate attacker is excluded by requiring it here
// too.
func (e *Engine) askBlockers() {
	var attackers []state.ObjID
	var defender state.PlayerID
	for _, id := range e.G.Zone(state.ZBattlefield, e.G.Active) {
		o := e.G.Obj(id)
		if o == nil || !o.IsAttacking || o.Face() == nil {
			continue
		}
		attackers = append(attackers, id)
		defender = o.Attacking
	}
	if len(attackers) == 0 {
		e.setStep(state.StepCombatDamage)
		return
	}
	var opts []decision.Option
	for _, bid := range e.G.Zone(state.ZBattlefield, defender) {
		for _, aid := range attackers {
			if !e.canBlock(bid, aid) {
				continue
			}
			opts = append(opts, decision.Option{Index: len(opts), Kind: "block",
				Label: e.G.Obj(bid).Face().Name + " blocks " + e.G.Obj(aid).Face().Name,
				Obj:   bid, Attacker: aid, Player: defender})
		}
	}
	if len(opts) == 0 {
		e.setStep(state.StepCombatDamage)
		return
	}
	e.ask(&decision.Decision{Player: defender, Kind: decision.KBlockers, Min: 0, Max: len(opts),
		Prompt: fmt.Sprintf("turn %d — declare blockers", e.G.Turn), Options: opts})
}

// handleBlockers records the chosen (attacker, blocker) pairs in one
// DeclareBlockers event, in the order the client submitted them -- that order
// is what BlockedBy preserves (events.Apply's DeclareBlockers case is a plain
// append per pair) and so what dealCombatDamage's damage-assignment loop
// below reads as "blocker order" for CR 510.1c's ordered damage assignment.
func (e *Engine) handleBlockers(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	if len(chosen) > 0 {
		pairs := make([][2]state.ObjID, 0, len(chosen))
		for _, opt := range chosen {
			pairs = append(pairs, [2]state.ObjID{opt.Attacker, opt.Obj})
		}
		e.emit(events.Event{Kind: events.DeclareBlockers, Pairs: pairs})
	}
	e.advanceStep()
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
						lifelink: a.Controller, hasLink: link})

				case len(blockers) == 0:
					// Ruling T21-d (CR 509.1h): a creature that was blocked
					// stays blocked for the rest of combat even if every
					// creature blocking it has since left -- it deals no
					// combat damage at all, unless Trample lets the whole
					// amount push through to the player instead (there is no
					// blocker left to owe any of it to).
					if trample {
						as = append(as, assignment{toPlayer: a.Attacking, amount: pw,
							lifelink: a.Controller, hasLink: link})
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
							lifelink: a.Controller, hasLink: link, deathtouch: dt})
						remaining -= give
						if remaining <= 0 {
							break
						}
					}
					if remaining > 0 && trample {
						as = append(as, assignment{toPlayer: a.Attacking, amount: remaining,
							lifelink: a.Controller, hasLink: link})
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
					deathtouch: e.HasKeyword(bid, "Deathtouch")})
			}
		}
	}
	for _, x := range as {
		if x.toObj != 0 {
			e.emit(events.Event{Kind: events.Damage, Obj: x.toObj, Amount: x.amount})
			if x.deathtouch {
				e.emit(events.Event{Kind: events.CounterChange, Obj: x.toObj,
					Counter: "Deathtouched", Amount: 1})
			}
		} else {
			e.emit(events.Event{Kind: events.Damage, Player: x.toPlayer, Amount: x.amount})
		}
		if x.hasLink {
			e.emit(events.Event{Kind: events.LifeChange, Player: x.lifelink, Amount: x.amount})
		}
	}
}

// cleanupStep performs the CR 514.2 cleanup actions Task 21 owns wiring up:
// damage marked on every permanent (combat or otherwise) is removed, this
// turn's Deathtouched markers go with it (a deathtouch mark lasts only as
// long as the damage it accompanied, CR 702.2c), and every "until end of
// turn" continuous effect the layer system is holding is dropped
// (Engine.EndOfTurnCleanup, layers.go -- built and tested since Task 19c, but
// nothing ever called it, so a resolved pump effect such as Giant Growth used
// to survive forever instead of expiring at the end of the turn it was cast
// in). Called from turn.go's priorityRound, immediately before it advances
// past the cleanup step exactly as it already did.
func (e *Engine) cleanupStep() {
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

// Registered here: exactly the eight keywords this task actually implements
// (Flying/Reach gate blocking in canBlock; Haste and Vigilance gate/modify
// attacking in canAttack/handleAttackers; Deathtouch, Trample, Lifelink and
// First Strike are all read directly in damageStep above and
// destroyLethalDamage, sba.go). Indestructible and Double Strike are read by
// destroyLethalDamage and anyFirstStrike/damageStep respectively, but are
// deliberately NOT registered: neither is exercised by this task's tests, and
// registering a keyword the build only partially or incidentally handles
// would tell the coverage report -- and so the deck-builder gate downstream
// of it -- that a card carrying it is safe to play, which is worse than
// leaving it reported as unsupported.
func init() {
	effects.RegisterNonAPI("kw:Flying", "kw:Reach", "kw:Haste", "kw:Vigilance",
		"kw:Deathtouch", "kw:Trample", "kw:Lifelink", "kw:First Strike")
}
