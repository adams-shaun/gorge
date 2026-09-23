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
//     draw a card", the corpus's dominant bare-Condition value at 54 SAs);
//     `Foretold` — the FlagForetold cast provenance (Poison the Cup,
//     Alrund's Epiphany); and `Revolt` — CR 702.38's "a permanent you
//     controlled left the battlefield this turn" (Decommission's DB$
//     GainLife), read through Host.RevoltHolds; and `Blessing` -- CR
//     702.131's city's-blessing latch (state.Player.Blessing, granted by
//     the Ascend machinery), read straight off the folded state; and
//     `Delirium` — four or more distinct core card types in the resolving
//     controller's graveyard (Descend upon the Sinful's DB$ Token), read
//     through Host.DeliriumHolds — the same census the "Delirium —"
//     activation/continuous/replacement gates already share. The other
//     bare-Condition values (OptionalCost, Bargain, Threshold, Metalcraft,
//     Hellbent, Surge — ~25 SAs) stay unresolved.
//  5. `ConditionDefined$ Imprinted` (34 corpus lines over 26 files) — the
//     source card's persistent imprint list (state.Object.Imprinted, the
//     events.Imprint associations: Chrome Mox's Imprint$ and now api:Play's
//     ImprintPlayed$ recording, Rashmi and Ragavan's played-exiled-card
//     marker). The group is exactly that list — never the remembered set —
//     and a source-less context (c.Source 0, a synthetic fixture) leaves
//     the gate unresolved, the fail-open run-anyway this file's convention.
//
// Deliberate scope (task fb-3f1cc033): the wider Condition vocabulary —
// Condition$ beyond Kicked, ConditionZone$ (57), ConditionManaSpent$ (34),
// the other ConditionDefined$ values (Targeted 161, ChosenCard 90, ... —
// Imprinted is IN since the ImprintPlayed task, shape 5 above), a bare
// ConditionCompare$ with no group, and ConditionNotPresent$ (8) — is NOT
// implemented. A sub carrying any of
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
//
// Two more keys landed with the Unbreakable Formation task
// (agent-20260918T200326Z-10b49320): `ConditionPlayerTurn$ True|False` —
// the resolving controller's turn vs. not — and `ConditionPhases$ <list>`
// (Main1,Main2 — the Addendum family), both read through the ONE shared
// phase-name parser state.ParsePhases and AND-ed with whatever group gate
// the SA also carries.
func conditionMet(h Host, c *Ctx, sa *cards.SA) (met bool, resolved bool) {
	defined := strings.TrimSpace(sa.Params["ConditionDefined"])
	present := strings.TrimSpace(sa.Params["ConditionPresent"])
	notPresent := strings.TrimSpace(sa.Params["ConditionNotPresent"])
	compare := strings.TrimSpace(sa.Params["ConditionCompare"])
	check := strings.TrimSpace(sa.Params["ConditionCheckSVar"])
	svarCmp := strings.TrimSpace(sa.Params["ConditionSVarCompare"])
	bare := strings.TrimSpace(sa.Params["Condition"])
	playerTurn := strings.TrimSpace(sa.Params["ConditionPlayerTurn"])
	phases := strings.TrimSpace(sa.Params["ConditionPhases"])
	firstCombat := strings.TrimSpace(sa.Params["ConditionFirstCombat"])
	if defined == "" && present == "" && notPresent == "" && compare == "" && check == "" && bare == "" &&
		playerTurn == "" && phases == "" && firstCombat == "" {
		return true, false // not gated (a lone ConditionSVarCompare$ compares nothing)
	}
	// Any other Condition* key (Zone, ManaSpent, ...) beside the supported
	// nine makes the shape unsupported. ConditionDescription$ is
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
			"ConditionCheckSVar", "ConditionSVarCompare", "Condition",
			"ConditionPlayerTurn", "ConditionPhases", "ConditionFirstCombat":
		default:
			return false, false
		}
	}
	// The player-turn / phase preconditions (ConditionPlayerTurn$ True|False,
	// ConditionPhases$ <phase-list>): the Unbreakable Formation Addendum
	// family and the conditional enters-tapped lands. ConditionPlayerTurn$
	// compares g.Active with the resolving controller, case-insensitively
	// True/False — Eddymurk Crab's `False` ("enters tapped if it's not your
	// turn") is a real shape, not a negation-by-absence. ConditionPhases$
	// parses through the ONE shared phase-name parser (state.ParsePhases —
	// the same parser Mode$ Phase triggers use) and requires the game's
	// current step to be in the named set; unknown names or an empty
	// resolved set are unsupported, fail-open per this file's convention.
	// Both are preconditions AND-ed with whatever group gate the SA also
	// carries (combine below); a value this gate cannot read leaves the
	// whole shape unsupported so the sub runs unconditionally, exactly as
	// before these keys existed.
	g := h.Game()
	extraMet := true
	if playerTurn != "" {
		switch strings.ToLower(playerTurn) {
		case "true":
			extraMet = g.Active == c.Controller
		case "false":
			extraMet = g.Active != c.Controller
		default:
			return false, false
		}
	}
	if phases != "" {
		set, unknown := state.ParsePhases(phases)
		if len(unknown) > 0 || set == 0 {
			return false, false
		}
		if !set.Has(g.Step) {
			extraMet = false
		}
	}
	// ConditionFirstCombat$ (the DB$ AddPhase gate: Raiyuu, Storm's Edge and
	// A-Raiyuu's "if it's the first combat phase of the turn" -- 3 corpus
	// lines, the gate that keeps an extra combat from granting another one):
	// met when the current combat is the turn's FIRST, read off the folded
	// per-turn combat count (state.Game.CombatsThisTurn, one increment per
	// BeginCombat entry). Only "True" is a corpus shape; anything else stays
	// unsupported (the fail-open run-anyway this file's convention).
	if firstCombat != "" {
		if !strings.EqualFold(firstCombat, "True") {
			return false, false
		}
		extraMet = extraMet && g.CombatsThisTurn == 1
	}
	// combine AND-s the group gate's answer with the player-turn/phase
	// preconditions above: a resolved group gate that says run still stays
	// skipped when a phase/turn precondition says no, and an unresolved
	// group gate keeps the whole shape fail-open.
	combine := func(met, resolved bool) (bool, bool) {
		if resolved && !extraMet {
			return false, true
		}
		return met, resolved
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
		if defined != "" || present != "" || notPresent != "" || compare != "" || bare != "" ||
			playerTurn != "" || phases != "" || firstCombat != "" {
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
	// A bare Condition$ is the cast-option family: Kicked and Foretold are
	// evaluated over the source's CastFlags (the bit the Kicker payment / the
	// Foretell action's CastInfo recorded -- the same provenance Count$
	// Foretold reads); Revolt is CR 702.38's leave-the-battlefield state
	// through Host.RevoltHolds; Blessing is CR 702.131's city's-blessing
	// latch (state.Player.Blessing); Delirium is the controller's graveyard
	// holding four or more distinct core card types, through
	// Host.DeliriumHolds (the same census the "Delirium —" activation,
	// continuous and replacement gates read, so the spellings cannot drift);
	// OptionalCost/Bargain/Threshold/Metalcraft/Hellbent/Surge stay
	// unresolved and run unconditionally. A bare Condition beside a group key or beside
	// ConditionSVarCompare$ is a mixed shape no single evaluator covers (~11
	// corpus SAs).
	if bare != "" {
		if defined != "" || present != "" || notPresent != "" || compare != "" || svarCmp != "" ||
			playerTurn != "" || phases != "" || firstCombat != "" {
			return false, false
		}
		switch {
		case strings.EqualFold(bare, "Kicked"):
			o := h.Game().Obj(c.Source)
			return o != nil && o.CastFlags&state.FlagKicked != 0, true
		case strings.EqualFold(bare, "Foretold"):
			// CR 702.126: the "if this spell was foretold" gate (Poison the
			// Cup's conditional scry, Alrund's Epiphany's conditional tokens) --
			// the same FlagForetold provenance Count$Foretold reads, the two
			// bare-Condition corpus carriers are exactly this shape.
			o := h.Game().Obj(c.Source)
			return o != nil && o.CastFlags&state.FlagForetold != 0, true
		case strings.EqualFold(bare, "Revolt"):
			// CR 702.38's ability-word gate (Decommission's DB$ GainLife |
			// Condition$ Revolt -- the corpus's only bare-Condition$ Revolt
			// line): a permanent the resolving controller controlled left
			// the battlefield this turn, through the Host predicate the
			// rules-side Revolt$ clauses and the Count$Revolt branch head
			// share, so the spellings cannot drift apart.
			return h.RevoltHolds(c.Controller), true
		case strings.EqualFold(bare, "Delirium"):
			// The Delirium ability word: four or more distinct core card types
			// among cards in the resolving controller's graveyard
			// (Descend upon the Sinful's DB$ Token is the deck card that
			// needed it; drag_to_the_roots-style Continuous statics and the
			// activation gates read the same census rules-side). An
			// out-of-range controller denies -- a graveyard this build cannot
			// name cannot hold four types.
			return h.DeliriumHolds(c.Controller), true
		case strings.EqualFold(bare, "Blessing"):
			// CR 702.131: the city's blessing (Ascend), read off the one-way
			// latch state.Player.Blessing that events.Apply's BlessingChange
			// fold writes (rules/ascend.go grants it). This is what makes
			// ocelot_pride's DB$ CopyPermanent and the_golden_city_of_orazca's
			// DB$ Draw condition-gated instead of run-anyway. An out-of-range
			// controller denies -- the fail-closed direction a blessing gate
			// that cannot name its seat must take.
			if int(c.Controller) >= len(g.Players) {
				return false, true
			}
			return g.Players[c.Controller].Blessing, true
		}
		return false, false
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
		return combine(conditionNotPresentMet(h, c, defined, notPresent))
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
			if compare == "" {
				// No group key named anything and the player-turn/phase gates
				// above resolved: met is exactly their conjunction (the
				// Eddymurk Crab shape — a lone ConditionPlayerTurn$ gate).
				return extraMet, true
			}
			return false, false
		}
		return combine(conditionMetBattlefield(h, c, present, compare))
	}
	if defined != "Remembered" && defined != "Self" && defined != "TriggeredCard" &&
		defined != "Imprinted" && defined != "Discarded" && defined != "Targeted" {
		// Only the Remembered, Self, TriggeredCard, Imprinted and Targeted
		// families are in scope among DEFINED groups: the objects a walk
		// carries in Ctx.Remembered, the resolving source object alone (the
		// Addendum shape: ConditionDefined$ Self | ConditionPresent$
		// Card.wasCast holds only when the sub is reached through a cast of
		// the source, which effects.Resolve's walk evaluates while the spell
		// is still on the stack), the card the triggering event moved (task
		// castprov2, Amped Raptor's `ConditionDefined$ TriggeredCard |
		// ConditionPresent$ Card.wasCastFromYourHandByYou`: the exile-until
		// runs only when the entering permanent was cast from its
		// controller's hand), the source card's persistent imprint list
		// (shape 5 above — Rashmi and Ragavan's `ConditionDefined$ Imprinted
		// | ConditionPresent$ Card | ConditionCompare$ EQ0`: the MayPlay
		// static registers only when the Play did NOT cast the card, the "if
		// you don't cast it this way" branch), and the resolving ability's
		// own chosen targets (Stalking Leonin's `ConditionDefined$ Targeted |
		// ConditionPresent$ Card.ChosenCtrl`: the exile runs only when the
		// targeted attacker is controlled by the secretly chosen player).
		// ChosenCard, the LKI-copy variants and the rest need Ctx state this
		// gate does not model (and whose fail-closed skip would change
		// unrelated cards).
		return false, false
	}
	sc := c.SpecContext(c.Controller)
	count := 0
	group := rememberedWithSource(h, c)
	if defined == "Targeted" {
		// ConditionDefined$ Targeted is the resolving ability's OWN answered
		// targets: Forge's `Targeted` defined group. It reads the same two
		// channels Defined$ Targeted does (effects/context.go — the generic
		// pre-ask's Ctx.PickedTargets while a pre-asked body dispatches, else
		// the resolution-level Ctx.Targets), so the gate and the effects' own
		// Defined$ Targeted can never disagree about which targets the
		// ability chose. Both empty is a resolved zero (the ability really
		// chose nothing), not the fail-open an absent binding gets elsewhere:
		// a Target-bearing ability always has a definite target list, and a
		// gate over an empty list genuinely denies.
		group = targetedGroup(c)
	}
	if defined == "Discarded" {
		// ConditionDefined$ Discarded (task mordorparams1: Moria Scavenger's
		// "If the discarded card was a creature card, amass Orcs 1",
		// Argentum Masticore's "When you discard a card this way, destroy
		// ..."): Forge's group is the cards the resolving chain discarded.
		// Two provenance channels enumerate it: the unless-payment's settled
		// Discard<...> component (Ctx.UnlessDiscarded, set by rules'
		// unless_pay resume arm — the mid-resolution channel), and the
		// resolving object's own activation cost discards read off the log
		// (Host.DiscardedInWindow — Moria's channel; the cost discard is
		// emitted at activation, the sub runs at resolution). Both channels
		// empty leaves the gate UNRESOLVED — the sub runs unconditionally,
		// this file's documented fail-open — never a resolved-false that
		// would silently stop subs that ran before this read existed.
		var grpOK bool
		group, grpOK = discardedGroup(h, c)
		if !grpOK {
			return false, false
		}
	}
	if defined == "Self" {
		// Self is the source object ALONE — not rememberedWithSource's
		// Source-union with the walk's remembered set.
		group = []state.Target{{Obj: c.Source}}
	}
	if defined == "Imprinted" {
		// The source card's persistent imprint list (state.Object.Imprinted,
		// the events.Imprint associations): the ONLY group for this family —
		// never the remembered set. A source-less context is the one
		// unresolved shape; a real source with an empty list is a resolved
		// zero (Rashmi's EQ0 arm), never fail-open.
		if c.Source == 0 {
			return false, false
		}
		group = nil
		if o := g.Obj(c.Source); o != nil {
			for _, id := range o.Imprinted {
				group = append(group, state.Target{Obj: id})
			}
		}
	}
	if defined == "TriggeredCard" {
		// The card the triggering event moved — the TriggerContext.TriggerCard
		// role rules' triggerReferents captures for every mode that names one
		// (ChangesZone, SpellCast, Drawn, ...). An ABSENT binding (a synthetic
		// fixture, a hand-built context, a mode with no card role) leaves the
		// gate UNSUPPORTED — the sub runs unconditionally, this file's
		// documented convention — never a resolved-false, which would silently
		// stop subs that ran before the group was enumerable. Measured corpus
		// population of `ConditionDefined$ TriggeredCard` gates: 33 raw lines
		// over 36 files, every one of which ran its sub unconditionally before.
		if c.TriggerCard == 0 {
			return false, false
		}
		group = []state.Target{{Obj: c.TriggerCard}}
	}
	// The wasCastFromYourHandByYou / !wasCastFromYourHandByYou qualifier
	// (task castprov2, Amped Raptor's gate) and its bare wasCastFromYourHand
	// sibling (task castprov3, Otterball Antics' `Card.wasCast+!
	// wasCastFromYourHand`) are not filter predicates: they are evaluated
	// per member against the Host's log reads (castFromHandAdmitsFilter /
	// castFromHandAnyAdmitsFilter), the same split rules' castProvenanceAdmits
	// applies at the rules-side match sites. The UnknownPredicates guard
	// below reads the token-STRIPPED spec — the tokens themselves are unknown
	// to the filter (that is the whole reason for the split), and an
	// unreadable remainder must still be unresolved.
	hasHandToken := strings.Contains(present, "wasCastFromYourHandByYou")
	// The bare spelling is a SUBSTRING of the ByYou token, so a ByYou spec
	// must not route to the bare helper — the ByYou branch owns it.
	hasBareHand := !hasHandToken && strings.Contains(present, "wasCastFromYourHand")
	// The card-level CastSa flag tokens (task mayhem: Sandman's
	// Quicksand's "if this spell's mayhem cost was paid" split; task
	// mayplay-warp: Full Bore's Card.CastSa Spell.Warp) are evaluated per
	// member off the cast's CastFlags provenance, the same split the hand
	// families take. The spend spellings of the CastSa family stay
	// fail-closed here (no strip; wordMatches never matches them) — the
	// documented convention above.
	hasCastSaFlagTok := hasCastSaFlag(present)
	if present != "" {
		check := present
		if hasHandToken || hasBareHand {
			check = stripWasCastFromHandToken(present)
		}
		if len(UnknownPredicates(check)) > 0 {
			return false, false
		}
	}
	for _, t := range group {
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
		memberSpec := present
		if hasHandToken {
			s, ok := castFromHandAdmitsFilter(h, present, t.Obj, c.Controller)
			if !ok {
				// This member fails its own provenance requirement.
				continue
			}
			memberSpec = s
		} else if hasBareHand {
			s, ok := castFromHandAnyAdmitsFilter(h, present, t.Obj)
			if !ok {
				continue
			}
			memberSpec = s
		}
		if hasCastSaFlagTok {
			s, ok := castSaAdmitsFilter(h, memberSpec, t.Obj)
			if !ok {
				continue
			}
			memberSpec = s
		}
		if MatchesObjectCtx(g, memberSpec, o, sc) {
			count++
		}
	}
	return combine(evalConditionCount(count, compare))
}

