package effects

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Mana", effMana)
	Register("ReplaceMana", effReplaceMana)
	Register("Effect", effEffect)
	Register("Cleanup", effCleanup)
	Register("SetState", effSetState)
	Register("Counter", effCounter)
	Register("DelayedTrigger", effDelayedTrigger)
	Register("Repeat", effRepeat)
	Register("Charm", effCharm)
	Register("Vote", effVote)
	Register("BecomeMonarch", effBecomeMonarch)
	Register("RestartGame", effRestartGame)
	Register("Goad", effGoad)
	Register("Ward", effWard)
}

// effGoad records each independently-lived goad relationship. Duration and
// source are event payload so replay can expire conditional goads identically.
func effGoad(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			if strings.EqualFold(sa.Params["NoLonger"], "True") {
				h.Emit(events.Event{Kind: events.Goad, Obj: o.ID, Amount: -1})
				continue
			}
			duration := sa.Params["Duration"]
			if duration == "" {
				duration = "UntilYourNextTurn"
			}
			h.Emit(events.Event{Kind: events.Goad, Obj: o.ID, Player: c.Controller,
				Text: duration, IDs: []state.ObjID{c.Source}, Amount: int32(o.Controller) + 1})
		}
	}
}

// effWard is the resolution half of the Ward keyword trigger. The triggering
// spell/ability is held in TriggerSource; after a declined payment it is
// countered and an ability is parked in exile (CR 608.2m).
func effWard(h Host, c *Ctx, sa *cards.SA) {
	cause := c.TriggerStack
	o := h.Game().Obj(cause)
	if o == nil || o.Zone != state.ZStack {
		return
	}
	if c.UnlessPay == "" {
		cost := sa.Params["UnlessCost"]
		d := &decision.Decision{Player: o.Controller, Kind: decision.KModes, Min: 1, Max: 1,
			Prompt: "Pay " + cost + " for ward?", ResumeKind: "unless_pay", ResumeSA: sa,
			Options: []decision.Option{{Index: 0, Kind: "mode", Label: "Pay " + cost, Player: o.Controller}, {Index: 1, Kind: "mode", Label: "Don't pay", Player: o.Controller}}}
		h.Ask(d)
		return
	}
	paid := c.UnlessPay == "pay"
	c.UnlessPay = ""
	if paid {
		return
	}
	to := state.ZGraveyard
	if o.Face() == nil {
		to = state.ZExile
	}
	h.Emit(events.Event{Kind: events.MoveZone, Obj: cause, From: state.ZStack, To: to, Text: "countered by ward"})
}

// CopySpellAbility is NOT registered. It needs to create a brand new game
// object mid-match (a copy of a spell or ability already on the stack), and
// every state mutation in this engine goes through events.Apply -- there is
// no Apply case yet that mints an ID and decides how a copy's Card/FaceIdx/
// Ability/Targets carry over. Token had the identical shape (see the Task
// 18 report for the original analysis of both) and closed it in Task 13 via
// events.TokenCreate, whose Apply case mints the new object from
// Game.Tokens; see token.go. CopySpellAbility's own object comes from
// wherever it is on the stack already, not a registry, so it needs its own
// event kind (StackCopy exists as of Task 12) wired up rather than reusing
// TokenCreate's shape verbatim. Registering it as a Note-only stub would
// make Supported()/Coverage claim a card is playable when it cannot
// actually do what its text says. Left unregistered, Resolve's existing
// "unimplemented API" Note fallback applies, and Coverage correctly
// excludes any card that needs it.

