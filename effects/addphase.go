package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("AddPhase", effAddPhase)
}

// delayedTriggerSpec resolves a DelayedTrigger DEFINITION SVar (the
// ExtraTurnDelayedTrigger$/ExtraPhaseDelayedTrigger$ value) to its Phase$
// step and ValidPlayer$ value. Two corpus shapes: the raw
// "Mode$ Phase | Phase$ ..." body (Final Fortune, Moraug -- parseSA cannot
// read it, a definition body has no DB$ head, so cards.ResolveSVar misses
// it and cards.ParseTriggerLine is the reader that shape needs) and the
// keyword expansions' "DB$ DelayedTrigger | Mode$ Phase | ..." body
// (ResolveSVar reads it). ok=false leaves the caller's fallback -- the
// registration the consumer side makes without a phase, never a wrong one.

func delayedTriggerSpec(c *Ctx, name string) (state.Step, string, bool) {
	if c == nil || c.SVars == nil || name == "" {
		return 0, "", false
	}
	if sa := cards.ResolveSVar(c.SVars, name); sa != nil {
		return parseDelayedTriggerBody(sa.Params["Phase"], sa.Params["ValidPlayer"])
	}
	raw, found := c.SVars[name]
	if !found {
		return 0, "", false
	}
	if trig, ok := cards.ParseTriggerLine(raw); ok {
		return parseDelayedTriggerBody(trig.Params["Phase"], trig.Params["ValidPlayer"])
	}
	return 0, "", false
}

// parseDelayedTriggerBody resolves a DelayedTrigger body's Phase$ through
// the ONE shared phase-name parser; ok=false on anything unparseable.
func parseDelayedTriggerBody(phase, vp string) (state.Step, string, bool) {
	set, unknown := state.ParsePhases(phase)
	if len(unknown) > 0 || set.Empty() {
		return 0, "", false
	}
	return set.Steps()[0], strings.TrimSpace(vp), true
}

