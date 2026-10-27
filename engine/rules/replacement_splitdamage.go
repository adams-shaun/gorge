package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// splitShieldIndex finds the Effect-created registration a DB$
// ReplaceSplitDamage match belongs to, or -1 for a printed line (none in the
// corpus; it then carries no pool). The registration is keyed by the Effect's
// source and timestamp like applyReplaceDamageTail's.
func (e *Engine) splitShieldIndex(m replMatch) int {
	source, ts, ok := parseEffectKey(m.key)
	if !ok {
		return -1
	}
	for i := range e.continuous {
		ce := &e.continuous[i]
		if ce.Source == source && ce.Timestamp == ts && ce.ReplacementEvent == "DamageDone" &&
			strings.Contains(ce.ReplacementBody, "ReplaceSplitDamage") {
			return i
		}
	}
	return -1
}

// splitDamageRecipient resolves the body's DamageTarget$ (Remembered,
// EffectSource, Imprinted, You) in the match's context to the first live
// recipient: a player still in the game or a permanent on the battlefield.
// ok is false when none is left, and the held event then stands unchanged.
func (e *Engine) splitDamageRecipient(m replMatch, ev events.Event) (state.Target, bool) {
	spec := effects.ReplaceSplitDamageOf(m.repl.With).DamageTarget
	if spec == "" {
		return state.Target{}, false
	}
	for _, t := range effects.DefinedSpec(e, e.replCtx(m, ev), spec) {
		if t.IsPlayer {
			if int(t.Player) < len(e.G.Players) && !e.G.Players[t.Player].Lost {
				return t, true
			}
			continue
		}
		if o := e.G.Obj(t.Obj); t.Obj != 0 && o != nil && o.Zone == state.ZBattlefield {
			return t, true
		}
	}
	return state.Target{}, false
}

// applyReplaceSplitDamageBody applies a DB$ ReplaceSplitDamage body ("the next
// N damage that would be dealt to X is dealt to Y instead", CR 614.1a): n =
// min(held amount, shield left) is dealt to the DamageTarget$ recipient as an
// ordinary Damage event from the same source, the held event keeps the rest,
// and the shield (an Effect registration's ChosenNumber pool, VarName$ or 1)
// is spent by n and dropped at 0 -- the Forge ReplaceSplitDamageEffect shape.
// It reports whether the held event is TERMINAL (nothing left for the original
// recipient); otherwise the reduced event stands for the next modifier.
func (e *Engine) applyReplaceSplitDamageBody(ev *events.Event, m replMatch) bool {
	if ev.Amount <= 0 {
		return false
	}
	idx := e.splitShieldIndex(m)
	pool := int32(0)
	if idx >= 0 {
		pool = e.continuous[idx].ChosenNumber
	}
	if pool <= 0 {
		var resolved bool
		if pool, resolved = effects.ReplaceSplitDamageShield(e, e.replCtx(m, *ev), m.repl.With); !resolved {
			return false
		}
	}
	n := min(ev.Amount, pool)
	if n <= 0 {
		return false
	}
	to, ok := e.splitDamageRecipient(m, *ev)
	if !ok {
		return false
	}
	if idx >= 0 {
		if pool-n > 0 {
			e.continuous[idx].ChosenNumber = pool - n
		} else {
			e.continuous = append(e.continuous[:idx], e.continuous[idx+1:]...)
		}
		e.continuousVersion++
	}
	redirected := *ev
	redirected.Amount = n
	redirected.Obj, redirected.Player = 0, 0
	if to.IsPlayer {
		redirected.Player = to.Player
	} else {
		redirected.Obj = to.Obj
	}
	// The recipient changed kind possibly (player <-> permanent): recompute the
	// infect/wither form markers exactly as the ReplaceEffect Affected rewrite does.
	e.recomputeInfectMarker(&redirected)
	e.recomputeWitherMarker(&redirected)
	ev.Amount -= n
	e.emit(redirected)
	return ev.Amount <= 0
}