// effEffect creates a lasting effect object holding StaticAbilities$ for
// Duration$. This is the part of Task ce1 that turns the M1 Note into a real
// registration: a StaticAbilities$ entry naming a RESTRICTION static
// (CantTarget for Vines of Vastwood, CantRegenerate for Incinerate) is
// registered into the engine's continuous-effect registry (rules' layer
// system, reached through Host.AddContinuous) so the rule it modifies is
// actually consulted rather than left as a silent Note.
//
// Registration is deliberately scoped: only the CantTarget and CantRegenerate
// modes become real effects this round. Every other StaticAbilities$ mode ---
// and every Triggers$ entry (Palace Jailer's "exile until an opponent becomes
// the monarch" is a command-zone trigger this build does not model) --- is
// still recorded as a Note, so nothing silently no-ops into looking supported
// when it is not. The registry entry the engine (rules/layers.go active())
// expires is the same until-end-of-turn / source-leaves discipline every other
// continuous effect uses: an Effect from an instant or sorcery (a one-shot
// spell) or carrying an explicit this-turn Duration$ is UntilEOT, dropped at
// end-of-turn cleanup; anything else persists while its source stays on the
// battlefield.
func effEffect(h Host, c *Ctx, sa *cards.SA) {
	dur := sa.Params["Duration"]
	if dur == "" {
		dur = "Permanent"
	}
	what := strings.TrimSpace(sa.Params["StaticAbilities"] + " " + sa.Params["Triggers"])
	remembered := effectRemembered(h, c, sa)
	registered := false
	for _, name := range strings.Fields(sa.Params["StaticAbilities"]) {
		mode, params := parseStaticLine(c.SVars, name)
		switch mode {
		case "Continuous":
			// A may-play-from-zone grant delivered by an Effect SA (Atsushi's
			// "you may play those cards" STPlay static): registered like the
			// S: static shape, with the Effect's Remembered set seeding the
			// grant so the Affected$ Card.IsRemembered spec matches the cards
			// the resolution exiled/remembered (rules' grant walk matches
			// through a SpecContext that carries this list). The shared
			// MayPlayStaticParams whitelist keeps both registration paths
			// honest: a rider this build does not read fails closed here too.
			if grant, ok := mayPlayGrantFromLine(params); ok {
				grant.Source = c.Source
				grant.Controller = c.Controller
				grant.UntilEOT = effectUntilEOT(h, c.Source, dur)
				grant.Remembered = remembered
				grant.Duration = dur
				h.AddContinuous(grant)
				registered = true
			} else if len(params) > 0 {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "continuous effect " + mode + " unimplemented (" + what + ")"})
				registered = true
			}
		case "CantTarget", "CantRegenerate":
			// A compound IsRemembered spec (Card.IsRemembered+Creature) cannot
			// be resolved by the remembered-set match alone -- the extra
			// predicate would be silently dropped, over-applying the
			// restriction. No corpus restriction static carries one (see the
			// report / AGENTS.md), so treat it as unsupported here and keep
			// the Note instead of registering something that over-applies.
			if compoundRememberedSpec(params) {
				if mode != "" {
					h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
						Text: "continuous effect " + mode + " unimplemented (" + what + ")"})
				}
				registered = true
				continue
			}
			ce := state.ContinuousEffect{
				Source:         c.Source,
				Controller:     c.Controller,
				UntilEOT:       effectUntilEOT(h, c.Source, dur),
				Restriction:    mode,
				RestrictParams: params,
				Remembered:     remembered,
				Duration:       dur,
			}
			h.AddContinuous(ce)
			registered = true
		default:
			// A resolvable but unsupported mode is reported honestly; an
			// unresolvable name (mode "") falls through to the generic Note
			// below rather than emitting an empty-mode message.
			if mode != "" {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "continuous effect " + mode + " unimplemented (" + what + ")"})
			}
		}
	}
	// Nothing registered (an unsupported StaticAbilities$ mode, or a
	// Triggers$-only effect such as Palace Jailer's command-zone trigger):
	// keep the original Note wording so a card whose effect this build still
	// does not make real does not move the chain for a purely cosmetic
	// reason. The registry is the feature; a Note that names what was asked
	// for is the honest stand-in until the mode is implemented.
	if !registered {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "registers a continuous effect (" + what + ") for " + dur})
	}
}

// mayPlayGrantFromLine builds the may-play ContinuousEffect from one parsed
// static line (an SVar static body effEffect registers, or the S: line rules
// passes through MayPlayStaticParams). ok=false is the fail-closed grant:
// nothing is registered rather than a half-read grant going live.
func mayPlayGrantFromLine(params map[string]string) (state.ContinuousEffect, bool) {
	ignoreColor, limit, playerTurn, ok := MayPlayStaticParams(params)
	if !ok {
		return state.ContinuousEffect{}, false
	}
	return state.ContinuousEffect{
		Affects:            params["Affected"],
		AffectedZone:       strings.TrimSpace(params["AffectedZone"]),
		MayPlay:            true,
		MayPlayIgnoreColor: ignoreColor,
		MayPlayLimit:       limit,
		MayPlayPlayerTurn:  playerTurn,
	}, true
}

// MayPlayStaticParams reports whether a Mode$ Continuous static body (an S:
// line or an SVar static an Effect SA registers) carries the may-play grant
// this build implements, and resolves its two readable riders. The
// implemented shape is MayPlay$ True plus an Affected$/AffectedZone$ pair and
// only display/placement metadata; MayPlayIgnoreColor$ (mana as any colour),
// MayPlayLimit$ (an integer once-per-turn cap) and Condition$ PlayerTurn
// ("during each of your turns", the Kess/Karador family) are read. Anything
// else -- MayPlayIgnoreType$/MayPlayWithoutManaCost$/MayPlayText$ (they change
// what the cast IS, not just where it may come from), a Condition$ whose value
// is not PlayerTurn, a ValidAfterStack$/Secondary$ qualifier (it changes when
// the grant lives), or a MayPlayLimit$ value that is not a non-negative
// integer -- fails closed:
func MayPlayStaticParams(params map[string]string) (ignoreColor bool, limit int32, playerTurn bool, ok bool) {
	v, okv := params["MayPlay"]
	if !okv || !strings.EqualFold(strings.TrimSpace(v), "True") {
		return false, 0, false, false
	}
	for key := range params {
		switch key {
		case "Mode", "MayPlay", "MayPlayIgnoreColor", "MayPlayLimit", "Condition",
			"Affected", "AffectedZone", "Description", "EffectZone":
			// The keys the implemented grant (and only it) carries.
		default:
			return false, 0, false, false
		}
	}
	limit = 0
	if raw, okv := params["MayPlayLimit"]; okv {
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
		if err != nil || n < 0 {
			// A MayPlayLimit$ value this build cannot enforce must not
			// silently become "unlimited".
			return false, 0, false, false
		}
		limit = int32(n)
	}
	playerTurn = strings.EqualFold(strings.TrimSpace(params["Condition"]), "PlayerTurn")
	if cond, okv := params["Condition"]; okv && !playerTurn {
		// A Condition$ other than PlayerTurn changes when the grant lives;
		// never register it half-read.
		_, _ = cond, okv
		return false, 0, false, false
	}
	ignoreColor = strings.EqualFold(strings.TrimSpace(params["MayPlayIgnoreColor"]), "True")
	return ignoreColor, limit, playerTurn, true
}

