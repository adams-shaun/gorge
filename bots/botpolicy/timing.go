package botpolicy

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TimingRules selects the A6 "is now a good time" rules. The zero value is
// off, so every existing Board and both adapters keep their current cast
// decisions.
type TimingRules struct {
	// CounterTop casts a counter only at a foreign spell on TOP of the
	// stack, and never at a mana value of counterSkipCMC (Forge's
	// deterministic majority reading of its 30/75/100% bands).
	CounterTop bool
	// DrawTokenLate holds a draw or token spell until the window Forge
	// waits for: a sorcery until the seat's own second main, an instant
	// draw until an opponent's end step (or a hand of at most one card),
	// and an instant token until an opponent's declare-attackers step
	// onward.
	DrawTokenLate bool
	// PumpInCombat casts an instant creature pump only at declare
	// blockers, and only when the bonus saves a combatant that would die,
	// lets one kill an opposing creature that would otherwise survive, or
	// makes an unblocked attack lethal.
	PumpInCombat bool
}

// Enabled reports whether any A6 timing rule is armed.
func (r TimingRules) Enabled() bool { return r.CounterTop || r.DrawTokenLate || r.PumpInCombat }

// counterSkipCMC is the mana value the counter rule refuses: Forge counters
// a CMC-1 spell only 30% of the time, so the deterministic reading holds
// the counter. CMC 0 is ungated there (and CmcOf counts {X} as 0), so zero
// is countered.
const counterSkipCMC int32 = 1

// timingNow reports whether the cast of o is a cast the armed timing rules
// allow in this window. A rule returns true for a card it does not govern,
// so the zero TimingRules is a no-op and each rule can be armed alone.
func (b Board) timingNow(d *decision.Decision, o decision.Option) bool {
	c := b.Cards.Get(o.Obj)
	if b.Timing.CounterTop && c.Counter {
		top, ok := b.stackTop(0)
		if !ok || !top.IsSpell || top.Controller == d.Player || top.CMC == counterSkipCMC {
			return false
		}
	}
	if b.Timing.DrawTokenLate {
		if !b.drawTokenNow(c) {
			return false
		}
	}
	if b.Timing.PumpInCombat && b.isTimedPump(c) {
		if !c.PumpOwn {
			return false
		}
		if b.Step != state.StepDeclareBlockers || b.ownSpellOnStack(d.Player) {
			return false
		}
		if _, ok := b.bestPumpTarget(d.Player, c.PumpPower, c.PumpToughness); !ok {
			return false
		}
	}
	return true
}

// isTimedPump reports whether c is a castable instant-speed creature pump
// with a literal, positive bonus -- the cards the PumpInCombat rule governs.
// A pump whose target filter cannot name both the caster's own attackers and
// blockers is governed too, and timingNow holds it: the rule must never fire
// a cast whose only legal target is the opponent's creature.
func (b Board) isTimedPump(c Card) bool {
	return c.Castable && c.InstantSpeed && (c.PumpPower > 0 || c.PumpToughness > 0)
}

// ownSpellOnStack reports whether the stack already holds a spell player
// controls. A second pump while the first is unresolved is never worth
// casting (CR 117.3c: it cannot affect the same combat before the first
// resolves), so the cast gate holds it.
func (b Board) ownSpellOnStack(player state.PlayerID) bool {
	for _, s := range b.Stack {
		if s.IsSpell && s.Controller == player {
			return true
		}
	}
	return false
}

// Pump decision classes, in ascending value: a save beats a kill, which
// beats a lethal push. Zero means "the bonus changes nothing".
const (
	pumpClassNone int32 = iota
	pumpClassLethal
	pumpClassKill
	pumpClassSave
)

