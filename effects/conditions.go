package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

// conditions.go implements the ONE Condition* shape this build gates a
// sub-ability on: `ConditionDefined$ Remembered` + optional
// `ConditionPresent$ <spec>` + optional `ConditionCompare$ <op><n>` —
// Delver of Secrets' "transform only if the revealed card was an instant or
// sorcery" shape, and the same shape the Kinship family gates on. Forge
// spells the compare without a space ("EQ1"; 793 corpus lines) and the
// brief's "EQ 1" spelling is accepted too.
//
// The gate scopes to the DEFINED group the brief authorized: the
// ConditionDefined$ value must be Remembered (the revealed/captured objects
// the resolving walk carries). A ConditionPresent$ WITHOUT a
// ConditionDefined$ is NOT implemented — Forge's default group there is the
// whole battlefield (408 raw corpus lines carry Present-without-Defined,
// re-measured at the worktree pin), which is a different, corpus-wide
// grammar round 1 built without authorization and round 2 removed (see the
// report); those subs run UNCONDITIONALLY, the pre-gate behaviour.
//
// Deliberate scope (task fb-3f1cc033): the wider Condition vocabulary —
// ConditionCheckSVar$ (802 lines), ConditionSVarCompare$ (705),
// Condition$ (577), ConditionZone$ (57), ConditionManaSpent$ (34), the
// other ConditionDefined$ values (Targeted 161, ChosenCard 90, Self 72,
// Imprinted 34, ...), and ConditionNotPresent$ (8) — is NOT implemented. A
// sub carrying any of those is UNRESOLVED: conditionMet reports
// resolved=false and Resolve's walk runs the sub UNCONDITIONALLY, exactly
// as it did before this file existed. That is the documented (not
// fail-closed) choice: fail-closed skipping would change the behaviour of
// cards whose gates name specs this build cannot evaluate (Fatal Push's
// Creature.cmcLEX, Molten Rain's Land.Basic on a Targeted defined group)
// in ways no test asked for, while ungated preserves every observable
// behaviour except the one shape the supported set covers. Every
// unresolved key family is listed in the task report's Issues section.
//
// The sacrifice chooser additionally needs one narrower bridge for a
// post-sacrifice loop body: a `Defined$ Player.IsRemembered` effect — or its
// `Defined$ You` continuation — whose SVar body is `Remembered$Valid
// <known-spec>`. Braids uses it to distinguish an
// opponent who took the optional sacrifice from one who declined. This is
// deliberately not general ConditionCheckSVar grammar: it does not evaluate
// PlayerCount, Count$, literals, or any other SVar head, all of which remain
// unresolved and retain the prior unconditional walk.

// conditionMet evaluates sa's Condition* gate against the resolving
// context. It returns (met, resolved):
//
//   - (…, true)  the gate is one of the supported shapes and met says
//     whether the sub should run;
//   - (…, false) the gate carries something unsupported — the caller runs
//     the sub anyway (the pre-condition-engine behaviour, documented in
//     the task report). A ConditionPresent$ with NO ConditionDefined$ is in
//     this class on purpose: its default group is the battlefield, a
//     grammar this task did not authorize.
//   - a sub with no Condition* key at all is not gated: (true, false).
func conditionMet(h Host, c *Ctx, sa *cards.SA) (met bool, resolved bool) {
	defined := strings.TrimSpace(sa.Params["ConditionDefined"])
	present := strings.TrimSpace(sa.Params["ConditionPresent"])
	notPresent := strings.TrimSpace(sa.Params["ConditionNotPresent"])
	compare := strings.TrimSpace(sa.Params["ConditionCompare"])
	checkSVar := strings.TrimSpace(sa.Params["ConditionCheckSVar"])
	if defined == "" && present == "" && notPresent == "" && compare == "" && checkSVar == "" {
		return true, false // not gated
	}
	if checkSVar != "" {
		return rememberedSacrificeCondition(h, c, sa, defined, present, notPresent, compare, checkSVar)
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
		// ConditionPresent$ (or Compare$) with NO ConditionDefined$: the
		// default group is the battlefield, a corpus-wide grammar (408 raw
		// lines, re-measured) round 1 built without authorization and round
		// 2 removed — resolving it here changed unrelated cards' behaviour
		// through the global Resolve hook. UNRESOLVED: the sub runs
		// unconditionally, the pre-gate behaviour, listed in the report's
		// Issues section.
		return false, false
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
	if compare == "" {
		// Forge's default for a Present/Defined gate with no Compare is
		// presence: at least one must match.
		return count >= 1, true
	}
	op, n, ok := parseConditionCompare(compare)
	if !ok {
		// A non-literal right-hand side (GTX/EQY — 5 corpus lines) needs the
		// SVar resolver this gate does not carry. Unresolved.
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

// rememberedSacrificeCondition is the one ConditionCheckSVar shape required
// by a Sacrifice loop's continuation. It deliberately accepts only the
// `Defined$ Player.IsRemembered` effect (or its `Defined$ You`
// continuation) and the Remembered$Valid SVar head, so unrelated conditions (notably Vampire Lacerator's
// PlayerCountOpponents$LowestLifeTotal) keep their pre-task
// unresolved/unconditional behaviour.
func rememberedSacrificeCondition(h Host, c *Ctx, sa *cards.SA, defined, present, notPresent, compare, checkSVar string) (bool, bool) {
	definedPlayer := strings.TrimSpace(sa.Params["Defined"])
	if defined != "" || (definedPlayer != "Player.IsRemembered" && definedPlayer != "You") || present != "" || notPresent != "" || c.SVars == nil {
		return false, false
	}
	for k := range sa.Params {
		if !strings.HasPrefix(k, "Condition") || k == "ConditionDescription" {
			continue
		}
		switch k {
		case "ConditionCheckSVar", "ConditionSVarCompare":
		default:
			return false, false
		}
	}
	body, ok := c.SVars[checkSVar]
	body = strings.TrimSpace(body)
	const head = "Remembered$Valid "
	if !ok || !strings.HasPrefix(body, head) {
		return false, false
	}
	spec := strings.TrimSpace(strings.TrimPrefix(body, head))
	if spec == "" || len(UnknownPredicates(spec)) != 0 {
		return false, false
	}
	op, n := "GE", 1
	if raw := strings.TrimSpace(sa.Params["ConditionSVarCompare"]); raw != "" {
		var valid bool
		op, n, valid = parseConditionCompare(raw)
		if !valid {
			return false, false
		}
	}
	value := EvalCount(h, c, body)
	switch op {
	case "EQ":
		return value == int32(n), true
	case "NE":
		return value != int32(n), true
	case "LT":
		return value < int32(n), true
	case "LE":
		return value <= int32(n), true
	case "GT":
		return value > int32(n), true
	case "GE":
		return value >= int32(n), true
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