// parseStaticLine parses an S: static body an SVar holds ("Mode$ CantTarget |
// ValidTarget$ Card.IsRemembered | ...") into its mode and parameter map. The
// body has no SP$/AB$/DB$ head, so cards' parseSA is the wrong shape; this is
// the S: line's own grammar (cards/parse.go's "S" case). An empty or
// malformed body degrades to "" mode and a nil map, which the switch in
// effEffect treats as unimplemented rather than as a registration.
func parseStaticLine(svars map[string]string, name string) (string, map[string]string) {
	body := strings.TrimSpace(svars[name])
	if body == "" {
		return "", nil
	}
	params := make(map[string]string)
	mode := ""
	for _, seg := range strings.Split(body, "|") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		key, val, ok := strings.Cut(seg, "$")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		params[key] = val
		if key == "Mode" {
			mode = val
		}
	}
	return mode, params
}

// effectRemembered resolves RememberObjects$ into the concrete object ids the
// Effect captured. "Targeted"/"ParentTarget" remember the chosen targets;
// "Remembered" (and creature-flavoured spellings) remember the objects the
// resolution already had; "You & Targeted" and the default degrade to the
// source plus the chosen targets. Objects only: a player-only remember yields
// an empty slice, which a restriction whose ValidCard$ is Card.IsRemembered
// then applies to nothing.
func effectRemembered(h Host, c *Ctx, sa *cards.SA) []state.ObjID {
	ro := sa.Params["RememberObjects"]
	if ro == "" {
		ro = "Targeted"
	}
	var out []state.ObjID
	for _, part := range strings.FieldsFunc(ro, func(r rune) bool {
		return r == '&' || r == ',' || r == ' '
	}) {
		part = strings.TrimSpace(part)
		switch part {
		case "You", "Self", "Source":
			out = append(out, c.Source)
		case "Targeted", "ParentTarget":
			for _, t := range c.Targets {
				if !t.IsPlayer && h.Game().Obj(t.Obj) != nil {
					out = append(out, t.Obj)
				}
			}
		case "Remembered", "Remembered.Creature", "Remembered.Permanent", "RememberedCard":
			for _, t := range c.Remembered {
				if !t.IsPlayer && h.Game().Obj(t.Obj) != nil {
					out = append(out, t.Obj)
				}
			}
		case "ChosenCard":
			// Dauthi Voidwalker and the wider ChooseCard -> Effect family do
			// not set RememberChosen$: the chosen card lives in Ctx.Chosen, or
			// on the event-backed source when a later ability reads it.
			chosen := c.Chosen
			if len(chosen) == 0 {
				if o := h.Game().Obj(c.Source); o != nil {
					chosen = o.Chosen
				}
			}
			for _, t := range chosen {
				if !t.IsPlayer && h.Game().Obj(t.Obj) != nil {
					out = append(out, t.Obj)
				}
			}
		}
	}
	return out
}

// IsNextTurnDuration reports whether a Duration$ value names the
// controller's NEXT-turn lifetime (UntilYourNextTurn, UntilTheEndOfYourNextTurn)
// -- the two largest non-Permanent durations the wave survey measured. Such
// an effect is neither UntilEOT (which would expire it a full turn early, at
// the end of the current turn) nor source-leaves (which never expires it);
// it gets a real turn-boundary lifetime via ContinuousEffect.UntilTurn,
// computed in rules.Engine.AddContinuous. Exported so rules/layers.go can
// recognise the same spelling effEffect saw; the two largest values by far
// (105 + 70 raw lines), so this closes most of the turn-spanning gap.
func IsNextTurnDuration(dur string) bool {
	switch strings.ToLower(strings.TrimSpace(dur)) {
	case "untilyournextturn", "untiltheendofyournextturn":
		return true
	}
	return false
}

// compoundRememberedSpec reports whether a restriction static's valid-spec is
// a COMPOUND expression containing IsRemembered (a + AND or a , OR list) --
// a shape the remembered-set match cannot resolve faithfully. The corpus's
// CantTarget/CantRegenerate statics all use a bare Card.IsRemembered, so this
// is a defensive guard against silently over-applying a restriction whose
// extra predicate would be dropped (see rules/layers.go restrictionApplies).
func compoundRememberedSpec(params map[string]string) bool {
	spec := params["ValidCard"]
	if spec == "" {
		spec = params["ValidTarget"]
	}
	return strings.Contains(spec, "IsRemembered") && strings.ContainsAny(spec, "+,")
}

// effectUntilEOT decides expiry for an Effect registration: a one-shot spell
// (instant/sorcery) source, or an explicit this-turn Duration$, is UntilEOT
// and is dropped at end-of-turn cleanup (rules' EndOfTurnCleanup); anything
// else -- Duration$ Permanent on a permanent, an until-untap form, ... ---
// persists while its source stays on the battlefield, the same rule the
// layer effects use. A Duration$ that spans the controller's NEXT turn is
// NOT UntilEOT (it would expire a turn early); it is instead given a real
// turn-boundary lifetime (state.ContinuousEffect.UntilTurn) computed in
// rules.Engine.AddContinuous, so effectUntilEOT returns false for it.
func effectUntilEOT(h Host, source state.ObjID, dur string) bool {
	if IsNextTurnDuration(dur) {
		return false
	}
	if o := h.Game().Obj(source); o != nil {
		if f := o.Face(); f != nil && (f.IsInstant() || f.IsSorcery()) {
			return true
		}
	}
	switch strings.ToLower(strings.TrimSpace(dur)) {
	case "eot", "endofturn", "untilendofturn", "untilyournextendstep",
		"untilhostleavesplayoreot", "untilendofcombat", "end of turn",
		"this turn", "thisturnandnextturn":
		return true
	}
	return false
}

