package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// conditions.go implements the TWO Condition* shapes this build gates a
// sub-ability on:
//
//  1. `ConditionDefined$ Remembered` + optional `ConditionPresent$ <spec>` +
//     optional `ConditionCompare$ <op><n>` — Delver of Secrets' "transform
//     only if the revealed card was an instant or sorcery" shape (task
//     fb-3f1cc033).
//  2. `ConditionPresent$ <spec>` with NO `ConditionDefined$`, whose Forge
//     default group is the whole battlefield — the conditional
//     enters-tapped land family (Blooming Marsh and the fast / check lands,
//     task fb-9d2338cc: `Land.YouCtrl` GT2 / `Plains.YouCtrl,Island.YouCtrl`
//     EQ0). See conditionMetBattlefield for the entering-object exclusion
//     that makes "two or fewer OTHER lands" mean what the oracle says.
//
// Forge spells the compare without a space ("EQ1"; 793 corpus lines) and the
// brief's "EQ 1" spelling is accepted too.
//
// Deliberate scope (task fb-3f1cc033): the wider Condition vocabulary —
// ConditionCheckSVar$ (802 lines), ConditionSVarCompare$ (705),
// Condition$ (577), ConditionZone$ (57), ConditionManaSpent$ (34), the
// other ConditionDefined$ values (Targeted 161, ChosenCard 90, Self 72,
// Imprinted 34, ...), a bare ConditionCompare$ with no group, and
// ConditionNotPresent$ (8) — is NOT implemented. A sub carrying any of
// those is UNRESOLVED: conditionMet reports resolved=false and Resolve's
// walk runs the sub UNCONDITIONALLY, exactly as it did before this file
// existed. That is the documented (not fail-closed) choice: fail-closed
// skipping would change the behaviour of cards whose gates name specs this
// build cannot evaluate (Fatal Push's Creature.cmcLEX, Molten Rain's
// Land.Basic on a Targeted defined group) in ways no test asked for, while
// ungated preserves every observable behaviour except the shapes the
// supported set covers. Every unresolved key family is listed in the task
// report's Issues section.