// bestPumpTarget is the combatant an instant pump of power/toughness should
// aim at, if any. It considers the seat's own creatures that are attacking
// or blocking (and are not shrouded), simulates the combat with and without
// the bonus through the same blockCombat the combat heuristic uses, and
// returns the best class, then the highest threat(), then the lowest object
// id -- an order-independent max fold.
func (b Board) bestPumpTarget(me state.PlayerID, power, toughness int32) (state.ObjID, bool) {
	if power <= 0 && toughness <= 0 {
		return 0, false
	}
	ids := make([]state.ObjID, 0, b.Creatures.Len())
	for id, cr := range b.Creatures.All() {
		if cr.Controller != me || cr.hasKeyword("Shroud") {
			continue
		}
		if !cr.Attacking && cr.Blocks == 0 {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var bestID state.ObjID
	var bestClass, bestThreat int32
	found := false
	for _, id := range ids {
		t := b.Creatures.Get(id)
		pumped := t
		pumped.Power += power
		pumped.Toughness += toughness
		class := b.pumpClass(me, id, t, pumped)
		if class == pumpClassNone {
			continue
		}
		th := t.threat()
		if !found || class > bestClass || (class == bestClass && th > bestThreat) {
			bestID, bestClass, bestThreat, found = id, class, th, true
		}
	}
	return bestID, found
}

// pumpClass scores what the pumped body does in its combat: saves it from
// death (3), lets it kill an opposing creature that would survive (2), or
// makes an unblocked attack lethal (1). Indestructible creatures are never
// saved or killed by damage, so they never justify the pump that way.
func (b Board) pumpClass(me state.PlayerID, id state.ObjID, t, pumped Creature) int32 {
	if t.Attacking {
		blockerIDs, blockers := b.blockersOf(id)
		if len(blockerIDs) > 0 {
			aDead, dead := blockCombat(t, blockers)
			aDeadP, deadP := blockCombat(pumped, blockers)
			if aDead && !aDeadP && !t.hasKeyword("Indestructible") {
				return pumpClassSave
			}
			for i := range blockers {
				if !dead[i] && deadP[i] && !blockers[i].hasKeyword("Indestructible") {
					return pumpClassKill
				}
			}
			return pumpClassNone
		}
		return b.pumpLethal(t, pumped.Power-t.Power)
	}
	attacker, ok := b.Creatures.Lookup(t.Blocks)
	if !ok || attacker.Controller == me {
		return pumpClassNone
	}
	blockerIDs, blockers := b.blockersOf(t.Blocks)
	idx := slices.Index(blockerIDs, id)
	if idx < 0 {
		return pumpClassNone
	}
	pumpedBlockers := slices.Clone(blockers)
	pumpedBlockers[idx] = pumped
	aDead, dead := blockCombat(attacker, blockers)
	aDeadP, deadP := blockCombat(attacker, pumpedBlockers)
	if dead[idx] && !deadP[idx] && !t.hasKeyword("Indestructible") {
		return pumpClassSave
	}
	if !aDead && aDeadP && !attacker.hasKeyword("Indestructible") {
		return pumpClassKill
	}
	return pumpClassNone
}

// pumpLethal reports the lethal class when an unblocked attacker's bonus
// takes its team's unblocked damage from below the defender's life to at or
// above it. An attack that is already lethal never justifies the pump.
func (b Board) pumpLethal(t Creature, bonus int32) int32 {
	if bonus <= 0 || t.Blocks != 0 {
		return pumpClassNone
	}
	life := b.Life.Get(t.Defender)
	if life <= 0 {
		return pumpClassNone
	}
	var sum int32
	for _, cr := range b.Creatures.All() {
		if cr.Controller != t.Controller || !cr.Attacking || cr.Blocks != 0 || cr.Defender != t.Defender {
			continue
		}
		if cr.Power > 0 {
			sum += cr.Power
		}
	}
	if sum < life && sum+bonus >= life {
		return pumpClassLethal
	}
	return pumpClassNone
}

// blockersOf returns the ids and facts of every creature blocking attacker,
// in ascending id order so the combat simulation (and every tie) is
// deterministic.
func (b Board) blockersOf(attacker state.ObjID) ([]state.ObjID, []Creature) {
	var ids []state.ObjID
	for id, cr := range b.Creatures.All() {
		if cr.Blocks == attacker {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	crs := make([]Creature, len(ids))
	for i, id := range ids {
		crs[i] = b.Creatures.Get(id)
	}
	return ids, crs
}

// drawTokenNow reports whether the draw/token timing rule allows casting c
// now. It governs only a card whose primary spell ability is Draw or Token;
// every other card, and every card while the rule is off, is allowed.
// Sorcery-speed draw/token is held in the seat's own first main (the second
// main re-offers it). An instant draw waits for an opponent's end step
// unless the hand is nearly empty, and an instant token waits for the
// opponent's declare-attackers step onward. The multiplayer reading of "the
// turn before the AI's" is any opponent's turn, as documented in the A6
// design note.
func (b Board) drawTokenNow(c Card) bool {
	if !b.Timing.DrawTokenLate || (!c.Draw && !c.Token) {
		return true
	}
	if !c.InstantSpeed {
		return !(b.MyTurn && b.FirstMain)
	}
	if c.Draw {
		return (!b.MyTurn && b.Step == state.StepEnd) || b.HandSize <= 1
	}
	return !b.MyTurn && b.Step >= state.StepDeclareAttackers && b.Step <= state.StepEnd
}

// castHeld is the tap gate's half of the timing rules: it reports whether
// the cast of card id is held right now, so no tap may be aimed at making it
// payable. It composes removalHeld (A1) and auraHeld (A5) with the A6 gate, so
// the main-phase tap gate and the timing rules share one answer.
func (b Board) castHeld(d *decision.Decision, id state.ObjID) bool {
	return b.removalHeld(d, id) || b.auraHeld(d, id) || !b.timingNow(d, decision.Option{Kind: "cast", Obj: id})
}

// timedCard reports whether c is a card one of the armed timing rules
// governs: a counter under CounterTop, an instant-speed draw/token under
// DrawTokenLate, or a castable instant creature pump with a positive literal
// bonus under PumpInCombat. A sorcery-speed draw/token is not a timed tap
// candidate (the tap gate is for instants); its hold comes from timingNow.
func (b Board) timedCard(c Card) bool {
	return b.Timing.CounterTop && c.Counter ||
		b.Timing.DrawTokenLate && c.InstantSpeed && (c.Draw || c.Token) ||
		b.Timing.PumpInCombat && b.isTimedPump(c)
}

// timedTap is the out-of-main tap gate for the timing rules. It is the
// manual seat's way to float mana toward an instant-speed spell the rules
// allow in a window whose ordinary tap gate is main-phase-only: among the
// castable instant-speed cards the rule governs and this window can pay, it
// picks the best cardScore (ties to the lowest object id) and hands it to
// the shared tap ranking (tapToward). It returns -1 when no such card
// exists, so the ordinary priority flow continues unchanged.
func (b Board) timedTap(d *decision.Decision) int {
	offered := b.offeredColours(d)
	bestID := state.ObjID(0)
	var best Card
	var bestScore int32
	found := false
	keys, cs := b.Cards.Keys(), b.Cards.Values()
	for i := range cs {
		id, c := keys[i], cs[i]
		if !c.Castable || !c.InstantSpeed || c.CMC <= 0 || !b.timedCard(c) {
			continue
		}
		if !tapTarget(d, id) || b.poolPays(id, c) || !b.offeredClosesPips(c, offered) {
			continue
		}
		if !b.castableNowCard(d.Player, id) {
			continue
		}
		if !b.timingNow(d, decision.Option{Kind: "cast", Obj: id}) {
			continue
		}
		s := b.cardScore(c)
		if !found || s > bestScore || (s == bestScore && id < bestID) {
			bestID, best, bestScore, found = id, c, s, true
		}
	}
	if !found {
		return -1
	}
	return b.tapToward(d, best)
}

// stackTop returns the last stack entry whose ID is not skip, i.e. the
// object below the spell being cast (skip 0 at an ordinary priority, d.Source
// in a target ask, where the spell itself is already on the stack).
func (b Board) stackTop(skip state.ObjID) (StackEntry, bool) {
	for i := len(b.Stack) - 1; i >= 0; i-- {
		if b.Stack[i].ID != skip {
			return b.Stack[i], true
		}
	}
	return StackEntry{}, false
}

// timingTargetPick is the target half of the timing rules. A counter may
// only target the foreign spell on top of the stack below the counter being
// cast (skip d.Source), so it picks that option instead of the ordinary
// removal ranking's highest-CMC foreign spell. A pump being cast picks the
// combatant bestPumpTarget names (the save/kill/lethal fold), not the
// ordinary ranking's target -- which would otherwise lead with the opposing
// creature. It returns false when no armed rule governs this ask, leaving
// chooseTargets unchanged.
func (b Board) timingTargetPick(d *decision.Decision) (int, bool) {
	if d.TargetEffect == nil {
		return 0, false
	}
	switch d.TargetEffect.API {
	case "Counter":
		if !b.Timing.CounterTop {
			return 0, false
		}
		top, ok := b.stackTop(d.Source)
		if !ok || !top.IsSpell || top.Controller == d.Player {
			return 0, false
		}
		for i := range d.Options {
			if d.Options[i].Obj == top.ID {
				return i, true
			}
		}
	case "Pump":
		if !b.Timing.PumpInCombat {
			return 0, false
		}
		var power, toughness int32
		found := false
		for i := range b.Stack {
			if b.Stack[i].ID == d.Source && b.Stack[i].IsSpell {
				power, toughness = b.Stack[i].PumpPower, b.Stack[i].PumpToughness
				found = true
				break
			}
		}
		if !found || (power <= 0 && toughness <= 0) {
			return 0, false
		}
		id, ok := b.bestPumpTarget(d.Player, power, toughness)
		if !ok {
			return 0, false
		}
		for i := range d.Options {
			if d.Options[i].Obj == id {
				return i, true
			}
		}
	}
	return 0, false
}