// effCleanup is "DB$ Cleanup | ClearRemembered$ True": nothing in this build
// persists a Remembered/Imprinted list on an object yet (Ctx.Remembered is a
// per-resolution parameter, not stored state), so there is nothing to
// actually clear. The Note records that the step ran.
func effCleanup(h Host, c *Ctx, sa *cards.SA) {
	// ClearRemembered$ True empties the resolution's remembered list. Forge's
	// Cleanup clears the (persisted) source remembered set after an earlier
	// sub has handed the list off; here the list is Ctx-local, so the clear
	// is invisible to the log but stops a later read in the same walk from
	// re-counting a list the script already handed off -- Valakut's DBCleanup
	// runs after DBEffect captured the dig's RememberChanged list into the
	// registered Effect, so the end-step trigger's own X=Remembered$Amount
	// must count only what IT moved. The Note stays exactly as it was (it is
	// a logged event whose text existing games pin).
	if strings.EqualFold(strings.TrimSpace(sa.Params["ClearRemembered"]), "True") {
		c.Remembered = nil
	}
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "clears remembered/imprinted objects"})
}

// effSetState flips a double-faced target to its other face. M1 does not
// model Mode$'s vocabulary (Transform/Flip/Meld all behave the same here):
// it just advances to the next face, wrapping to 0, which is correct for the
// overwhelmingly common two-face case and a no-op for anything with fewer
// than two faces (a token, or a single-faced card).
func effSetState(h Host, c *Ctx, sa *cards.SA) {
	mode := sa.Params["Mode"]
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Card == nil || len(o.Card.Faces) < 2 {
			continue
		}
		next := (int(o.FaceIdx) + 1) % len(o.Card.Faces)
		h.Emit(events.Event{Kind: events.Note, Obj: o.ID,
			Text: "flips to face " + strconv.Itoa(next) + " (" + mode + ")"})
		h.Emit(events.Event{Kind: events.FlipFace, Obj: o.ID, Amount: int32(next)})
	}
}

