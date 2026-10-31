// static_trigger.go resolves Forge's Static$ True triggers off the stack.
//
// Forge marks a trigger line Static$ True when its Execute body is
// bookkeeping: TriggerHandler.runWaitingTrigger's isStatic arm runs such a
// trigger's body immediately at its event (playTrigger, no stack object), so
// no player can respond to it and it is never part of a CR 603.3b order
// choice. The corpus marks 174 cards' T: lines this way (ChangesZone 126,
// TapsForMana 49, SpellCast 18, Phase 17, ...), plus AB$ DelayedTrigger
// lines. Two of the shapes are already handled elsewhere:
//
//   - A triggered mana ability (TapsForMana + Static$ True) resolves through
//     the CR 605.3b mana path (engine/rules/mana_activation.go).
//   - An event-matched delayed registration's static fire resolves inline at
//     fire time (engine/rules/trigger_delayed.go's f.static arm).
//
// This file covers the remaining shape: a printed (or granted/merged) face
// trigger whose line carries Static$ True, queued like any other trigger and
// resolved off the stack by the drain before the ordering ask.
//
// The resolution is gated on the same ask-free judgement the resolution
// kernel uses for a resolving object (cards.SAChainMayAsk plus the board
// gates). An off-stack resolution has no tape run to serve a mid-resolution
// decision, so a trigger whose body could ask stays on the stack and resolves
// through the ordinary path, exactly as it does today. That keeps the three
// corpus static triggers that ask (God-Eternal Kefnet and Inquisitor
// Eisenhorn's optional Reveal chains, Buzzing Whack-a-Doodle's GenericChoice,
// Ulamog's Despoiler's Mode) on their existing path.

package rules

import (
	mathbits "math/bits"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
)

// staticTriggerLine returns the trigger line, the SVar table that owns its
// Execute body and that owning face (nil when not known) for a pending
// trigger whose line carries Static$ True, or ok false for everything else.
//
// A granted (AddTrigger$) or gained trigger carries its line on pt.Trigger; a
// printed, merged or gained one is recovered by pointer from the source's
// faces. A Mode$ Phase delayed registration carries no line at all (its body
// was resolved from the Execute$ SVar when it was queued), and a printed
// trigger whose face went inactive mid-batch is carried as Delayed too, so
// both are skipped: they keep the stack path.
func (e *Engine) staticTriggerLine(pt pendingTrigger) (cards.Trigger, map[string]string, *cards.Face, bool) {
	if pt.Trigger.Mode != "" {
		if !strings.EqualFold(strings.TrimSpace(pt.Trigger.ParamStr(cards.PKStatic)), "True") {
			return cards.Trigger{}, nil, nil, false
		}
		return pt.Trigger, pt.TriggerSVars, nil, true
	}
	if pt.Delayed && pt.DelayedStatic && pt.SA != nil {
		// A Static$ True Mode$ Phase registration: no line was kept, so a
		// synthetic bare one stands in (no Optional/Unless/cost riders).
		src := e.G.Obj(pt.Source)
		if src == nil || src.Face() == nil {
			return cards.Trigger{}, nil, nil, false
		}
		f := src.Face()
		return cards.Trigger{Mode: "Phase"}, f.SVars, f, true
	}
	if pt.Delayed || pt.SA == nil {
		return cards.Trigger{}, nil, nil, false
	}
	t, f, ok := e.findTriggerForAbilityFace(pt.Source, pt.SA)
	if !ok || !strings.EqualFold(strings.TrimSpace(t.ParamStr(cards.PKStatic)), "True") {
		return cards.Trigger{}, nil, nil, false
	}
	var svars map[string]string
	if f != nil {
		svars = f.SVars
	}
	return t, svars, f, true
}

