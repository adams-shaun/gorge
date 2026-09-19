package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("ImmediateTrigger", effImmediateTrigger) }

// effImmediateTrigger implements Forge's ImmediateTrigger — the "when you do"
// body an SA chains after a cost (or a token creation with
// RememberOriginalTokens$) resolves: "At the beginning of your upkeep, create
// a token. When you do, return up to one target Aura ... attached to that
// token" (Forum Filibuster), "create two Zombies. When you do, exile up to
// two target cards" (Diregraf Horde), "you may pay {1}. When you do, target
// creature with haste can't be blocked ..." (Speed, Young Avenger's AB shape).
//
// The accepted approximation (this brief's contract, and deliberately NOT a
// new events.Kind nor a stack object): Forge pushes one real trigger instance
// per amount onto the stack, so each instance is a separate stack object that
// responds to priority ordering; this build resolves the Execute$ body
// INLINE, amount times, inside the resolving ability's own resolution. The
// ordering vs other stack objects therefore differs from Forge's whenever
// another ability could be placed between the "when you do" offer and its
// resolution — with no response window in between, the effect resolves
// immediately. Every per-instance behaviour (its target ask, its UnlessCost$
// gate, its own remembered set) is real; only the stack-object ceremony is
// not.
//
// Per instance the loop builds a FRESH Ctx copy (the fx42 scoping rule: an
// answer must never carry between instances) and resolves Execute$ through
// the ordinary Resolve, so a sub's own mid-resolution ask (Forum Filibuster's
// TargetMin$ 0 / TargetMax$ 1 ChangeZone) suspends the WHOLE resolution and
// re-enters through the same RepeatCursor machinery effRepeatEach uses —
// SuspendRepeat reports the loop's position (SA identity + next instance
// index), the enclosing Resolve chain drops its own continuation report for
// this SA (SuspendContinuation's repeatReported rule), and the resumed pass
// re-enters this function at the cursor, runs the remaining instances, and
// then lets Resolve walk SubAbility$ (the DBCleanup tail) exactly once.
// Anything else would ask again on resume or drop the remaining instances.
//
// The instance Remembered set comes from RememberObjects$ (the parent set is
// this resolution's Ctx.Remembered MINUS its trigger capture — the same
// exclusion iterationBase applies and the documented convention that a
// trigger's captured event object is not part of Forge's host remembered
// list; rememberOriginalTokens' tokens are what the flag names, never the
// firing Phase/ChangesZone trigger's own capture):
//
//   - RememberObjects$ Remembered with RememberEach$ True — one instance per
//     parent remembered object, the i-th instance's Ctx.Remembered is exactly
//     the i-th object (DelayTriggerRememberedLKI then resolves to it for the
//     body's ChangeZone AttachedTo$ / Attach Object$ reads), and the instance
//     count clamps to the remembered count when TriggerAmount$ exceeds it
//     (never index-past-end);
//   - RememberObjects$ Remembered / RememberedLKI (or any Remembered...
//     predicate form) without RememberEach$ — every instance sees the whole
//     parent set;
//   - any other RememberObjects$ value — ONE loud Note naming the value, then
//     the whole parent set (never silence);
//   - absent — the whole parent set.
//
// TriggerAmount$ resolves through the ordinary Num grammar against a ctx
// whose Remembered is the capture-excluded parent set (so Remembered$Amount
// and Count$RememberedNumber count exactly the tokens RememberOriginalTokens$
// remembered, not the capture), defaulting to 1. Missing Execute$ is a loud
// Note and a no-op. Conditions gate free (effects.Resolve's shared
// conditionMet pass), and the DB forms' UnlessCost$ gates free through the
// shared unless gate; the AB forms' Cost$ is paid by rules' triggered-cost
// window before this function ever runs (rules/stack.go's resolveTop gate).
func effImmediateTrigger(h Host, c *Ctx, sa *cards.SA) {
	execName := strings.TrimSpace(sa.Params["Execute"])
	sub := cards.ResolveSVar(c.SVars, execName)
	if sub == nil {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ImmediateTrigger with no resolvable Execute$ " + execName})
		return
	}
	// The parent remembered set minus the trigger's own event capture.
	var parent []state.Target
	captured := map[state.Target]bool{}
	for _, t := range c.Captured {
		captured[t] = true
	}
	for _, t := range c.Remembered {
		if !captured[t] {
			parent = append(parent, t)
		}
	}
	// TriggerAmount$ against the capture-excluded ctx, so the Remembered$-
	// anchored count heads see exactly what the script remembered.
	amountCtx := *c
	amountCtx.Remembered = append([]state.Target(nil), parent...)
	amount := Num(h, &amountCtx, sa, "TriggerAmount", 1)
	if amount < 0 {
		amount = 0
	}
	remember := strings.TrimSpace(sa.Params["RememberObjects"])
	each := strings.EqualFold(strings.TrimSpace(sa.Params["RememberEach"]), "True")
	// subjects is the per-instance index space; for a RememberEach loop it
	// holds one entry per instance's remembered object, otherwise amount
	// identical slots that all share the whole parent set.
	var subjects []state.Target
	eachMode := false
	switch {
	case each && (remember == "" || strings.HasPrefix(remember, "Remembered")):
		n := amount
		if n > int32(len(parent)) {
			n = int32(len(parent))
		}
		subjects = append(subjects, parent[:n]...)
		eachMode = true
	case remember == "" || remember == "RememberedLKI" || strings.HasPrefix(remember, "Remembered"):
		for i := int32(0); i < amount; i++ {
			subjects = append(subjects, state.Target{})
		}
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ImmediateTrigger RememberObjects$ " + remember + " is not implemented; every instance sees the whole remembered set"})
		for i := int32(0); i < amount; i++ {
			subjects = append(subjects, state.Target{})
		}
	}
	start := 0
	if cur := c.Repeat; cur != nil && cur.SA == sa {
		// Re-entry after an instance suspended: continue with the subjects
		// captured when the loop started, after the one that asked.
		c.Repeat = nil
		subjects, start = cur.Subjects, cur.Next
		if eachMode && start > len(subjects) {
			start = len(subjects)
		}
	}
	for i := start; i < len(subjects); i++ {
		cc := *c
		cc.Repeat = nil
		if eachMode {
			// The i-th instance remembers exactly its own subject (the token
			// DelayTriggerRememberedLKI names).
			cc.Remembered = []state.Target{subjects[i]}
		} else {
			cc.Remembered = copyTargets(parent)
		}
		Resolve(h, &cc, sub)
		if h.Suspended() {
			h.SuspendRepeat(RepeatSuspension{
				RepeatCursor: RepeatCursor{SA: sa, Subjects: copyTargets(subjects), Next: i + 1},
				Body:         copyTargets(cc.Remembered),
				Subject:      subjects[i],
				Outer:        copyTargets(c.Remembered),
			})
			return
		}
	}
}