// effCounter removes the targeted spell from the stack to its owner's
// graveyard. This is CR 608.2b's canonical case: if the target is no longer
// on the stack by the time this resolves (already resolved, or itself
// countered by an earlier effect in the same response), it is simply skipped
// rather than moved from wherever it now sits.
//
// Task 9 fix round 1 (Important 2): a spell cast for its flashback cost is
// exiled "any time it would leave the stack" (CR 702.33b), which includes
// being countered -- spellRestZone (rules/stack.go) covers every exit in
// resolveTop, but this primitive hard-coded the graveyard, so a countered
// flashbacked spell (Force of Will / Daze / Counterspell against Cabal
// Therapy) returned to the graveyard and could be flashbacked again. The
// cast flags are already readable here (effects/filter.go reads them the
// same way), so the destination is chosen the same way spellRestZone does.
//
// UnlessCost$ (Mana Leak, Spell Pierce, Daze, Rust Tick, Runeboggle) is the
// "counter target spell unless its controller pays {N}" shape -- a real
// mid-resolution ask since M2d-2 closed R-8. On the first pass the
// CONTROLLER OF THE COUNTERED SPELL (the object in c.Targets[0], per CR
// 119) is offered a KModes pay/decline decision and the resolution suspends;
// the answer re-enters this effect with Ctx.UnlessPay set, rules'
// resumeResolution (rules/resolution.go) having already paid the cost via
// payMana on an affordable "pay". "pay" therefore means the spell is NOT
// countered; "decline" -- including an affordable-looking "pay" that
// payMana reports it could not cover -- counters it.
//
// Unlike effCopySpellAbility, the default payer is the first target's
// controller. UnlessPayer$ is NOT read here. All 12 of the corpus's targeted
// Counter lines carrying it name TargetedController (9) or
// ThisTargetedController (3), matching that default; the other 22 lines are
// untargeted and fall back to c.Controller, which is the right player for
// only the 3 that say You. The remaining 19 name someone else -- 14
// Triggered* selectors (TriggeredSourceSAController x7, TriggeredActivator
// x4, TriggeredSpellAbilityController, TriggeredCardController,
// NonTriggeredCardController), Player x4 and RememberedController x1 -- and
// general payer selection is NOT implemented, so those ask the wrong player.
// Reality Smasher (eldrazi-stompy) is one of them. Counts are raw
// .cards/cardsfolder lines from GNU grep; see AGENTS.md for the commands and
// for the unsupported cost, switched and multi-target shapes.
//
// UnlessSwitched$ True inverts the whole ask -- paying CAUSES the counter --
// and is not implemented. The ask is therefore SUPPRESSED on those five
// corpus shapes rather than posed backwards, which keeps the unconditional
// counter they had before this ask existed. See .superpowers/ISSUES.md I-4.
func effCounter(h Host, c *Ctx, sa *cards.SA) {
	skip := false
	switched := strings.EqualFold(strings.TrimSpace(sa.Params["UnlessSwitched"]), "True")
	// RememberCountered$ True / RememberCounteredSA$ True append every object
	// this effect counters (spells AND ability objects) to Ctx.Remembered, in
	// stack order, so a SubAbility$ chained after the counter can count them:
	// Swift Silence's "Draw a card for each spell countered this way"
	// (SVar:X:Remembered$Amount) and Glen Elendra's Answer's "Create a 1/1
	// Faerie for each spell and ability countered this way"
	// (SVar:X:Count$RememberedSize). Forge spells the flag two ways because
	// its own Defined$ forms distinguish a remembered CARD from a remembered
	// SA; every consumer this build has counts Remembered, so both flags
	// append the countered object itself.
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberCountered"]), "True") ||
		strings.EqualFold(strings.TrimSpace(sa.Params["RememberCounteredSA"]), "True")
	if cost := strings.TrimSpace(sa.Params["UnlessCost"]); cost != "" && !switched {
		// fx42: take the answer into a local and clear c.UnlessPay BEFORE
		// handling it, so a NESTED unless-pay consumer reached below this one
		// in the same walk (a Counter or CopySpellAbility in this effect's Sub
		// chain) does not inherit this consumer's answered value — it must
		// pose its own ask. The only readers of Ctx.UnlessPay are this
		// primitive's top and effCopySpellAbility's top, and neither reads it
		// again afterwards, so clearing here confines the answer to the
		// primitive that asked for it.
		ans := c.UnlessPay
		c.UnlessPay = ""
		switch ans {
		case "pay":
			// Re-entry, paid: the spell resolves normally, so do NOT counter.
			skip = true
		case "decline":
			// Re-entry, declined: counter it below.
		default:
			// First pass: pose the pay decision to the controller of the
			// countered spell. Untargeted scripts fall back to c.Controller;
			// their explicit UnlessPayer selectors are not implemented.
			payer := c.Controller
			if len(c.Targets) > 0 {
				payer = PlayerOf(h, c, c.Targets[0])
			}
			if strings.TrimSpace(sa.Params["UnlessPayer"]) == "TriggeredCardController" {
				if p, ok := TriggeredCardController(h.Game(), c.TriggerContext, c.Remembered); ok {
					payer = p
				}
			}
			shown := unlessCostLabel(cost)
			d := &decision.Decision{Player: payer, Kind: decision.KModes,
				Min: 1, Max: 1, Source: c.Source, ResumeKind: "unless_pay",
				ResumeSA: sa, Prompt: "Pay " + shown + " to save the spell, or decline",
				Options: []decision.Option{
					{Index: 0, Kind: "mode", Label: "Pay " + shown + " — don't counter", Obj: c.Source, Player: payer},
					{Index: 1, Kind: "mode", Label: "Don't pay", Obj: c.Source, Player: payer},
				}}
			if Ask(h, d) == AskAsked {
				return // resolution suspended; the answer re-enters this effect.
			}
			// Fuzz/no-engine host: the deterministic decline (R-9). The pay
			// was never posed, so resolve as if the player declined: counter.
			// (AskEmpty is unreachable by construction -- Min == Max == 1 over
			// two options -- but the shared helper owns the guard either way.)
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "may pay declined (UnlessCost not asked on this host)"})
		}
	}
	if skip {
		return
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZStack {
			continue
		}
		if o.Ability != nil {
			// CR 701.5a: to counter a spell or ability is to cancel it,
			// removing it from the stack so it never resolves. An ability is
			// not a card and has no graveyard to move to -- this is the same
			// "ceases to exist" rest every resolved ability already takes
			// (CR 608.2m, rules/stack.go's ability tail parks it in exile),
			// so a countered ability moves there, never to the graveyard.
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
				From: state.ZStack, To: state.ZExile, Text: "countered"})
			continue
		}
		to := state.ZGraveyard
		if o.CastFlags&state.FlagFlashback != 0 {
			to = state.ZExile
		}
		if remember {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZStack, To: to, Text: "countered"})
	}
}

// unlessCostLabel renders an UnlessCost$ value for the humans a
// decision.Decision can reach. A plain mana cost ("1", "3", "2 U", "R R") is
// already readable and comes back verbatim -- that is every repo-deck Counter
// with an UnlessCost$ except Mausoleum Wanderer and Reality Smasher.
// Everything else is raw Forge script: a bare SVar name (X, Y, Z, whose value
// this engine does not read at all) or a bracket form (Discard<1/Hand>,
// ExileFromGrave<1/All>, PayLife<5>). Those must not reach a player's screen,
// so they render as "the cost". Display only: the amount actually charged is
// still ParseCost(sa.Params["UnlessCost"]) in rules' resumeResolution, and
// AGENTS.md records what that substitution really costs.
func unlessCostLabel(cost string) string {
	fields := strings.Fields(cost)
	if len(fields) == 0 {
		return "the cost"
	}
	for _, f := range fields {
		if _, err := strconv.Atoi(f); err == nil {
			continue // generic amount
		}
		if strings.Trim(f, "WUBRGC") == "" {
			continue // colour/colourless symbols
		}
		return "the cost"
	}
	return cost
}