// effAddPhase implements DB$ AddPhase (Forge's AddPhaseEffect; 56 corpus SA
// lines across 56 files: "after this phase, there is an additional combat
// phase" -- Aurelia the Warleader, Aggravated Assault, Moraug, Fury of
// Akoum, Éomer, Marshal of Rohan, Obeka, Splitter of Seconds, ...). The
// grant is one events.ExtraPhase (+1) whose fold is state.Game.ExtraPhases
// (state/game.go); the turn structure (rules/turn.go's advanceStep tail)
// is the consumer, the ExtraTurn/ExtraTurnQueue precedent one level up:
//
//   - ExtraPhase$ names the extra phase: Combat (48 corpus lines) is the
//     whole combat (BeginCombat..EndCombat), Beginning (4) the whole
//     beginning phase (Untap..Draw -- the added untap, upkeep AND draw all
//     run, Shadow of the Second Sun's oracle), Upkeep (3) and End of Turn
//     (1) single steps. Any other value -- or an unresolvable one -- is a
//     loud Note and no grant.
//   - AfterPhase$ names the splice point (EndCombat 34, "End of Turn" 1);
//     when omitted the grant splices after the phase it resolves in, so the
//     granting step rides the event (Step) and replay folds the same splice
//     the live game makes. An unparseable value is a loud Note and the
//     omitted form (the current step) stands in.
//   - FollowedBy$ (Main2, 12 lines -- the Aggravated Assault "followed by an
//     additional main phase" family) names the resume point after the extra
//     phase completes. Default (Forge's): the phase that would naturally
//     have followed the splice point (AfterStep+1) -- which is what makes
//     an extra Beginning spliced after Main2 resume at the END STEP, never
//     back into Main1. An unparseable value is a loud Note and the default
//     stands in.
//   - NumPhases$ (Obeka's "you get that many additional upkeep steps",
//     TriggerCount$DamageAmount) is the grant count through the ordinary Num
//     SVar indirection; an unresolvable value is a loud Note and grants one.
//   - SubAbility$ chains through the ordinary effect walk (free).
//   - ConditionPhases$ / ConditionPlayerTurn$ / ConditionFirstCombat$ and
//     the Present/Defined/Compare/SVar family are the shared condition gate
//     (effects/conditions.go), run by the dispatch before this effect.
//
// The ExtraPhaseDelayedTrigger$/ExtraPhaseDelayedTriggerExcute$ pair
// (Moraug's "At the beginning of that combat, untap all creatures you
// control"; note the corpus's "Excute" typo -- read verbatim) forwards the
// delayed trigger on the grant event the way effAddTurn forwards Final
// Fortune's: the DelTrig SVar's Phase$ and ValidPlayer$ are parsed through
// delayedTriggerSpec (the ONE shared definition-body parser) and ride the
// grant Text's ExtraPhaseRiders marker, the Execute$ name rides Counter;
// events.Apply registers the one-shot delayed trigger at CONSUME time with
// MinTurn = the current turn, so it fires exactly once, at the extra
// phase's entry step. An unresolvable DelTrig SVar or an unparseable Phase$
// is a loud Note and the grant still applies without the rider.
func effAddPhase(h Host, c *Ctx, sa *cards.SA) {
	entry, ok := parseExtraPhaseValue(sa.Params["ExtraPhase"])
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented ExtraPhase$ value " + sa.Params["ExtraPhase"]})
		return
	}
	n := int32(1)
	if raw, present := sa.Params["NumPhases"]; present && strings.TrimSpace(raw) != "" {
		if v, resolved := NumResolved(h, c, sa, "NumPhases", 1); resolved {
			n = v
		} else {
			// The documented loud-degrade convention: one Note naming the
			// param, the grant still applied (as one).
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unresolvable NumPhases$ " + strings.TrimSpace(raw) + " (granting one)"})
		}
	}
	if n <= 0 {
		return
	}

	g := h.Game()
	after := g.Step
	if raw := strings.TrimSpace(sa.Params["AfterPhase"]); raw != "" {
		if set, unknown := state.ParsePhases(raw); len(unknown) == 0 && !set.Empty() {
			if steps := set.Steps(); len(steps) == 1 {
				after = steps[0]
			} else {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "multi-step AfterPhase$ " + raw + " (splicing after the current phase)"})
			}
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unparseable AfterPhase$ " + raw + " (splicing after the current phase)"})
		}
	}

	ids := []state.ObjID{state.ObjID(entry)}
	if raw := strings.TrimSpace(sa.Params["FollowedBy"]); raw != "" {
		if set, unknown := state.ParsePhases(raw); len(unknown) == 0 && !set.Empty() {
			if steps := set.Steps(); len(steps) == 1 {
				ids = append(ids, state.ObjID(steps[0]))
			} else {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "multi-step FollowedBy$ " + raw + " (resuming after the splice point)"})
			}
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unparseable FollowedBy$ " + raw + " (resuming after the splice point)"})
		}
	}

	text := ""
	counter := ""
	if name := strings.TrimSpace(sa.Params["ExtraPhaseDelayedTrigger"]); name != "" {
		if phase, vp, ok := delayedTriggerSpec(c, name); ok {
			// The Text marker carries the forwarded delayed phase (the
			// consume-time registration reads it); the Execute$ name rides
			// Counter and a ValidPlayer$ rides the marker's VP= part.
			text = events.EncodeExtraPhaseRiders(events.ExtraPhaseRiders{
				HasDelayedPhase: true, DelayedPhase: phase, ValidPlayer: vp})
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unparseable ExtraPhaseDelayedTrigger$ " + name + " (no delayed trigger)"})
		}
	}
	// The corpus writes the Execute name under the misspelled parameter
	// ("Excute"); read it verbatim, like Forge's own AddPhaseEffect does.
	if name := strings.TrimSpace(sa.Params["ExtraPhaseDelayedTriggerExcute"]); name != "" {
		counter = name
	}

	h.Emit(events.Event{Kind: events.ExtraPhase, Player: c.Controller, Obj: c.Source,
		Amount: n, Step: after, Counter: counter, Text: text, IDs: ids})
}

// parseExtraPhaseValue resolves a Forge ExtraPhase$ value to the extra
// phase's entry step (state.ExtraPhaseRangeEnd derives the range end). The
// two compound values -- Combat (the whole combat) and Beginning (the whole
// beginning phase) -- are not Forge phase NAMES (state.ParsePhases does not
// know bare "Combat"), so they map here; every other value goes through the
// ONE shared phase-name parser and must be a singleton (Upkeep, End of
// Turn, ...).
func parseExtraPhaseValue(v string) (state.Step, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "combat":
		return state.StepBeginCombat, true
	case "beginning":
		return state.StepUntap, true
	}
	set, unknown := state.ParsePhases(v)
	if len(unknown) > 0 || set.Empty() {
		return 0, false
	}
	steps := set.Steps()
	if len(steps) != 1 {
		return 0, false
	}
	return steps[0], true
}
