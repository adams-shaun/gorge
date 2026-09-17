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
// Two more shapes landed with the action-level SVar-condition task (the
// paramcensus Draw/LoseLife/ChangeZone/Reveal/DealDamage cluster):
//
//  3. `ConditionCheckSVar$` + optional `ConditionSVarCompare$` — the SVar
//     gate (544 corpus SAs carry the pair): the named SVar (or inline
//     Count$-style expression) is evaluated with EvalCount and compared
//     under SVarCompare$ ("<op><threshold>", threshold optionally an SVar
//     name). No SVarCompare$ means "nonzero", Forge's default truthiness
//     read. The shared evaluator is CheckSVarHolds below — the same one the
//     statics' CheckSVar$ (rules.Engine.checkSVarHolds) and the
//     ability-offer gate (rules/legal.go sVarGateOK) delegate to. The FAIL
//     DIRECTION on an unmodelled count body is the CALLER's, not this
//     file's: CheckSVarHolds reports evaluated=false and each of the three
//     call sites documents its own choice — conditionMet and the offer gate
//     fail OPEN (run-anyway), the statics wrapper fails CLOSED.
//  4. a bare `Condition$` whose value is `Kicked` — the source was cast
//     with its Kicker paid (Into the Roil's "If this spell was kicked,
//     draw a card", the corpus's dominant bare-Condition value at 54 SAs).
//     The other bare-Condition values (Delirium, OptionalCost, Bargain,
//     Threshold, Blessing, Metalcraft, Foretold, Hellbent, Revolt, Surge —
//     ~28 SAs) stay unresolved.
//
// Deliberate scope (task fb-3f1cc033): the wider Condition vocabulary —
// Condition$ beyond Kicked, ConditionZone$ (57), ConditionManaSpent$ (34),
// the other ConditionDefined$ values (Targeted 161, ChosenCard 90, Self 72,
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
//
// The sacrifice chooser additionally needs one narrower bridge for a
// post-sacrifice loop body: a `Defined$ Player.IsRemembered` effect — or its
// `Defined$ You` continuation — whose SVar body is `Remembered$Valid
// <known-spec>`. Braids uses it to distinguish an
// opponent who took the optional sacrifice from one who declined. That
// shape is subsumed by the general CheckSVarHolds gate below (the
// Remembered$Valid body evaluates through evalRememberedOK/evalRefProperty),
// so no separate bridge is kept: every Remembered$Valid gate — Braids's
// included — goes through the shared evaluator.

// CheckSVarHolds evaluates Forge's CheckSVar$/SVarCompare$ intervening-if
// gate — the ONE SVar-compare evaluator this build ships, shared by three
// call sites: conditionMet's ConditionCheckSVar$ branch (this file),
// rules.Engine.checkSVarHolds (the statics' CheckSVar$), and rules/legal.go's
// ability-offer gate (an AB's CheckSVar$, Bloodsoaked Champion's Raid). It
// reports (holds, evaluated). holds is the compare's answer;
// evaluated is false when the gate's count body is not one the evaluator
// models (EvalCountOK's verdict) or the compare operator/threshold is
// unreadable — the caller picks its own fail direction for that case, and
// each of the three call sites documents its own:
//
//   - conditionMet (ConditionCheckSVar$, this file): fail OPEN — the gated
//     ability runs, the pre-gate behaviour;
//   - rules/legal.go sVarGateOK (an AB's CheckSVar$ at offer time): fail
//     OPEN — the ability is offered (a gate you cannot read must not
//     silently remove a card's only activation);
//   - rules/statics.go checkSVarHolds (a static's CheckSVar$): fail CLOSED
//     — the shipped statics convention (an unreadable "as long as" gate
//     must not silently always-apply a continuous effect).
//
// The named SVar (c.SVars first, then the source face's own table) or the
// raw inline Count$-style expression is evaluated with EvalCountOK and
// compared under cmp ("<op><threshold>", e.g. GE11 — the threshold may also
// be an SVar name or an inline expression, resolved the same way). No cmp
// means "nonzero" (Forge's default truthiness read).
func CheckSVarHolds(h Host, c *Ctx, check, cmp string) (holds, evaluated bool) {
	check = strings.TrimSpace(check)
	if check == "" {
		return true, false
	}
	if c == nil {
		c = &Ctx{}
	}
	g := h.Game()
	body := check
	if c.SVars != nil {
		if b, ok := c.SVars[check]; ok {
			body = b
		}
	}
	if body == check && c.Source != 0 {
		// The ctx carried no table (or no entry): fall back to the source
		// face's own SVar table, the same lookup rules.Engine.checkSVarHolds
		// builds its ctx around.
		if o := g.Obj(c.Source); o != nil && o.Face() != nil {
			if b, ok := o.Face().SVars[check]; ok {
				body = b
			}
		}
	}
	val, ok := EvalCountOK(h, c, body)
	if !ok {
		return false, false
	}
	cmp = strings.TrimSpace(cmp)
	if cmp == "" {
		return val != 0, true
	}
	if len(cmp) < 3 {
		return false, false
	}
	op, rhs := strings.ToUpper(cmp[:2]), cmp[2:]
	var threshold int32
	switch n, err := strconv.ParseInt(rhs, 10, 64); {
	case err == nil:
		threshold = int32(n)
	case c.SVars != nil:
		if b, found := c.SVars[rhs]; found {
			threshold = EvalCount(h, c, b)
		} else if t, ok2 := EvalCountOK(h, c, rhs); ok2 {
			threshold = t
		} else {
			// A threshold that resolves nowhere: unreadable compare.
			return false, false
		}
	default:
		return false, false
	}
	switch op {
	case "EQ":
		return val == threshold, true
	case "GE":
		return val >= threshold, true
	case "GT":
		return val > threshold, true
	case "LE":
		return val <= threshold, true
	case "LT":
		return val < threshold, true
	}
	return false, false
}