// effDelayedTrigger implements Mode$ Phase delayed triggers -- the
// "at the beginning of the next end step, return it" shape (Flickerwisp and
// its family, CR 603.7). It registers a delayed trigger by emitting a
// DelayedRegister event, which events.Apply folds into state.Game.Delayed so
// the registration survives replay (a delayed trigger is registered during
// one resolution and fires later, in general a different turn). The engine
// then, on entering the registered phase, mints a triggered-ability stack
// object for it through events.DelayedPush and it resolves like any other
// triggered ability.
//
// The registration carries the source object (whose face's SVar table holds
// the Execute$ sub-ability), the controller, the phase to fire in, the
// Execute$ SVar name, and the Remembered captured at registration -- which
// is what a later Defined$ DelayTriggerRememberedLKI resolves against when
// the delayed trigger fires (Flickerwisp's DelTrig remembers the exiled
// permanent via the ChangeZone's RememberChanged$ True, so TrigBounce knows
// which object to return).
//
// Only Mode$ Phase is implemented. The other Mode$ values (ChangesZone,
// SpellCast, ChangesController, DamageDone, AttackersDeclared) stay the
// deterministic Note-only recording, so a card that needs one still says
// what it intended without pretending to have fired.
func effDelayedTrigger(h Host, c *Ctx, sa *cards.SA) {
	mode := sa.Params["Mode"]
	if mode != "Phase" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "registers a delayed trigger at " + mode + " (not implemented)"})
		return
	}
	set, unknown := state.ParsePhases(sa.Params["Phase"])
	if len(unknown) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "registers a delayed trigger at unrecognized phase " + sa.Params["Phase"]})
		return
	}
	// One one-shot registration for the FIRST member of the set the game will
	// still reach (state.EarliestAfter): Forge's delayed trigger is removed
	// from TriggerHandler.delayedTriggers the moment it fires, so even a
	// multi-step Phase$ value (`Main1,Main2`, the open `Upkeep->` range) fires
	// exactly once, at the first listed phase still ahead -- and a single-step
	// value maps to the very step a registration used to carry, so every
	// already-working shape is unchanged.
	step, ok := state.EarliestAfter(set, h.Game().Step)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "registers a delayed trigger with no Phase"})
		return
	}
	exec := sa.Params["Execute"]
	if exec == "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "registers a delayed trigger with no Execute"})
		return
	}
	h.Emit(events.Event{Kind: events.DelayedRegister, Obj: c.Source,
		Player: c.Controller, Step: step, Counter: exec,
		IDs: encodeRemembered(c.Remembered), Text: sa.Params["Phase"]})
}

// encodeRemembered turns a Remembered target list into the []ObjID an event
// carries, PlayerRef-encoding a player target the same way rules.pushTrigger
// does (FL-41) so events.Apply's rememberedFrom decodes it back to a player
// target rather than a zero object id. It mirrors the rule in effects since
// effects cannot import rules.
func encodeRemembered(remembered []state.Target) []state.ObjID {
	var out []state.ObjID
	for _, t := range remembered {
		if t.IsPlayer {
			out = append(out, state.PlayerRef(t.Player))
			continue
		}
		out = append(out, t.Obj)
	}
	return out
}

// effRepeat runs RepeatSubAbility$ MaxRepeat$ times -- the fetched corpus's
// real parameter name. RepeatNum$ (the Task 18 brief's name, which real
// cards never use) is still honoured, as a fallback for anything that
// predates MaxRepeat$. Either way the run count goes through Num(), so an
// SVar-indirected Count$ works for either name. It is capped at 1000 so a
// malformed or absurdly large repeat can never spin the engine.
func effRepeat(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "MaxRepeat", -1)
	if n < 0 {
		n = Num(h, c, sa, "RepeatNum", 1)
	}
	if n < 0 {
		n = 0
	}
	if n > 1000 {
		n = 1000
	}
	name := sa.Params["RepeatSubAbility"]
	if name == "" || c.SVars == nil {
		return
	}
	sub := cards.ResolveSVar(c.SVars, name)
	if sub == nil {
		return
	}
	for i := int32(0); i < n; i++ {
		Resolve(h, c, sub)
	}
}

// CharmModeBounds resolves a Charm's selectable range. Forge defaults
// MinCharmNum$ to CharmNum$, but an explicit MinCharmNum$ permits choosing
// fewer modes. Both values use Num so literal, SVar, and inline Count$ forms
// share the same evaluation in spell, trigger, and resolution paths.
func CharmModeBounds(h Host, c *Ctx, sa *cards.SA, choices int) (min, max int) {
	max = int(Num(h, c, sa, "CharmNum", 1))
	if max < 1 {
		max = 1
	}
	min = max
	if _, ok := sa.Params["MinCharmNum"]; ok {
		min = int(Num(h, c, sa, "MinCharmNum", int32(min)))
	}
	if max > choices {
		max = choices
	}
	if min < 0 {
		min = 0
	}
	return min, max
}