// staticTriggerAskFree reports whether resolving the static trigger's body
// is provably free of decisions. It is the pending-trigger form of the
// resolution kernel's ask-free predicate (engine/rules/resolve_mayask.go): the line
// must carry no optional/unless/cost rider, the body's chain must be outside
// every asking parameter and API, and the board must open no replacement
// gate. Anything not proven ask-free is refused, and the trigger takes the
// ordinary stack path.
func (e *Engine) staticTriggerAskFree(pt pendingTrigger, line cards.Trigger, svars map[string]string, self *cards.Face) bool {
	if pt.SA == nil || line.Mode == "TapsForMana" {
		// The CR 605.3b mana path owns triggered mana abilities; a nil body
		// has nothing to run.
		return false
	}
	if cards.TriggerLineMayAsk(line) || e.triggerBodyNeedsCostWindow(pt.SA) {
		return false
	}
	for sa := pt.SA; sa != nil; sa = sa.Sub {
		// CR 603.3d: a triggered ability's targets are announced when it is
		// put on the stack. A static trigger never is, so any targeted link
		// would have no announced targets to resolve with.
		if effects.TargetsOf(sa).Targeted() {
			return false
		}
		// A Charm picks its modes as it resolves unless the placement
		// announced them; a static trigger has no placement.
		if sa.API == "Charm" {
			return false
		}
	}
	st := saMayAskState(e, pt.SA, svars, self)
	if st&mayAskText != 0 || mayAskOnBoard(e, st) {
		return false
	}
	return !staticBodyCompetes(e, pt.SA, cards.ReplEventMask(st>>mayAskGateShift&mayAskGateMask))
}

// staticBodyCompetes is the replacement gate of a static trigger's body, which has
// no continuation to serve an ask. mayAskOnBoard has already judged elections and
// competing pairs on the events the body proposes, so what is left for these APIs
// is a replacement of those same events whose own body may ask; bookkeeping
// proposes no event at all. The bonus stays on the stack only there (CR 614.1c
// does not allow it, and nothing could answer the ask off it). Any other API may
// move a permanent and sweep an exiled card back, so it takes the engine-wide gate.
func staticBodyCompetes(e *Engine, sa *cards.SA, gates cards.ReplEventMask) bool {
	for s := sa; s != nil; s = s.Sub {
		switch s.API {
		case "Animate", "Cleanup", "DealDamage", "Effect", "PutCounter", "Pump", "PumpAll", "StoreSVar", "Token":
		default:
			return tapeBoardCompetes(e)
		}
	}
	var bits uint32
	for g := gates; g != 0; g &= g - 1 {
		bits |= replEventBits[cards.ReplEvent(mathbits.TrailingZeros32(uint32(g)))]
	}
	return bits != 0 && tapeReplBodyMayAsk(e, bits)
}

// resolveStaticTriggers resolves every queued trigger whose line carries
// Static$ True and whose body is provably ask-free, in queue order, and
// reports whether it resolved any. It runs at the top of the drain loop,
// before the CR 603.3b ordering ask: Forge resolves a static trigger
// immediately and never offers it to an order choice, so it is not part of
// the batch the players order. A trigger whose body may ask stays in the
// queue and is pushed like any other.
//
// A resolution can queue further triggers (its own events match), so the
// scan reads the live queue length each step; the caller re-runs the scan
// until it reports nothing, which also settles the ordering ask over a queue
// holding no statics.
func (e *Engine) resolveStaticTriggers() bool {
	resolved := false
	for i := 0; i < len(e.pendingTriggers); {
		pt := e.pendingTriggers[i]
		line, svars, self, ok := e.staticTriggerLine(pt)
		if !ok || !e.staticTriggerAskFree(pt, line, svars, self) {
			i++
			continue
		}
		e.removeTriggerAt(i)
		if pt.Delayed {
			// CR 603.7: consume the one-shot registration, no stack object.
			e.emit(events.Event{Kind: events.DelayedPush, Obj: pt.Source,
				Player: pt.Controller, Amount: int32(pt.DelayedID), Counter: pt.Execute,
				Text: "static"})
		}
		e.resolveStaticTrigger(pt, svars)
		resolved = true
	}
	return resolved
}

// resolveStaticTrigger runs a static trigger's body inline, the way
// resolveTop runs a resolving stack object's: the body's SVar table is bound
// onto the pending context first, and the in-flight damage source names the
// trigger's source for the body's own Damage events. The damage scratch is
// saved and restored because a static trigger can resolve inside a combat
// damage batch whose own source attribution is still live.
func (e *Engine) resolveStaticTrigger(pt pendingTrigger, svars map[string]string) {
	if svars != nil {
		effects.SetSVars(&pt.Ctx, svars)
	}
	damaging := e.damaging
	e.damaging = pt.Source
	effects.Resolve(e, &pt.Ctx, pt.SA)
	e.damaging = damaging
}