// conditionMet evaluates sa's Condition* gate against the resolving
// context. It returns (met, resolved):
//
//   - (…, true)  the gate is one of the supported shapes and met says
//     whether the sub should run;
//   - (…, false) the gate carries something unsupported — the caller runs
//     the sub anyway (the pre-condition-engine behaviour, documented in
//     the task report). A ConditionNotPresent$, a non-Remembered
//     ConditionDefined$, a bare ConditionCompare$ with no group, a bare
//     Condition$ whose value is not Kicked, and the mixed shapes are all in
//     this class.
//   - a sub with no Condition* key at all is not gated: (true, false).
func conditionMet(h Host, c *Ctx, sa *cards.SA) (met bool, resolved bool) {
	defined := strings.TrimSpace(sa.Params["ConditionDefined"])
	present := strings.TrimSpace(sa.Params["ConditionPresent"])
	notPresent := strings.TrimSpace(sa.Params["ConditionNotPresent"])
	compare := strings.TrimSpace(sa.Params["ConditionCompare"])
	check := strings.TrimSpace(sa.Params["ConditionCheckSVar"])
	svarCmp := strings.TrimSpace(sa.Params["ConditionSVarCompare"])
	bare := strings.TrimSpace(sa.Params["Condition"])
	if defined == "" && present == "" && notPresent == "" && compare == "" && check == "" && bare == "" {
		return true, false // not gated (a lone ConditionSVarCompare$ compares nothing)
	}
	// Any other Condition* key (Zone, ManaSpent, PlayerTurn, ...) beside the
	// supported seven makes the shape unsupported. ConditionDescription$ is
	// display text, not part of the evaluation, and is ignored.
	// (The ConditionCheckSVar$ shape below covers the sacrifice-continuation
	// bridge the pre-merge build carried as rememberedSacrificeCondition:
	// Braids's `Defined$ Player.IsRemembered` legs with a `Remembered$Valid`
	// SVar body evaluate through the same shared gate.)
	for k := range sa.Params {
		if !strings.HasPrefix(k, "Condition") || k == "ConditionDescription" {
			continue
		}
		switch k {
		case "ConditionDefined", "ConditionPresent", "ConditionNotPresent", "ConditionCompare",
			"ConditionCheckSVar", "ConditionSVarCompare", "Condition":
		default:
			return false, false
		}
	}
	// The SVar gate (ConditionCheckSVar$ + optional ConditionSVarCompare$,
	// Forge's dominant pair at 544 corpus SAs): Vampire Lacerator's upkeep
	// trigger, Braids's per-opponent lose-life/draw, Electrostatic Bolt's
	// artifact-creature modal split, Victimize's "If you do", Infernal
	// Tutor's hellbent legs. Enforced only when the SVar gate is the SA's
	// ONLY condition — a CheckSVar beside a Present/Defined/Compare group
	// (~18 corpus SAs) has no single evaluator and stays unsupported, the
	// same fail-open run-anyway the other unsupported shapes take.
	if check != "" {
		if defined != "" || present != "" || notPresent != "" || compare != "" || bare != "" {
			return false, false
		}
		holds, evaluated := CheckSVarHolds(h, c, check, svarCmp)
		if !evaluated {
			// The gate's count body is not one the evaluator models: fail
			// OPEN, the same run-anyway the other unsupported Condition*
			// shapes take (the doc above). Enforcing a zero would silence
			// cards whose gates name counts this build cannot compute
			// (Sephiroth's Count$ResolvedThisTurn transform, 52 corpus
			// sites' worth of families).
			return false, false
		}
		return holds, true
	}
	// A bare Condition$ is the cast-option family: only Kicked is evaluated
	// (the source's CastFlags FlagKicked, the same bit the Kicker payment
	// recorded); Delirium/OptionalCost/Bargain/Threshold/Blessing/Metalcraft/
	// Foretold/Hellbent/Revolt/Surge stay unresolved and run unconditionally.
	// A bare Condition beside a group key or beside ConditionSVarCompare$ is
	// a mixed shape no single evaluator covers (~11 corpus SAs).
	if bare != "" {
		if defined != "" || present != "" || notPresent != "" || compare != "" || svarCmp != "" {
			return false, false
		}
		if !strings.EqualFold(bare, "Kicked") {
			return false, false
		}
		o := h.Game().Obj(c.Source)
		return o != nil && o.CastFlags&state.FlagKicked != 0, true
	}
	if notPresent != "" {
		// ConditionNotPresent$ (8 corpus lines, two shapes): met when NO object
		// matching the spec is present in the gate's group. With a
		// ConditionDefined$ group (Ajani Sleeper Agent, Ravenous Gigamole,
		// Fallaji Archeologist: ConditionDefined$ Remembered | NotPresent$ Card)
		// the group is that remembered list; without one the group is the
		// battlefield, the escape shape (Kroxa/Uro/Phlage's TrigSac spec
		// Card.Self+escaped: the entered permanent matches exactly when it
		// did NOT escape, so "sacrifice it unless it escaped" runs only for a
		// non-escape entry). A second present/compare key beside NotPresent is
		// not a corpus shape; a defined group this build cannot enumerate
		// (Targeted, TriggeredCardLKICopy) or an unknown predicate in the spec
		// stays unresolved and runs the sub unconditionally, the documented
		// pre-condition-engine behaviour.
		if present != "" || compare != "" {
			return false, false
		}
		return conditionNotPresentMet(h, c, defined, notPresent)
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
	for _, t := range rememberedWithSource(h, c) {
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

// conditionNotPresentMet is the ConditionNotPresent$ evaluator the two
// supported group shapes share (see conditionMet's notPresent branch for the
// shape census). The spec is matched with MatchesObjectCtx -- the same object
// grammar ValidCard$ uses -- so the escape family's Card.Self+escaped reads
// the CastFlags FlagEscaped provenance the escape cast recorded.
func conditionNotPresentMet(h Host, c *Ctx, defined, spec string) (met, resolved bool) {
	if len(UnknownPredicates(spec)) > 0 {
		return false, false
	}
	g := h.Game()
	sc := c.SpecContext(c.Controller)
	count := 0
	switch defined {
	case "":
		// Forge's default group for a group-less ConditionPresent$/NotPresent$
		// gate is the battlefield (conditionMetBattlefield's group).
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone != state.ZBattlefield || o.Face() == nil {
				continue
			}
			if MatchesObjectCtx(g, spec, o, sc) {
				count++
			}
		}
	case "Remembered":
		for _, t := range rememberedWithSource(h, c) {
			if t.IsPlayer {
				continue
			}
			o := g.Obj(t.Obj)
			if o == nil {
				continue
			}
			if MatchesObjectCtx(g, spec, o, sc) {
				count++
			}
		}
	default:
		// A defined group this build cannot enumerate (Targeted,
		// TriggeredCardLKICopy): unresolved, the sub runs unconditionally.
		return false, false
	}
	return count == 0, true
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