// effCharm runs the selected Choices$ sub-abilities in chosen order.
// Cast spells (CR 601.2b) and triggered abilities (CR 603.3c) arrive with
// Ctx.Modes pre-seeded from their earlier announcement. A Charm reached only
// during resolution still poses KModes and suspends until resumeResolution
// re-enters it with Ctx.Modes. A host that cannot ask retains the deterministic
// first-mode stand-in and records why with a Note.
func effCharm(h Host, c *Ctx, sa *cards.SA) {
	if c.SVars == nil {
		return
	}
	choices := strings.Split(sa.Params["Choices"], ",")
	if len(choices) == 0 {
		return
	}
	for i := range choices {
		choices[i] = strings.TrimSpace(choices[i])
	}
	// Re-entry after the modal choice was answered: Ctx.Modes already names
	// the chosen SVars in execution order, so run exactly those and do not
	// ask again.
	if c.Modes != nil {
		// fx41: take the names into a local and clear c.Modes BEFORE running
		// them. The same Ctx is handed to Resolve for every mode AND to the
		// Charm's own SubAbility$, and nothing else in the walk reads Modes,
		// so an uncleared field would leak the OUTER Charm's answered modes
		// into a NESTED Charm reached anywhere below it -- that inner
		// effCharm sees Modes != nil, takes this re-entry branch, and "runs"
		// the outer's mode names against its own SVars instead of posing its
		// own ask (or, when a name resolves back to a chain containing it,
		// re-resolves itself endlessly). Clearing here confines the answer
		// to the Charm that asked for it.
		names := c.Modes
		c.Modes = nil
		for _, name := range names {
			if sub := cards.ResolveSVar(c.SVars, name); sub != nil {
				Resolve(h, c, sub)
			}
		}
		return
	}
	// Subs is resolved once per choice, so the label (SpellDescription$ on
	// the choice's own SVar body) and the mode-run share one parse; the
	// ordering of options mirrors Choices$ order, which is also how the
	// engine maps a chosen index back to an SVar name.
	subs := make([]*cards.SA, len(choices))
	for i, name := range choices {
		subs[i] = cards.ResolveSVar(c.SVars, name)
	}
	min, max := CharmModeBounds(h, c, sa, len(choices))
	if min > len(choices) {
		// Forge declines a Charm whose required minimum exceeds its available
		// modes. A no-engine host must likewise make no arbitrary choice.
		return
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KModes,
		Min: min, Max: max, Source: c.Source,
		ResumeKind: "modes", ResumeSA: sa,
		Prompt: "Choose " + strconv.Itoa(min) + " to " + strconv.Itoa(max) + " mode(s)"}
	for i, name := range choices {
		label := name
		if subs[i] != nil {
			if d := strings.TrimSpace(subs[i].Params["SpellDescription"]); d != "" {
				label = d
			}
		}
		d.Options = append(d.Options, decision.Option{
			Index: i, Kind: "mode", Label: label, Obj: c.Source, Player: c.Controller})
	}
	if Ask(h, d) == AskAsked {
		return // resolution suspended; the answer re-enters this effect with Ctx.Modes set.
	}
	// Fuzz/no-engine host: the deterministic first-mode default (R-9), with
	// the Note that records why the richer path did not run. (AskEmpty is
	// unreachable by construction -- charmNum is clamped to >= 1 and
	// strings.Split never yields fewer than one choice -- but the shared
	// helper owns the guard either way.)
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
		Text: "chose its first mode (no engine host to ask)"})
	if subs[0] != nil {
		Resolve(h, c, subs[0])
	}
}

// effVote has each Defined$ player vote for the first Choices$ entry (M1's
// simplification -- a real vote is a per-player choice, Task 20's territory)
// and records one Note per vote rather than running any chosen mode: unlike
// Charm, the brief's own spec for Vote is "Note per vote", not "whatever the
// chosen mode emits".
func effVote(h Host, c *Ctx, sa *cards.SA) {
	first := ""
	if choices := sa.Params["Choices"]; choices != "" {
		first = strings.TrimSpace(strings.SplitN(choices, ",", 2)[0])
	}
	for _, t := range Defined(h, c, sa) {
		h.Emit(events.Event{Kind: events.Note, Player: PlayerOf(h, c, t), Text: "votes for " + first})
	}
}

// effBecomeMonarch records the game-level designation as an event so a
// conditional trigger observes it identically in the live game and on replay.
func effBecomeMonarch(h Host, c *Ctx, sa *cards.SA) {
	targets := Defined(h, c, sa)
	if len(targets) == 0 {
		return
	}
	h.Emit(events.Event{Kind: events.MonarchChange, Player: PlayerOf(h, c, targets[0])})
}

// effRestartGame ends the game as a draw. Actually restarting (leaving
// exiled permanents in play under the restarting player's control, per the
// real card text) is out of M1's scope; ending the match honestly rather
// than hanging or silently no-op-ing is the closest correct degradation.
//
// Ruling T22-k (fix round 2): Amount: 1 is required, not cosmetic --
// rules/sba.go's checkGameOver is not the only GameOver emitter in this
// tree, and Amount is the shape discriminator events.Apply's GameOver case
// reads (0 = win, 1 = draw; Task 22 fix round 1). Left at its zero value,
// this event's own Amount reads as "Amount 0", a win -- and Player is also
// left at its zero value, which validates as seat 0 -- so despite this
// function's name, its own comment and its own Text all saying "draw", the
// event it actually emitted a win for seat 0. Every other RestartGame-style
// primitive in this file already carries no Player of its own, so seat 0
// winning was never a deliberate choice anywhere in this file; it was
// simply the one call site nobody had reason to re-examine once Amount
// became meaningful.
func effRestartGame(h Host, c *Ctx, sa *cards.SA) {
	h.Emit(events.Event{Kind: events.GameOver, Amount: 1, Text: "game restarted: ended as a draw"})
}