// discardedGroup enumerates the ConditionDefined$ Discarded group: the
// cards the resolving chain discarded, over the two provenance channels
// conditionMet's Discarded branch documents. The log-scan channel is scoped
// to c.ResolvingObj — the wrapper whose resolution is walking — so another
// activation's cost discard cannot bleed in; a wrapper-less context (a
// hand-built one) has no window to scan. Both channels empty is UNRESOLVED
// (false), never a resolved-empty: the fail-open convention must not turn
// into a resolved gate just because no channel carried evidence.
func discardedGroup(h Host, c *Ctx) ([]state.Target, bool) {
	var out []state.Target
	seen := make(map[state.ObjID]bool, len(c.UnlessDiscarded)+4)
	add := func(id state.ObjID) {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, state.Target{Obj: id})
		}
	}
	for _, t := range c.UnlessDiscarded {
		add(t.Obj)
	}
	if c.ResolvingObj != 0 {
		for _, id := range h.DiscardedInWindow(c.ResolvingObj) {
			add(id)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
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
	case "Targeted":
		// The resolving ability's own answered targets — the same group
		// conditionMet's Targeted branch enumerates.
		for _, t := range targetedGroup(c) {
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
		// A defined group this build cannot enumerate (TriggeredCardLKICopy):
		// unresolved, the sub runs unconditionally.
		return false, false
	}
	return count == 0, true
}

// targetedGroup is the ConditionDefined$ Targeted group: the resolving
// ability's own answered targets, over the same two channels Defined$
// Targeted reads (effects/context.go) — Ctx.PickedTargets while a pre-asked
// body dispatches, else the resolution-level Ctx.Targets.
func targetedGroup(c *Ctx) []state.Target {
	if c.PickedTargets != nil {
		return c.PickedTargets
	}
	return c.Targets
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

// admitProvenanceAlternativesFilter is the effects-side rejoin loop shared
// by the two cast-provenance filter helpers (rules' twin,
// admitProvenanceAlternatives, lives in rules/cast_provenance.go): the spec
// is split into its comma alternatives, every alternative CARRYING the
// qualifier but failing the provenance test is dropped, and the surviving
// alternatives are rejoined for the ordinary filter. ok is false when no
// alternative survives: the spec matches nothing. A spec without the token
// is returned unchanged, so every unrelated gate is byte-identical.
func admitProvenanceAlternativesFilter(spec, pred string, holds bool) (string, bool) {
	if !strings.Contains(spec, pred) {
		return spec, true
	}
	var b strings.Builder
	first, alive := true, false
	for alt := range FilterAlternatives(spec) {
		s1, hadPos := StripPredicateToken(alt, pred)
		s2, hadNeg := StripPredicateToken(s1, "!"+pred)
		// The positive spelling requires the provenance to HOLD; the negated
		// spelling requires it to FAIL.
		if (hadPos && !holds) || (hadNeg && holds) {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		b.WriteString(s2)
		first = false
		alive = true
	}
	if !alive {
		return "", false
	}
	return b.String(), true
}

// castFromHandAdmitsFilter evaluates the bare wasCastFromYourHandByYou /
// !wasCastFromYourHandByYou qualifier of a Forge filter spec against ONE
// object through the Host's log read (task castprov2, Amped Raptor's
// `ConditionPresent$ Card.wasCastFromYourHandByYou` gate — the effects-side
// twin of rules' castFromHandAdmits, which runs the same split at the
// rules-side match sites where the Engine and its log are in scope). The
// object was NOT cast from you's hand by you, or the object is a copy (never
// cast, the same IsCopy guard the Count$wasCastFromYourHandByYou head
// takes) — every alternative carrying the qualifier but failing the
// provenance test is dropped. ok is false when no alternative survives: the
// spec matches nothing (this member fails its own provenance requirement).
func castFromHandAdmitsFilter(h Host, spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	if !strings.Contains(spec, "wasCastFromYourHandByYou") {
		return spec, true
	}
	holds := false
	if o := h.Game().Obj(objID); o != nil && !o.IsCopy {
		holds = h.WasCastFromHandByYou(objID, you)
	}
	return admitProvenanceAlternativesFilter(spec, "wasCastFromYourHandByYou", holds)
}

// castFromHandAnyAdmitsFilter evaluates the BARE wasCastFromYourHand /
// !wasCastFromYourHand qualifier (task castprov3 — the player-less hand
// provenance: the "from anywhere other than your hand" carriers whose
// scripts omit the ByYou suffix, Otterball Antics' `ConditionPresent$
// Card.wasCast+!wasCastFromYourHand`) against ONE object through the Host's
// log read — the effects-side twin of rules' castFromHandAnyAdmits. The
// provenance is any caster's hand: every carrier that needs player scoping
// supplies it elsewhere in the spec. Copies were never cast; a card never
// put on the stack reads false. A spec carrying the ByYou spelling is NOT
// this helper's family (ByYou is a SUPERSTRING of the bare token; its own
// helper runs first wherever both could appear) and is returned unchanged.
func castFromHandAnyAdmitsFilter(h Host, spec string, objID state.ObjID) (string, bool) {
	if strings.Contains(spec, "wasCastFromYourHandByYou") || !strings.Contains(spec, "wasCastFromYourHand") {
		return spec, true
	}
	holds := false
	if o := h.Game().Obj(objID); o != nil && !o.IsCopy {
		holds = h.WasCastFromHand(objID)
	}
	return admitProvenanceAlternativesFilter(spec, "wasCastFromYourHand", holds)
}

// castSaFlagTokens is the effects-side list of the CastSa flag spellings
// whose truth rides the cast's pay-time CastInfo CastFlags (state.FlagMayhem,
// state.FlagWarped). It mirrors the flag entries of rules' castSaTokens
// (effects cannot import rules); a new flag spelling must be added to BOTH
// tables, and the census recognition in filter.go's wordKind switch is the
// third site. Spell.MayPlaySource is deliberately NOT here: it is the
// sibling task mayplay-src's predicate, recorded and read rules-side only.
// The loop in castSaAdmitsFilter and the hasCastSaFlag gate in the
// ConditionPresent walk both read this one table, so the next flag spelling
// cannot be handled by one read and missed by the other.
var castSaFlagTokens = []struct {
	token string
	flag  uint64
}{
	{token: "CastSa Spell.Mayhem", flag: state.FlagMayhem},
	{token: "CastSa Spell.Warp", flag: state.FlagWarped},
}

// hasCastSaFlag reports whether spec carries ANY flag spelling, the cheap
// gate the ConditionPresent walk uses before calling castSaAdmitsFilter.
func hasCastSaFlag(spec string) bool {
	for _, t := range castSaFlagTokens {
		if strings.Contains(spec, t.token) {
			return true
		}
	}
	return false
}

// castSaAdmitsFilter evaluates the card-level CastSa flag tokens (task
// mayhem: Sandman's Quicksand's `ConditionPresent$ Card.!
// CastSa Spell.Mayhem` / `Card.CastSa Spell.Mayhem` split; task
// mayplay-warp: Full Bore's `ConditionPresent$ Card.CastSa Spell.Warp`)
// against ONE object through the Host's game read — the effects-side twin
// of rules' castSaAdmits (which runs the same strip at the rules-side match
// sites where the Engine is in scope). holds reads the object's LATEST
// cast's CastFlags: the pay-time CastInfo REPLACES the set (events.Apply's
// CastInfo case), so a re-cast object cannot inherit an older cast's flags.
// A copy was never cast (CR 707.10) and carries no provenance bit for the
// flags state.CastProvenanceFlags strips at the copy; FlagWarped is
// deliberately NOT in that set (a warp cast's alternative cost is a choice
// the copy rules carry), so a warp copy reads true — the same ruling the
// FlagWarped entry hook takes. Both spellings of each token (positive and
// !-negated) are handled; the spend spellings of the CastSa family and the
// sibling task mayplay-src's Spell.MayPlaySource stay fail-closed here, the
// documented convention.
func castSaAdmitsFilter(h Host, spec string, objID state.ObjID) (string, bool) {
	var flags uint64
	flagsRead := false
	for _, t := range castSaFlagTokens {
		if !strings.Contains(spec, t.token) {
			continue
		}
		if !flagsRead {
			if o := h.Game().Obj(objID); o != nil && !o.IsCopy {
				flags = o.CastFlags
			}
			flagsRead = true
		}
		var ok bool
		if spec, ok = admitProvenanceAlternativesFilter(spec, t.token, flags&t.flag != 0); !ok {
			return "", false
		}
	}
	return spec, true
}

// stripWasCastFromHandToken removes both spellings of the
// wasCastFromYourHandByYou qualifier AND the bare wasCastFromYourHand
// spelling (task castprov3) from every comma alternative of a spec,
// text-only and polarity-agnostic: the UnknownPredicates guard must read the
// token-STRIPPED spec (the tokens themselves are unknown to the filter —
// that is the whole reason the split exists), while the per-member polarity
// lives in castFromHandAdmitsFilter / castFromHandAnyAdmitsFilter. A spec
// without either token is returned unchanged.
func stripWasCastFromHandToken(spec string) string {
	if !strings.Contains(spec, "wasCastFromYourHand") {
		return spec
	}
	var b strings.Builder
	first := true
	for alt := range FilterAlternatives(spec) {
		s1, _ := StripPredicateToken(alt, "wasCastFromYourHandByYou")
		s2, _ := StripPredicateToken(s1, "!wasCastFromYourHandByYou")
		s3, _ := StripPredicateToken(s2, "wasCastFromYourHand")
		s4, _ := StripPredicateToken(s3, "!wasCastFromYourHand")
		if !first {
			b.WriteByte(',')
		}
		b.WriteString(s4)
		first = false
	}
	return b.String()
}
