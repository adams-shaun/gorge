package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Clone deep-copies the engine: game, log, RNG position, the pending
// decision, continuous effects, the pending-trigger queue and the trigger
// bookkeeping maps. The copy and the original then evolve independently —
// the same intents submitted to both produce the same events, chain head
// and RNG draw count (clone_test.go pins that), and nothing submitted to
// one is visible to the other. Card data (*cards.Card, *cards.SA) is
// shared: the compiled corpus is immutable once loaded.
//
// Call it only at an intent boundary — after New, Advance or Submit has
// returned, so Pending() != nil or G.Over. That is the only moment the
// fields below are not being written. A match host clones at every turn
// start to answer "view at seq N" with at most one turn of replay.
func (e *Engine) Clone() *Engine {
	c := &Engine{
		G:                   e.G.Clone(),
		L:                   e.L.Clone(),
		rng:                 e.rng.clone(),
		orderedTriggers:     e.orderedTriggers,
		applyingReplacement: e.applyingReplacement,
		choosing:            e.choosing,
	}
	if e.pending != nil {
		d := *e.pending
		d.Options = append([]decision.Option(nil), e.pending.Options...)
		c.pending = &d
	}
	if e.continuous != nil {
		c.continuous = make([]ContinuousEffect, len(e.continuous))
		for i, ce := range e.continuous {
			ce.AddKeywords = append([]string(nil), ce.AddKeywords...)
			ce.AddTypes = append([]string(nil), ce.AddTypes...)
			c.continuous[i] = ce
		}
	}
	if e.pendingTriggers != nil {
		c.pendingTriggers = make([]pendingTrigger, len(e.pendingTriggers))
		for i, pt := range e.pendingTriggers {
			pt.Ctx.Targets = append([]state.Target(nil), pt.Ctx.Targets...)
			pt.Ctx.Remembered = append([]state.Target(nil), pt.Ctx.Remembered...)
			if pt.Ctx.SVars != nil {
				m := make(map[string]string, len(pt.Ctx.SVars))
				for k, v := range pt.Ctx.SVars {
					m[k] = v
				}
				pt.Ctx.SVars = m
			}
			c.pendingTriggers[i] = pt
		}
	}
	c.triggerFireCount = cloneCounts(e.triggerFireCount)
	c.damageOnceFired = cloneCounts(e.damageOnceFired)
	if e.cast != nil {
		pc := *e.cast
		pc.cost.Sac = append([]CostPart(nil), e.cast.cost.Sac...)
		pc.cost.SubCounter = append([]CostPart(nil), e.cast.cost.SubCounter...)
		pc.delve = append([]state.ObjID(nil), e.cast.delve...)
		pc.sacs = append([]state.ObjID(nil), e.cast.sacs...)
		c.cast = &pc
	}
	return c
}

// cloneCounts copies a trigger-bookkeeping map, preserving nil (trigger.go
// lazily allocates these on first use and checks for nil itself).
func cloneCounts(m map[triggerKey]int32) map[triggerKey]int32 {
	if m == nil {
		return nil
	}
	out := make(map[triggerKey]int32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
