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
// Condition$ (577 raw lines), ConditionZone$ (57), ConditionManaSpent$ (34),
// the other ConditionDefined$ values (Targeted 161, ChosenCard 90, Self 72,
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
// ConditionCheckSVar$/ConditionSVarCompare$ ARE implemented (added with the
// unless-cost task): the SVar name — an SVar body the count evaluator
// knows, an inline $ expression, or a signed literal — is compared under
// <op><operand> (Forge's default GE1), and an unknown name stays
// UNRESOLVED (run-unconditionally), never a silent 0. This is what makes
// Vampire Lacerator's "you lose 1 life unless an opponent has 10 or less
// life" (ConditionCheckSVar$ OpponentSmallest | ConditionSVarCompare$ GE11)
// gate correctly.

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
	checkSVarParam := strings.TrimSpace(sa.Params["ConditionCheckSVar"])
	if defined == "" && present == "" && notPresent == "" && compare == "" && checkSVarParam == "" {
		return true, false // not gated
	}
	// Any other Condition* key (Zone, ManaSpent, PlayerTurn, ...) beside the
	// supported six makes the shape unsupported. ConditionDescription$ is
	// display text, not part of the evaluation, and is ignored.
	for k := range sa.Params {
		if !strings.HasPrefix(k, "Condition") || k == "ConditionDescription" {
			continue
		}
		switch k {
		case "ConditionDefined", "ConditionPresent", "ConditionNotPresent", "ConditionCompare",
			"ConditionCheckSVar", "ConditionSVarCompare":
		default:
			return false, false
		}
	}
	// The ConditionCheckSVar$/ConditionSVarCompare$ gate (802/705 raw corpus
	// lines): the SVar name is resolved through the count evaluator and
	// compared under <op><operand> — Forge's SpellAbilityCondition default is
	// GE 1 when no ConditionSVarCompare$ is written, and the operand is itself
	// SVar-resolvable (Braids, Arisen Nightmare's "ConditionCheckSVar$ X |
	// ConditionSVarCompare$ EQ0" where SVar:X:Remembered$Valid
	// Card.RememberedPlayerCtrl answers "did any remembered opponent sacrifice
	// this way"). An SVar name that resolves to nothing and is not a literal
	// keeps the unresolved (run-unconditionally) behaviour, never a silent 0.
	if checkSVar := checkSVarParam; checkSVar != "" {
		value, ok := resolveSVarCount(h, c, checkSVar)
		if !ok {
			return false, false
		}
		op, operand := "GE", "1"
		if cmp := strings.TrimSpace(sa.Params["ConditionSVarCompare"]); cmp != "" {
			if len(cmp) < 3 {
				return false, false
			}
			op, operand = cmp[:2], cmp[2:]
		}
		rhs, ok := resolveSVarCount(h, c, operand)
		if !ok {
			return false, false
		}
		switch op {
		case "EQ":
			if value != rhs {
				return false, true
			}
		case "NE":
			if value == rhs {
				return false, true
			}
		case "GE":
			if value < rhs {
				return false, true
			}
		case "GT":
			if value <= rhs {
				return false, true
			}
		case "LE":
			if value > rhs {
				return false, true
			}
		case "LT":
			if value >= rhs {
				return false, true
			}
		default:
			return false, false
		}
		// A sub gated ONLY by the SVar check (the corpus's dominant shape) is
		// fully resolved here; the Defined/Present grammar below does not
		// apply to it.
		if defined == "" && present == "" && notPresent == "" {
			return true, true
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

// resolveSVarCount resolves a ConditionCheckSVar$ name (or a
// ConditionSVarCompare$ operand) to its count. A signed literal passes
// through; an SVar name resolves through the resolving card's table and its
// body through the count evaluator (Remembered$Valid Card.RememberedPlayerCtrl,
// Count$..., Sacrificed$...); anything else is offered to the evaluator as an
// inline expression. ok is false only for a name that is neither a literal
// nor a table entry (the unresolved, run-anyway behaviour).
func resolveSVarCount(h Host, c *Ctx, name string) (int32, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(name); err == nil {
		return int32(n), true
	}
	if c.SVars != nil {
		if body, ok := c.SVars[name]; ok {
			return EvalCount(h, c, body), true
		}
	}
	// An inline Count$/Remembered$/Sacrificed$ expression is its own body.
	if strings.Contains(name, "$") {
		return EvalCount(h, c, name), true
	}
	return 0, false
}
