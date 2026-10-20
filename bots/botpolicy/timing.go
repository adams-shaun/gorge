package botpolicy

import (
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
}

// Enabled reports whether any A6 timing rule is armed.
func (r TimingRules) Enabled() bool { return r.CounterTop || r.DrawTokenLate }

// counterSkipCMC is the mana value the counter rule refuses: Forge counters
// a CMC-1 spell only 30% of the time, so the deterministic reading holds
// the counter. CMC 0 is ungated there (and CmcOf counts {X} as 0), so zero
// is countered.
const counterSkipCMC int32 = 1

// timingNow reports whether the cast of o is a cast the armed timing rules
// allow in this window. A rule returns true for a card it does not govern,
// so the zero TimingRules is a no-op and each rule can be armed alone.
func (b Board) timingNow(d *decision.Decision, o decision.Option) bool {
	if b.Timing.CounterTop {
		c := b.Cards.Get(o.Obj)
		if c.Counter {
			top, ok := b.stackTop(0)
			if !ok || !top.IsSpell || top.Controller == d.Player || top.CMC == counterSkipCMC {
				return false
			}
		}
	}
	if b.Timing.DrawTokenLate {
		if !b.drawTokenNow(b.Cards.Get(o.Obj)) {
			return false
		}
	}
	return true
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
// payable. It composes removalHeld (A1) with the A6 gate, so the main-phase
// tap gate and the timing rules share one answer.
func (b Board) castHeld(d *decision.Decision, id state.ObjID) bool {
	return b.removalHeld(d, id) || !b.timingNow(d, decision.Option{Kind: "cast", Obj: id})
}

// timedCard reports whether c is a card one of the armed timing rules
// governs: a counter under CounterTop, or an instant-speed draw/token under
// DrawTokenLate. A sorcery-speed draw/token is not a timed tap candidate
// (the tap gate is for instants); its hold comes from timingNow.
func (b Board) timedCard(c Card) bool {
	return b.Timing.CounterTop && c.Counter ||
		b.Timing.DrawTokenLate && c.InstantSpeed && (c.Draw || c.Token)
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
// removal ranking's highest-CMC foreign spell. It returns false when no
// armed rule governs this ask, leaving chooseTargets unchanged.
func (b Board) timingTargetPick(d *decision.Decision) (int, bool) {
	if !b.Timing.CounterTop || d.TargetEffect == nil || d.TargetEffect.API != "Counter" {
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
	return 0, false
}