// conditionMet evaluates sa's Condition* gate against the resolving
// context. It returns (met, resolved):
//
//   - (…, true)  the gate is one of the supported shapes and met says
//     whether the sub should run;
//   - (…, false) the gate carries something unsupported — the caller runs
//     the sub anyway (the pre-condition-engine behaviour, documented in
//     the task report). A ConditionNotPresent$, a non-Remembered
//     ConditionDefined$, a bare ConditionCompare$ with no group, and a
//     ConditionCheckSVar$/ConditionSVarCompare$/Condition$/… key are all in
//     this class.
//   - a sub with no Condition* key at all is not gated: (true, false).
func conditionMet(h Host, c *Ctx, sa *cards.SA) (met bool, resolved bool) {
	defined := strings.TrimSpace(sa.Params["ConditionDefined"])
	present := strings.TrimSpace(sa.Params["ConditionPresent"])
	notPresent := strings.TrimSpace(sa.Params["ConditionNotPresent"])
	compare := strings.TrimSpace(sa.Params["ConditionCompare"])
	if defined == "" && present == "" && notPresent == "" && compare == "" {
		return true, false // not gated
	}
	// Any other Condition* key (CheckSVar, SVarCompare, Zone, ManaSpent,
	// PlayerTurn, ...) beside the supported four makes the shape
	// unsupported. ConditionDescription$ is display text, not part of the
	// evaluation, and is ignored.
	for k := range sa.Params {
		if !strings.HasPrefix(k, "Condition") || k == "ConditionDescription" {
			continue
		}
		switch k {
		case "ConditionDefined", "ConditionPresent", "ConditionNotPresent", "ConditionCompare":
		default:
			return false, false
		}
	}
	if notPresent != "" {
		// Count-must-be-zero is the mirror of Present; 8 corpus lines carry
		// it and none is the shape this file scopes. Unresolved.
		return false, false
	}
	if defined == "" {
		// ConditionPresent$ with NO ConditionDefined$: Forge's default group
		// is the battlefield (the conditional enters-tapped land family —
		// fast lands' Land.YouCtrl GT2, check lands' Plains.YouCtrl,Island.
		// YouCtrl EQ0). This is the shape task fb-9d2338cc resolves; it was
		// previously removed (round 2) for changing unrelated cards through
		// the global Resolve hook, and re-authorizing it is this task's
		// scope. The battlefield scan, including the entering-object
		// exclusion, lives in conditionMetBattlefield.
		//
		// A BARE ConditionCompare$ (no Present, no Defined) names no count
		// group here, so it stays unresolved.
		if present == "" {
			return false, false
		}
		return conditionMetBattlefield(h, c, present, compare)
	}
	if defined != "Remembered" {
		// Only the Remembered family is in scope among DEFINED groups: the
		// revealed/captured objects a walk carries in Ctx.Remembered.
		// Targeted, ChosenCard, Self, Imprinted and the rest need Ctx state
		// this gate does not model (and whose fail-closed skip would change
		// unrelated cards).
		return false, false
	}
	g := h.Game()
	sc := c.SpecContext(c.Controller)
	count := 0
	for _, t := range c.Remembered {
		if t.IsPlayer {
			// A Card spec never matches a player entry; skip rather than
			// hand MatchesObjectCtx an object-less target.
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil {
			continue
		}
		if present == "" {
			count++
			continue
		}
		if MatchesObjectCtx(g, present, o, sc) {
			count++
		}
	}
	if present != "" {
		// An unknown predicate in the spec cannot be evaluated: the whole
		// gate is unresolved rather than counting a false-negative zero
		// (which would silently stop subs that used to run — Skyclave
		// Apparition's Card.ExiledWithSource). Checked once, outside the
		// object loop: an EMPTY remembered set with an unreadable spec must
		// also be unresolved, not a resolved "count 0".
		if len(UnknownPredicates(present)) > 0 {
			return false, false
		}
	}
	return evalConditionCount(count, compare)
}

// conditionMetBattlefield resolves a ConditionPresent$ (with an optional
// ConditionCompare$ but NO ConditionDefined$) against Forge's default group
// for that shape: every permanent on the battlefield. Task fb-9d2338cc (the
// conditional enters-tapped land family — Blooming Marsh and the fast / check
// lands) is what needs it.
//
// The count EXCLUDES the entering object (c.Replaced). For the
// ReplacementResult$ Updated shape rules/replacement.go emits the original
// MoveZone (the land enters the battlefield) BEFORE resolving the With, so
// when conditionMet runs here the entering land is already a battlefield
// permanent; Forge evaluates the gate before entry, so Land.YouCtrl there
// means "other lands". Without the exclusion a naive count would include the
// land itself and fire the fast lands' GT2 one land early (tapped at 2 other
// lands instead of 3). c.Replaced names exactly that moved object (and is 0
// outside a replacement, where nothing is excluded). c.Source is deliberately
// NOT excluded: it is the permanent owning the replacement, which for the
// shared "other permanents enter tapped" shapes (Blind Obedience) is a DIFFERENT
// object that the count must still see.
//
// An unreadable spec (UnknownPredicates) is unresolved, counted once before
// the object loop, mirroring the Remembered path: an empty battlefield with
// an unknown spec must be unresolved, not a resolved "count 0" that silently
// stops subs that used to run.
func conditionMetBattlefield(h Host, c *Ctx, present, compare string) (met, resolved bool) {
	if len(UnknownPredicates(present)) > 0 {
		return false, false
	}
	g := h.Game()
	sc := c.SpecContext(c.Controller)
	count := 0
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		if c.Replaced != 0 && o.ID == c.Replaced {
			// The entering object is already a battlefield permanent (see the
			// doc above): it is not an "other" permanent and must not count.
			continue
		}
		if MatchesObjectCtx(g, present, o, sc) {
			count++
		}
	}
	return evalConditionCount(count, compare)
}

// evalConditionCount turns a counted group into (met, resolved) from the
// ConditionCompare$ operator, sharing the comparison logic between the
// Remembered and battlefield groups. No Compare key means presence (>= 1),
// Forge's default; a non-literal right-hand side (GTX/EQY — 5 corpus lines)
// needs the SVar resolver this gate does not carry and stays unresolved.
func evalConditionCount(count int, compare string) (met, resolved bool) {
	if compare == "" {
		return count >= 1, true
	}
	op, n, ok := parseConditionCompare(compare)
	if !ok {
		return false, false
	}
	switch op {
	case "EQ":
		return count == n, true
	case "NE":
		return count != n, true
	case "LT":
		return count < n, true
	case "LE":
		return count <= n, true
	case "GT":
		return count > n, true
	case "GE":
		return count >= n, true
	}
	return false, false
}

// parseConditionCompare parses Forge's ConditionCompare$ value: a
// two-letter operator immediately followed by an integer, with or without
// an intervening space ("EQ1" — the corpus spelling, 793 lines — or
// "EQ 1"). Unknown operators or non-literal right-hand sides fail.
func parseConditionCompare(v string) (op string, n int, ok bool) {
	if len(v) < 3 {
		return "", 0, false
	}
	op = strings.ToUpper(v[:2])
	switch op {
	case "EQ", "NE", "LT", "LE", "GT", "GE":
	default:
		return "", 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(v[2:]))
	if err != nil {
		return "", 0, false
	}
	return op, n, true
}