// effMana implements "AB$ Mana": add Amount mana of Produced's colour(s) to
// the pool of each ManaRecipients player (the activating player unless
// Defined$ names another). Absorbed from Task 14's stopgap: the
// negative-Amount clamp is Ruling T14-f, kept verbatim for the same reason as
// DealDamage's -- events.Apply's ManaAdd case is a plain "+=", so an
// unclamped negative would drop the pool below zero instead of doing
// nothing.
//
// Two things are folded in on top of that. "Any"/"Combo Any" resolves to
// colourless rather than asking (a real choice awaits the milestone that
// makes every R-9 stand-in real; the real ask lives in rules, on both the
// activation path and the CR 605.3b triggered-mana path, which rewrite
// Produced to a single chosen colour before this primitive ever runs). A dual-producing ability such as "Add {R}{R}" is
// walked one symbol at a time rather than split on whitespace, since
// Produced$ carries no spaces of its own.
//
// The one thing this primitive must NEVER do is walk a value it does not
// understand. A "Combo R G" reaches effMana from a path with no colour
// chooser (the activation path substitutes the chosen colour in first), and
// walking it one rune at a time turned "Combo R G" into five stray
// colourless plus a red and a green -- o, m, b are not mana symbols. An
// unrecognised Produced$ value therefore emits nothing and records a Note
// naming it, following this repo's fail-closed convention (an unknown token
// never invents a value).
// effReplaceMana rewrites one in-flight ManaAdd event for a ProduceMana
// replacement. The surrounding rules code supplies the amount and colour in
// Ctx, then logs the rewritten ManaAdd; this effect itself has no game-state
// mutation to emit. ReplaceAmount multiplies the whole production.
// ReplaceType/ReplaceColor preserve its amount and replace only its colour;
// ReplaceMana is Forge's "one mana instead of any other type and amount"
// form (Damping Sphere, Contamination), so it sets the amount to exactly one
// as well as replacing the colour. For a choice-valued replacement
// (Any/Chosen), rules parks the ManaAdd and supplies the player's W/U/B/R/G
// answer in Ctx.ManaChoice; without a valid answer this pure effect fails
// closed rather than inventing colourless mana.
func effReplaceMana(_ Host, c *Ctx, sa *cards.SA) {
	if c == nil {
		return
	}
	if only := strings.TrimSpace(sa.Params["ReplaceOnly"]); only != "" && only != c.ManaType {
		return
	}
	if n := Num(nil, c, sa, "ReplaceAmount", 1); n > 0 {
		c.ManaAmount *= n
	}
	kind := strings.TrimSpace(sa.Params["ReplaceMana"])
	if kind != "" {
		c.ManaAmount = 1
	}
	if kind == "" {
		kind = strings.TrimSpace(sa.Params["ReplaceType"])
	}
	if kind == "" {
		kind = strings.TrimSpace(sa.Params["ReplaceColor"])
	}
	if kind == "" {
		return
	}
	switch strings.ToLower(kind) {
	case "white":
		kind = "W"
	case "blue":
		kind = "U"
	case "black":
		kind = "B"
	case "red":
		kind = "R"
	case "green":
		kind = "G"
	case "any", "chosen":
		kind = c.ManaChoice
	}
	if len(kind) == 1 && strings.ContainsRune(ManaSymbols, rune(kind[0])) {
		c.ManaType = kind
	}
}

func effMana(h Host, c *Ctx, sa *cards.SA) {
	produced := strings.TrimSpace(sa.Params["Produced"])
	if produced == "" || produced == "Any" || produced == "Combo Any" {
		produced = "C"
	}
	// A "Combo" head lists every colour the production may be taken in (CR
	// 107.5-style "any combination"). Forge asks for the combination; this
	// executor still degenerates to the FULL amount in EVERY listed colour --
	// the documented stand-in (the colour-choice ask is the M4 mana-choice
	// milestone) -- but that must not be the hard "unhandled Produced$" no-op
	// it was: Burnt Offering's Produced$ Combo B R added NOTHING. Chosen/
	// ComboChosen shapes (a remembered or chosen colour) still fail loudly --
	// they have no degenerate reading.
	produced = strings.TrimSpace(strings.TrimPrefix(produced, "Combo "))
	// Strip braces and spaces, then validate every remaining rune before any
	// of them reaches the pool: ComboChosen/ChosenColor/Special ... values
	// that do not name plain mana symbols fail closed instead of splitting
	// into garbage.
	runes := strings.NewReplacer("{", "", "}", "", " ", "").Replace(produced)
	for _, r := range runes {
		if !strings.ContainsRune(ManaSymbols, r) {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unhandled Produced$ " + produced})
			return
		}
	}
	amt := Num(h, c, sa, "Amount", 1)
	if amt < 0 {
		amt = 0
	}
	for _, p := range ManaRecipients(h, c, sa) {
		for _, r := range runes {
			h.Emit(events.Event{Kind: events.ManaAdd, Player: p,
				Counter: string(r), Amount: amt})
		}
	}
}

// ManaRecipients is the player or players a Mana SA adds its mana for. Forge's
// ManaEffect adds to getDefinedPlayersOrTargeted: with no Defined$ that is the
// activating player; with Defined$ it is each player the selector names, so
// Vernal Bloom's Defined$ TriggeredCardController gives the extra {G} to the
// tapped Forest's controller rather than to the enchantment's. An object
// selector names that object's controller (PlayerOf). A Defined$ that resolves
// to nobody adds nothing, as in Forge (SpellAbilityEffect.getDefinedPlayers has
// no activator fallback): Valleymaker's Defined$ ChosenPlayer must not hand the
// mana to its controller when no player was chosen.
func ManaRecipients(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	if strings.TrimSpace(sa.Params["Defined"]) == "" {
		return []state.PlayerID{c.Controller}
	}
	g := h.Game()
	var out []state.PlayerID
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		if int(p) >= len(g.Players) || slices.Contains(out, p) {
			continue
		}
		out = append(out, p)
	}
	return out
}
