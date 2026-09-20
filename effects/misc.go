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
	// Name$ is the effect's own display name (Sephiroth's emblem, Wrenn and
	// Six's): the log names the effect after it wherever this function would
	// otherwise print a bare mode list, and the registrations below carry it
	// into the continuous-effect registry so Stackable$ can dedup by it.
	effectName := strings.TrimSpace(sa.Params["Name"])
	// Stackable$ False (Wrenn and Six's emblem): the effect does not stack.
	// Forge's EffectEffect.createEffect skips creating a second effect when an
	// un-stackable one already exists. Forge's default is STACKABLE — the
	// corpus carries Stackable$ only as "False" (38 raw lines, no "True"), so
	// the dedup gate fires ONLY on an explicit "False": an absent key keeps
	// the stacking behaviour (en-Kor's "en-Kor Redirection" redirection
	// stacking is the point of the card). The dedup ask goes through
	// Host.ContinuousNamed so the registry, not this resolution, decides
	// whether the same named effect from this controller is active.
	if stackable, present := sa.Params["Stackable"]; present &&
		strings.EqualFold(strings.TrimSpace(stackable), "False") &&
		effectName != "" && h.ContinuousNamed(c.Controller, effectName) {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "effect not stacked (" + effectName + ")"})
		return
	}
	// The two move-driven lifetimes: ForgetOnMoved$ drops a remembered card
	// from the registered effect's set when it moves to the named zone;
	// ExileOnMoved$ ENDS the effect on such a move (Vines of Vastwood's
	// blinked target). Both ride the registrations below.
	forgetOn := strings.TrimSpace(sa.Params["ForgetOnMoved"])
	exileOn := strings.TrimSpace(sa.Params["ExileOnMoved"])
	// ForgetCounter$ <kind> (task vow1): a remembered card whose count of
	// that kind reaches zero after a counter-removal leaves the registered
	// effect's Remembered set. Both this and ForgetOnMoved$ ride every
	// registration below.
	forgetCounter := strings.TrimSpace(sa.Params["ForgetCounter"])
	// RememberLKI$ (Quicksilver Elemental's "RememberLKI$ Targeted"): the
	// effect remembers the TARGETED cards — "Targeted" (and Forge's bare
	// "True", which is Targeted in the corpus's spelling) is exactly the
	// set effectRemembered's default already captures, so the registered
	// grants below see it either way; the read pins the flag's presence so
	// the grant's Remembered does not depend on the RememberObjects$
	// default. Any other value (an LKI grammar this build does not model —
	// the LKI persistence a vanished card would need) is a loud Note.
	if rl := strings.TrimSpace(sa.Params["RememberLKI"]); rl != "" {
		switch rl {
		case "Targeted", "True":
		default:
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unmodelled Effect RememberLKI$ " + rl})
		}
	}
	remembered := effectRemembered(h, c, sa)
	// SetChosenNumber$ binds the Effect's number ONCE, here at creation,
	// against THIS resolution's own context: the trigger-time board (Torgal's
	// Count$Valid Dog.YouCtrl,Wolf.YouCtrl, Communal Brewing's
	// Count$CardCounters.INGREDIENT) or the fire-time snapshot (Wildgrowth
	// Archaic's TriggeredCard$Converge, tconverge1). The registered
	// replacement's body later reads the frozen number through the
	// Count$ChosenNumber head; a live re-read at entry time would answer a
	// different question. An unresolvable value is the fail-closed loud Note
	// plus a zero binding (which reads as zero everywhere).
	chosenNumber := int32(0)
	if v := strings.TrimSpace(sa.Params["SetChosenNumber"]); v != "" {
		n, ok := resolveCountOperand(h, c, v, 0)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unresolvable SetChosenNumber$ " + v})
		}
		chosenNumber = n
	}
	registered := false
	// Effect can also create a replacement rather than a layer restriction.
	// Forge stores its R: body behind an SVar name in ReplacementEffects$.
	// Keep the parsed event data in state (which cannot import cards) and the
	// body text for rules to resolve under this Effect's source context.
	for _, name := range strings.FieldsFunc(sa.Params["ReplacementEffects"], func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		event, params := parseReplacementLine(c.SVars, name)
		body := ""
		if with := replacementLineWith(params); with != "" {
			body = c.SVars[with]
		}
		// A LIVE replacement registration: a body this build's replacement
		// dispatcher actually resolves. DamageDone is the Taii Wakeen shape
		// (the body is a DB$ ReplaceEffect damage rewrite); Event$ Moved with
		// a PutCounter body is the "that creature enters with an additional
		// +1/+1 counter for each ..." family (torgal_a_fine_hound,
		// communal_brewing, wildgrowth_archaic, task wildgrowth1): the
		// Updated-shaped MoveZone dispatch already applies the original move,
		// fires entry triggers, then runs the body, and effPutCounter handles
		// ETB$ True on the entered object. Every OTHER Moved body (the
		// destination-changing ChangeZone/Tap/Clone family, 44 measured
		// files) and every Draw/ProduceMana/CreateToken body keeps its loud
		// Note: a half-modelled Replaced-result could LOSE the moved object.
		// The effect's own capture state rides every live registration:
		// Remembered (the trigger's RememberObjects$ card, what the body's
		// IsRemembered/Remembered$ specs and Count$ChosenNumber's neighbours
		// read), the two move-driven lifetimes (ExileOnMoved$ Stack ends the
		// effect exactly after the one entry it upgrades -- load-bearing:
		// without it the effect would upgrade EVERY later creature cast this
		// turn), and the frozen SetChosenNumber$ binding.
		if body != "" && (event == "DamageDone" ||
			(event == "Moved" && replacementBodyAPI(body) == "PutCounter")) {
			h.AddContinuous(state.ContinuousEffect{
				Source: c.Source, Controller: c.Controller,
				UntilEOT: effectUntilEOT(h, c.Source, dur), Duration: dur,
				Name:             effectName,
				Remembered:       remembered,
				ForgetOnMoved:    forgetOn,
				ExileOnMoved:     exileOn,
				ForgetCounter:    forgetCounter,
				ChosenNumber:     chosenNumber,
				ReplacementEvent: event, ReplacementParams: params, ReplacementBody: body,
			})
			registered = true
		} else if event != "" && body == "" && (replacementLineCantHappen(params) ||
			(event == "DamageDone" && replacementLinePrevents(params))) {
			// The bodyless CantHappen form (Mistrise Village's AntiMagic: the
			// Event$ Counter | ValidCard$ Card.IsRemembered | Layer$ CantHappen
			// R: the delayed Effect registers): stopping the event is the
			// complete replacement, the same shape printed R: lines take —
			// rules' effect-created scan matches it With-less. The remembered
			// set (the cast spell the trigger captured) rides the registration,
			// so the ValidCard$ IsRemembered gate scopes the promise to the
			// exact spell.
			// The bodyless Prevent$ True DamageDone form is the same idiom for
			// damage: full prevention IS the complete replacement (Selfless
			// Squire's RPrevent, and the Fog family's DB$ Effect bodies -- 131
			// measured carriers). The shared damage dispatch prevents through
			// damageReplacementPrevents and stores the prevention Note whose
			// Amount Mode$ DamagePreventedOnce triggers read.
			untilEOT := effectUntilEOT(h, c.Source, dur)
			if event == "DamageDone" && sa.Params["Duration"] == "" {
				// This family's oracle text is always "this turn" (Selfless
				// Squire, Kurbis, the Fog spells) and none of its bodyless lines
				// names Duration$: a prevent from a PERMANENT source with no
				// explicit Duration$ is a this-turn grant, not the Permanent
				// default the other shapes keep. An explicit Duration$ wins.
				untilEOT = true
			}
			h.AddContinuous(state.ContinuousEffect{
				Source: c.Source, Controller: c.Controller,
				UntilEOT: untilEOT, Duration: dur,
				Name:             effectName,
				Remembered:       remembered,
				ReplacementEvent: event, ReplacementParams: params,
			})
			registered = true
		} else if name != "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "continuous replacement unimplemented (" + name + ")"})
		}
	}
	for _, name := range strings.FieldsFunc(sa.Params["StaticAbilities"], func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
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
				grant.Name = effectName
				grant.UntilEOT = effectUntilEOT(h, c.Source, dur)
				grant.Remembered = remembered
				grant.Duration = dur
				grant.ForgetOnMoved = forgetOn
				grant.ExileOnMoved = exileOn
				grant.ForgetCounter = forgetCounter
				h.AddContinuous(grant)
				registered = true
			} else if len(params) > 0 {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "continuous effect " + mode + " unimplemented (" + what + ")"})
				registered = true
			}
		case "CantTarget", "CantRegenerate", "CantPreventDamage", "CantAttack", "CantSacrifice":
			// A COMPOUND IsRemembered spec (Card.IsRemembered+Creature) resolves
			// faithfully through the general filter now that it implements
			// IsRemembered (rules/layers.go restrictionApplies consults the
			// same matcher with the registered remembered set bound), so the
			// old "reject compounds, keep the Note" guard is gone: the
			// restriction registers for real.
			//
			// CantAttack/CantSacrifice additionally gate on the same parameter
			// whitelist the face-static readers (rules/layers.go
			// cantRestrictionParamsReadable) enforce: a body carrying a
			// condition or scoping this build does not evaluate (UnlessCost$,
			// ValidCause$, ForCost$, IsPresent$, ...) must not register
			// blanket — it is reported unimplemented instead, so the two
			// registration paths cannot disagree about what is readable.
			if (mode == "CantAttack" || mode == "CantSacrifice") && !CantRestrictionParamsReadable(params) {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "continuous effect " + mode + " unimplemented (" + what + ")"})
				registered = true
				break
			}
			ce := state.ContinuousEffect{
				Source:         c.Source,
				Controller:     c.Controller,
				Name:           effectName,
				UntilEOT:       effectUntilEOT(h, c.Source, dur),
				Restriction:    mode,
				RestrictParams: params,
				Remembered:     remembered,
				Duration:       dur,
				ForgetOnMoved:  forgetOn,
				ExileOnMoved:   exileOn,
				ForgetCounter:  forgetCounter,
			}
			if mode == "CantAttack" || mode == "CantSacrifice" {
				// The player half of the remembered capture: Call for Aid's
				// RememberObjects$ TargetedPlayer must reach the registered
				// CantAttack, whose Target$ Player.IsRemembered ("you can't
				// attack that player") resolves against this set at
				// consultation time (rules/layers.go
				// restrictionPlayerSpecMatches) — effectRemembered records
				// objects only, so without this the remembered player would
				// silently vanish.
				ce.RememberedPlayers = effectRememberedPlayers(h, c, sa)
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
		if effectName != "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "registers a continuous effect " + effectName + " (" + what + ") for " + dur})
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "registers a continuous effect (" + what + ") for " + dur})
		}
	}
}

// mayPlayGrantFromLine builds the may-play ContinuousEffect from one parsed
// static line (an SVar static body effEffect registers, or the S: line rules
// passes through MayPlayStaticParams). ok=false is the fail-closed grant:
// nothing is registered rather than a half-read grant going live.
func mayPlayGrantFromLine(params map[string]string) (state.ContinuousEffect, bool) {
	ignoreColor, ignoreType, limit, playerTurn, ok := MayPlayStaticParams(params)
	if !ok {
		return state.ContinuousEffect{}, false
	}
	return state.ContinuousEffect{
		Affects:            params["Affected"],
		AffectedZone:       strings.TrimSpace(params["AffectedZone"]),
		MayPlay:            true,
		MayPlayIgnoreColor: ignoreColor,
		MayPlayIgnoreType:  ignoreType,
		MayPlayLimit:       limit,
		MayPlayPlayerTurn:  playerTurn,
	}, true
}

// MayPlayStaticParams reports whether a Mode$ Continuous static body (an S:
// line or an SVar static an Effect SA registers) carries the may-play grant
// this build implements, and resolves its readable riders. The
// implemented shape is MayPlay$ True plus an Affected$/AffectedZone$ pair and
// only display/placement metadata; MayPlayIgnoreColor$ (mana as any colour),
// MayPlayIgnoreType$ (mana as any type -- Rakdos, the Muscle's rider: the
// colour widening plus {C} pips payable by any colour), MayPlayLimit$ (an
// integer once-per-turn cap) and Condition$ PlayerTurn
// ("during each of your turns", the Kess/Karador family) are read. Anything
// else -- MayPlayWithoutManaCost$/MayPlayText$ (they change what the cast IS,
// not just where it may come from), a Condition$ whose value is not
// PlayerTurn, a ValidAfterStack$/Secondary$ qualifier (it changes when the
// grant lives), or a MayPlayLimit$ value that is not a non-negative integer
// -- fails closed:
func MayPlayStaticParams(params map[string]string) (ignoreColor, ignoreType bool, limit int32, playerTurn bool, ok bool) {
	v, okv := params["MayPlay"]
	if !okv || !strings.EqualFold(strings.TrimSpace(v), "True") {
		return false, false, 0, false, false
	}
	for key := range params {
		switch key {
		case "Mode", "MayPlay", "MayPlayIgnoreColor", "MayPlayIgnoreType",
			"MayPlayLimit", "Condition", "Affected", "AffectedZone", "Description", "EffectZone":
			// The keys the implemented grant (and only it) carries.
		default:
			return false, false, 0, false, false
		}
	}
	limit = 0
	if raw, okv := params["MayPlayLimit"]; okv {
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
		if err != nil || n < 0 {
			// A MayPlayLimit$ value this build cannot enforce must not
			// silently become "unlimited".
			return false, false, 0, false, false
		}
		limit = int32(n)
	}
	playerTurn = strings.EqualFold(strings.TrimSpace(params["Condition"]), "PlayerTurn")
	if cond, okv := params["Condition"]; okv && !playerTurn {
		// A Condition$ other than PlayerTurn changes when the grant lives;
		// never register it half-read.
		_, _ = cond, okv
		return false, false, 0, false, false
	}
	ignoreColor = strings.EqualFold(strings.TrimSpace(params["MayPlayIgnoreColor"]), "True")
	ignoreType = strings.EqualFold(strings.TrimSpace(params["MayPlayIgnoreType"]), "True")
	return ignoreColor, ignoreType, limit, playerTurn, true
}

// parseStaticLine parses an S: static body an SVar holds ("Mode$ CantTarget |
// ValidTarget$ Card.IsRemembered | ...") into its mode and parameter map. The
// body has no SP$/AB$/DB$ head, so cards' parseSA is the wrong shape; this is
// the S: line's own grammar (cards/parse.go's "S" case). An empty or
// malformed body degrades to "" mode and a nil map, which the switch in
// effEffect treats as unimplemented rather than as a registration.
// parseReplacementLine parses an Effect's SVar replacement body ("Event$
// DamageDone | ...") using the same key/value grammar as parseStaticLine.
func parseReplacementLine(svars map[string]string, name string) (string, map[string]string) {
	body := strings.TrimSpace(svars[name])
	if body == "" {
		return "", nil
	}
	params := make(map[string]string)
	for _, seg := range strings.Split(body, "|") {
		key, val, ok := strings.Cut(strings.TrimSpace(seg), "$")
		if !ok {
			continue
		}
		params[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return params["Event"], params
}

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
// then applies to nothing. The player half of the same capture lives in
// effectRememberedPlayers below.
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
		case "ReplacedCard":
			// The card the enclosing replacement acted on (Opposition Agent's
			// RepExile → DBEffect: the found card the replacement just exiled
			// is the one the may-play grant remembers). Outside a replacement
			// (c.Replaced zero) or after the object ceased to exist, nothing.
			if c.Replaced != 0 && h.Game().Obj(c.Replaced) != nil {
				out = append(out, c.Replaced)
			}
		case "TriggeredCard":
			// The card the firing trigger's event captured (Mistrise Village's
			// Effect RememberObjects$ TriggeredCard: the spell the can't-be-
			// countered promise covers). The SpellCast referent capture binds
			// c.TriggerCard to the cast stack object; a stale id (the spell
			// already resolved) remembers nothing, the same live-object
			// discipline the cases above apply.
			if c.TriggerCard != 0 && h.Game().Obj(c.TriggerCard) != nil {
				out = append(out, c.TriggerCard)
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

// effectRememberedPlayers resolves RememberObjects$ into the concrete PLAYER
// ids the Effect captured — the player half of effectRemembered, which
// deliberately records objects only (a player-only remember yields an empty
// slice there). Only the player-flavoured RememberObjects$ spellings are
// read: "TargetedPlayer" (the chosen player targets — Call for Aid's
// "target opponent", whose remembered self the registered CantAttack's
// Target$ Player.IsRemembered then resolves), "RememberedPlayer"/
// "RememberedPlayers" (the resolution's remembered players). Anything else
// contributes no player, so an effect whose remember the helper cannot read
// registers a restriction with an empty player set (its IsRemembered target
// clauses match nobody — fail closed). Deduplicated, first-capture order.
func effectRememberedPlayers(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	ro := sa.Params["RememberObjects"]
	if ro == "" {
		return nil
	}
	var out []state.PlayerID
	seen := make(map[state.PlayerID]bool)
	add := func(p state.PlayerID) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, part := range strings.FieldsFunc(ro, func(r rune) bool {
		return r == '&' || r == ',' || r == ' '
	}) {
		part = strings.TrimSpace(part)
		switch part {
		case "TargetedPlayer":
			for _, t := range c.Targets {
				if t.IsPlayer {
					add(t.Player)
				}
			}
		case "RememberedPlayer", "RememberedPlayers":
			for _, t := range c.Remembered {
				if t.IsPlayer {
					add(t.Player)
				}
			}
		}
	}
	return out
}

// CantRestrictionParamsReadable is the parameter whitelist a CantAttack /
// CantSacrifice static must pass before this build enforces it — used BOTH by
// the face-static readers (rules/layers.go's SacrificeBlocked/attackBlocked
// activeStatics walks) and by effEffect's registration case (an Effect body
// carrying an unreadable parameter must not register blanket, so the two
// registration paths cannot disagree about what is readable): Mode$, the
// ValidCard$ object spec, the Target$ player spec, and display text only.
// A static carrying any other parameter (UnlessDefender$, IsPresent$,
// Cost$, CheckSVar$, ValidSA$, ...) names a condition or scoping this build
// does not evaluate; enforcing it blanket would OVER-restrict — a "can't
// attack unless ..." would become "can't attack at all", and a creature a
// MustAttack static requires could be left without a single legal pair — so
// the static is skipped/reported, which is the pre-registration behaviour and
// the permissive direction for a restriction. Secondary$ is allowed: it marks
// a Forge-side duplicate for modifier composition, and a boolean restriction
// cannot be applied twice.
func CantRestrictionParamsReadable(params map[string]string) bool {
	for k := range params {
		switch k {
		case "Mode", "ValidCard", "Target", "Description", "Secondary":
		default:
			return false
		}
	}
	return true
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

// replacementLineWith reads ReplaceWith$ off a parseReplacementLine-built
// static line -- the SVar name of the R: body's own ReplaceWith$ body, not a
// card Params map. Factored into its own function so the paramcensus rot
// guard can classify the read through a tracked helper parameter rather than
// an unclassified local.
func replacementLineWith(params map[string]string) string {
	return params["ReplaceWith"]
}

// replacementLineCantHappen reports whether a parseReplacementLine-built
// replacement body declares Layer$ CantHappen with no body of its own -- the
// complete replacement is stopping the event (Mistrise Village's AntiMagic).
// Factored into its own function so the paramcensus rot guard can classify
// the read through a tracked helper parameter rather than an unclassified
// local, the same shape replacementLineWith takes.
func replacementLineCantHappen(params map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(params["Layer"]), "CantHappen")
}

// replacementLinePrevents reports whether a parseReplacementLine-built
// replacement body is the bodyless full-prevention form: Prevent$ True with
// no ReplaceWith$ body of its own -- the complete replacement is stopping
// the damage (Selfless Squire's RPrevent, task dponce1). Factored into its
// own function so the paramcensus rot guard can classify the read through a
// tracked helper parameter rather than an unclassified local, the same shape
// replacementLineCantHappen takes.
func replacementLinePrevents(params map[string]string) bool {
	return strings.EqualFold(params["Prevent"], "True")
}

// replacementBodyAPI names the API a retained replacement body's head
// resolves to ("DB$ PutCounter | Defined$ ReplacedCard | ..." ->
// "PutCounter"), so an effEffect registration gate can admit exactly the
// body shapes the rules dispatcher handles without hard-coding card names.
// An unparsable body returns "" (and the gate declines it).
func replacementBodyAPI(body string) string {
	head, _, _ := strings.Cut(body, "|")
	kind, api, ok := strings.Cut(strings.TrimSpace(head), "$")
	if !ok || strings.TrimSpace(kind) == "" || strings.TrimSpace(api) == "" {
		return ""
	}
	return strings.TrimSpace(api)
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
	// Forge's CleanUpEffect: ClearRemembered$ True clears the host card's
	// remembered list (the persistent list the next resolution of this card
	// reads -- without this an activated ability that remembers would
	// accumulate across activations). The ctx-level list is cleared with it:
	// every consumer downstream of this point in the chain (and the next
	// resolution) must see an empty list, which is what Forge's host
	// list clear produces. The clear is recorded as a real event ONLY when
	// the source's list actually held entries -- clearing an empty list is
	// a no-op, and emitting for it would move every chain head that carries
	// a ClearRemembered$ cleanup for no observable change (measured: Delver
	// of Secrets' DBCleanup in the 4/6/8-seat golden games runs its cleanup
	// with an empty list).
	noted := false
	if strings.EqualFold(sa.Params["ClearRemembered"], "True") {
		c.Remembered = nil
		if c.Source != 0 {
			if o := h.Game().Obj(c.Source); o != nil && len(o.Remembered) > 0 {
				// A real clear: the event is what a replay folds, so the next
				// resolution of this card sees the empty list.
				h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "clear-remembered"})
				noted = true
			}
		}
	}
	// ClearChosenCard$ / ClearChosenPlayer$ (Party Thrasher's DBClearChosen,
	// Wishclaw Talisman's DBCleanup, Vial Smasher's): the same discipline as
	// ClearRemembered for Forge's chosen-card / chosen-player fields -- the
	// persistent lists a later resolution's Card.ChosenCard/ChosenPlayer
	// predicates read. Only a real clear emits; an empty-list clear (the
	// overwhelmingly common case for one-shot effects) stays a no-op so no
	// golden game gains an event for nothing.
	if strings.EqualFold(sa.Params["ClearChosenCard"], "True") {
		c.Chosen = keepChosenPlayers(c.Chosen)
		if c.Source != 0 {
			if o := h.Game().Obj(c.Source); o != nil && hasChosenCards(o.Chosen) {
				h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "clear-chosen-card"})
				noted = true
			}
		}
	}
	if strings.EqualFold(sa.Params["ClearChosenPlayer"], "True") {
		c.Chosen = keepChosenCards(c.Chosen)
		if c.Source != 0 {
			if o := h.Game().Obj(c.Source); o != nil && hasChosenPlayers(o.Chosen) {
				h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "clear-chosen-player"})
				noted = true
			}
		}
	}
	// The cosmetic fallback (an empty-list clear, or a Cleanup with nothing
	// to clear): the Note main has always emitted, byte-for-byte, so golden
	// games whose cleanups run on empty lists replay identically. This also
	// covers main's independent Valakut concern: Valakut's DBCleanup runs
	// after DBEffect captured the dig's RememberChanged list into the
	// registered Effect, so the end-step trigger's own X=Remembered$Amount
	// must count only what IT moved -- c.Remembered is unconditionally
	// cleared above regardless of whether the source object held a
	// persisted list to clear too.
	if !noted {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "clears remembered/imprinted objects"})
	}
}

// hasChosenCards reports whether the chosen list holds any object entries.
func hasChosenCards(ts []state.Target) bool {
	for _, t := range ts {
		if !t.IsPlayer {
			return true
		}
	}
	return false
}

// hasChosenPlayers reports whether the chosen list holds any player entries.
func hasChosenPlayers(ts []state.Target) bool {
	for _, t := range ts {
		if t.IsPlayer {
			return true
		}
	}
	return false
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
// "counter target spell unless its controller pays {N}" shape. It rides the
// ONE shared unless gate (effects.Resolve's unlessProceed dispatch, shared
// by every API): the payer comes from UnlessPayer$ (effects.UnlessPayers
// resolves every corpus selector form; a named selector whose binding is
// unavailable declines rather than asking an unrelated player), and the
// unqualified default — the first target's controller per CR 119 — is
// exactly what the corpus's 12 targeted Counter lines name (Targeted
// Controller x9, ThisTargetedController x3). The pay/decline labels for a
// Counter's ask live in poseUnlessAsk's Counter arm. "pay" means the spell
// is NOT countered; "decline" — including an affordable-looking "pay" the
// payment path could not cover — counters it.
//
// UnlessSwitched$ True inverts the whole ask — paying CAUSES the counter —
// and is real switched semantics through the same gate; the orientation is
// read from the SA, not hardcoded here.
func effCounter(h Host, c *Ctx, sa *cards.SA) {
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberCountered"]), "True") ||
		strings.EqualFold(strings.TrimSpace(sa.Params["RememberCounteredSA"]), "True")
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZStack {
			continue
		}
		// AddsNoCounter$ mana (Cavern of Souls): a spell paid with that mana
		// carries state.FlagNoCounter and can't be countered — it stays on the
		// stack and resolves (CR 608.2b's removal never happens). The spell is
		// still a legal TARGET (CR: "can't be countered" does not stop
		// targeting), so the record is one loud Note naming the object, and
		// the Counter's remaining targets (and SubAbility$ chain) run on.
		if o.CastFlags&state.FlagNoCounter != 0 {
			h.Emit(events.Event{Kind: events.Note, Obj: o.ID, Text: "can't be countered"})
			continue
		}
		if !h.CounterAllowed(o.ID, c.Source) {
			h.Emit(events.Event{Kind: events.Note, Obj: o.ID, Text: "counter prevented"})
			if h.Suspended() {
				return // replacement order must settle before any later target/SA
			}
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
				eventRemember(h, c, o.ID)
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
				From: state.ZStack, To: state.ZExile, Text: "countered"})
			continue
		}
		// Destination$ (Remand's "into its owner's hand instead of into that
		// player's graveyard", Force of Will's explicit Graveyard): the zone a
		// countered CARD goes to instead of the default graveyard. Only the
		// three plain hand-off zones are honoured -- Battlefield (Desertion's
		// take-control), Library and the TopOfLibrary/BottomOfLibrary forms
		// (Memory Lapse) need control/library-position machinery a plain move
		// cannot express, so those record a Note and take the default rather
		// than moving a spell somewhere the card text never asked for.
		to := state.ZGraveyard
		if dest := strings.TrimSpace(sa.Params["Destination"]); dest != "" {
			switch dest {
			case "Hand", "Graveyard", "Exile":
				to, _ = parseZone(dest)
			default:
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "counter destination " + dest + " is not a plain hand-off zone; the card goes to the graveyard"})
			}
		}
		// CR 702.34a: a flashback spell is exiled instead of going anywhere
		// else when it leaves the stack -- but an explicit non-graveyard
		// destination (Remand's hand) is that anywhere-else, so the override
		// applies only on the graveyard/default path.
		if o.CastFlags&state.FlagFlashback != 0 && to == state.ZGraveyard {
			to = state.ZExile
		}
		if remember {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
			eventRemember(h, c, o.ID)
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
			From: state.ZStack, To: to, Text: "countered"})
	}
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
	if mode == "SpellCast" {
		effDelayedTriggerSpellCast(h, c, sa)
		return
	}
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
	// RememberObjects$ (Flickerwisp's and Necropotence's RememberedLKI)
	// names what the delayed trigger remembers when it fires. The
	// registration below ALWAYS captures the resolving chain's Remembered --
	// which is exactly what RememberedLKI means (the parent effect's captured
	// set, e.g. the exiled permanent RememberChanged$ put there) -- so the
	// read confirms the corpus's dominant value and changes nothing for it.
	// Every other value resolves through the Defined grammar (Targeted,
	// TriggeredAttackerLKICopy, the " & " joins, ...) and unions into the
	// same captured set, so a delayed trigger whose parent chain did not
	// remember its subjects still learns them; an unresolvable value is loud
	// rather than silently dropped.
	if spec := strings.TrimSpace(sa.Params["RememberObjects"]); spec != "" && spec != "RememberedLKI" {
		if ts, known := knownDefinedTargets(h, c, spec); known {
			for _, t := range ts {
				dup := false
				for _, have := range c.Remembered {
					if have == t {
						dup = true
						break
					}
				}
				if !dup {
					c.Remembered = append(c.Remembered, t)
				}
			}
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unmodelled DelayedTrigger RememberObjects$ " + spec})
		}
	}
	// NextTurn$ True (Mishra's/Urza's/Lodestone Bauble's slowtrip: "draw a
	// card at the beginning of the NEXT turn's upkeep"): the one-shot fires
	// in a LATER turn only. The decode folds Amount into the registration's
	// MinTurn (the same bound the ExtraTurn grant's registration carries),
	// so the first Upkeep still inside the current turn does not consume the
	// registration — the exact defect a bauble activated during its own
	// upkeep would otherwise hit. An absent flag keeps the unbounded fire
	// every earlier registration had (Amount zero).
	amount := int32(0)
	if strings.EqualFold(strings.TrimSpace(sa.Params["NextTurn"]), "True") {
		amount = int32(h.Game().Turn + 1)
	}
	// The registration's ValidPlayer$ rides the event's Text next to the
	// phase: "<Phase>|VP=<value>". The rules-side delayed scan gates the
	// fire on it at the phase occurrence (Necropotence's "YOUR next end
	// step" -- a phase the gate fails leaves the one-shot registration
	// pending for the first later occurrence that matches), and the view
	// layer strips the suffix for display.
	text := sa.Params["Phase"]
	if vp := strings.TrimSpace(sa.Params["ValidPlayer"]); vp != "" {
		text += "|VP=" + vp
	}
	// RememberChain$ False (this repo's own generated-SA param, the
	// Annihilator$-marker precedent: no raw corpus card carries it, only
	// cards/keywords.go's generated Mobilize delay SVar does): the
	// registration keeps only what THIS resolving chain itself remembered
	// beyond the referents its triggering event captured -- Ctx.Captured is
	// exactly the part of Remembered the trigger put there (the attacking
	// creature, the defending player), RememberTokens$ True put the minted
	// tokens in the chain part -- so Mobilize's end-step sacrifice touches
	// the Warrior tokens and never the creature that merely triggered. The
	// default (absent) keeps the whole-chain capture every earlier
	// registration had, byte for byte.
	remembered := c.Remembered
	if strings.EqualFold(strings.TrimSpace(sa.Params["RememberChain"]), "False") {
		chain := make([]state.Target, 0, len(c.Remembered))
		for _, t := range c.Remembered {
			captured := false
			for _, cp := range c.Captured {
				if cp == t {
					captured = true
					break
				}
			}
			if !captured {
				chain = append(chain, t)
			}
		}
		remembered = chain
	}
	h.Emit(events.Event{Kind: events.DelayedRegister, Obj: c.Source,
		Player: c.Controller, Step: step, Counter: exec, Amount: amount,
		IDs: encodeRemembered(remembered), Text: text})
}

// effDelayedTriggerSpellCast registers the event-matched delayed shape: a
// Mode$ SpellCast DelayedTrigger (Mistrise Village's "{U}, {T}: The next
// spell you cast this turn can't be countered") fires on a spell's
// PutOnStack exactly like checkEventDelayedTriggers' keyword-minted
// registrations do. The registration is one-shot (the DelayedPush that
// fires it removes it), so "the NEXT spell" is exactly one spell. The
// SA's own trigger clauses (ValidCard$, ValidActivatingPlayer$) are stored
// INLINE in the event's Text — a face Ability's DelayedTrigger has no SVar
// name of its own for the decode to reference — and the fire-time matcher
// re-parses them against the actual cast. ThisTurn$ True (Mistrise) bounds
// the registration to the CURRENT turn ("...you cast THIS TURN"): the
// expiry rides "|TT=<turn>" and folds into state.DelayedTrigger.MaxTurn;
// a turn that ends with the registration unfired leaves it inert forever
// (skipped, never removed — removal would need its own event). Static$
// True (the corpus's only value, 8 raw DelayedTrigger lines) marks Forge's
// static-style registration; every registration here is already
// source-independent once created (CR 603.7), so the gate below documents
// the carrier and a future non-True value gets the loud Note the
// fail-closed convention takes.
func effDelayedTriggerSpellCast(h Host, c *Ctx, sa *cards.SA) {
	if st := strings.TrimSpace(sa.Params["Static"]); st != "" && !strings.EqualFold(st, "True") {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unmodelled DelayedTrigger Static$ " + st})
	}
	exec := strings.TrimSpace(sa.Params["Execute"])
	if exec == "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "registers a delayed SpellCast trigger with no Execute"})
		return
	}
	body := "Mode$ SpellCast"
	// Each clause unrolled over its explicit key: the census's rot guard
	// refuses a dynamic Params key it cannot attribute, and four explicit
	// reads cannot hide one. Static$ rides the body too — Forge's static
	// delayed trigger resolves its Execute IMMEDIATELY at fire time (no
	// stack push), which is what makes Mistrise's promise active before the
	// opponent can respond.
	if v := strings.TrimSpace(sa.Params["ValidCard"]); v != "" {
		body += " | ValidCard$ " + v
	}
	if v := strings.TrimSpace(sa.Params["ValidActivatingPlayer"]); v != "" {
		body += " | ValidActivatingPlayer$ " + v
	}
	if v := strings.TrimSpace(sa.Params["PlayerTurn"]); v != "" {
		body += " | PlayerTurn$ " + v
	}
	if v := strings.TrimSpace(sa.Params["ValidSA"]); v != "" {
		body += " | ValidSA$ " + v
	}
	if v := strings.TrimSpace(sa.Params["Static"]); v != "" {
		body += " | Static$ " + v
	}
	text := "SpellCast:" + body
	if strings.EqualFold(strings.TrimSpace(sa.Params["ThisTurn"]), "True") {
		text += "|TT=" + strconv.Itoa(int(h.Game().Turn))
	}
	h.Emit(events.Event{Kind: events.DelayedRegister, Obj: c.Source,
		Player: c.Controller, Step: h.Game().Step, Counter: exec, Text: text})
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
// Optional$ True (Shadrix Silverquill's "you may choose two") lowers the
// minimum to 0: the election is real, and choosing nothing is a legal answer
// at every site that asks (the placement ask, the cast announcement and
// effCharm's own mid-resolution ask all share this one helper).
func CharmModeBounds(h Host, c *Ctx, sa *cards.SA, choices int) (min, max int) {
	max = int(Num(h, c, sa, "CharmNum", 1))
	if max < 1 {
		max = 1
	}
	min = max
	if _, ok := sa.Params["MinCharmNum"]; ok {
		min = int(Num(h, c, sa, "MinCharmNum", int32(min)))
	}
	if strings.EqualFold(sa.Params["Optional"], "True") {
		min = 0
	}
	if max > choices {
		max = choices
	}
	if min < 0 {
		min = 0
	}
	return min, max
}

// CharmUniqueNone/Supported/Unsupported classify a Charm's chosen-mode set
// against the cross-mode "each mode must target a different player" family
// (Shadrix Silverquill, the Tarkir/Ninja duo cycle, Balor, Vindictive Lich,
// Chaos Balor -- 8 corpus files, all trigger-side DB$ Charm).
type CharmUniqueStatus int

const (
	// CharmUniqueNone: fewer than two target-bearing chosen modes, or none
	// of them carries TargetUnique$ — the ordinary shared-target narrowing
	// applies, byte-identically to the pre-family engine.
	CharmUniqueNone CharmUniqueStatus = iota
	// CharmUniqueSupported: at least two target-bearing chosen modes, every
	// one of them single-target (no TargetMin$/TargetMax$ beyond 1), every
	// one targeting the SAME player-kind spec ("Player" or "Opponent"),
	// and at least one carrying TargetUnique$ True. The combined
	// different-player target ask is posed and the per-mode split applies.
	CharmUniqueSupported
	// CharmUniqueUnsupported: TargetUnique$ is present on the chosen modes
	// but some member the combined ask cannot serve — differing ValidTgts$
	// specs, a non-player spec, or multi-target bounds. The ordinary
	// narrowing keeps and a loud Note names the shape (never silent).
	CharmUniqueUnsupported
)

// charmUniquePlayerSpec reports whether a ValidTgts$ spec names players in
// the exact form every corpus carrier of the family uses. Wider player
// grammars are not served: a "You" spec could never satisfy two different
// players anyway, and a compound spec's candidates are not all players.
func charmUniquePlayerSpec(spec string) bool {
	return spec == "Player" || spec == "Opponent"
}

// charmUniqueBounds mirrors rules' targetBounds for the single-target check:
// absent TargetMin$/TargetMax$ mean 1..1 (the M1 single-target contract).
// Anything a caller set explicitly beyond 1..1 keeps the mode out of the
// combined ask.
func charmUniqueBounds(sa *cards.SA) (min, max int) {
	min, max = 1, 1
	if v, ok := sa.Params["TargetMin"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			min = n
		}
	}
	if v, ok := sa.Params["TargetMax"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			max = n
		}
	}
	if min < 1 {
		min = 1
	}
	if max < min {
		max = min
	}
	return min, max
}

// CharmCrossModeShape classifies a Charm's mode list for the cross-mode
// TargetUnique family, with a reason fragment for the Unsupported loud Note.
// It is deliberately a property of the CHARM's full
// Choices$ list, not of whichever subset a particular answer selected: the
// classification must be stable across the charm's whole lifetime, because
// the per-mode target split (effCharm) and the suspension re-entry
// (rules' resumeResolution) re-derive it after a mid-mode ask — and a
// continuation can carry only a suffix of the original chosen order.
// Measured at the corpus pin: every charm outside the 8-file family carries
// TargetUnique$ on NONE of its target-bearing modes (the anyUnique trigger
// below never fires for them), so they classify None and keep today's
// narrowing byte-identically.
func CharmCrossModeShape(svars map[string]string, modes []string) (CharmUniqueStatus, string) {
	var spec string
	tbms, anyUnique, oneSpec, boundsOK := 0, false, true, true
	for _, name := range modes {
		sub := cards.ResolveSVar(svars, strings.TrimSpace(name))
		if sub == nil {
			continue
		}
		s := strings.TrimSpace(sub.Params["ValidTgts"])
		if s == "" {
			continue
		}
		tbms++
		if strings.EqualFold(sub.Params["TargetUnique"], "True") {
			anyUnique = true
		}
		if spec == "" {
			spec = s
		} else if s != spec {
			oneSpec = false
		}
		if min, max := charmUniqueBounds(sub); min != 1 || max != 1 {
			boundsOK = false
		}
	}
	if tbms < 2 || !anyUnique {
		return CharmUniqueNone, ""
	}
	if !oneSpec {
		return CharmUniqueUnsupported, "differing ValidTgts$ specs across the target-bearing modes"
	}
	if !charmUniquePlayerSpec(spec) {
		return CharmUniqueUnsupported, "ValidTgts$ " + spec + " is not a player-kind spec"
	}
	if !boundsOK {
		return CharmUniqueUnsupported, "a target-bearing mode declares multi-target bounds"
	}
	return CharmUniqueSupported, ""
}

// charmCrossModeRun is effCharm's cross-mode TargetUnique family runner. It
// runs the chosen modes in order, giving each target-bearing mode its OWN
// target — the positional slice of Ctx.Targets the combined placement ask
// recorded — instead of the one-undivided target list every mode shared
// before. A mode that suspends (donnie's and mikey's hidden graveyard pick)
// stops the run and reports the remaining modes as a continuation
// (Host.SuspendCharmRest), so the rest re-enter through the answered ask's
// chain rather than running while the suspension is still outstanding.
// Non-target-bearing modes keep the shared context exactly as before.
// Returns false when the shape does not apply and the caller must keep the
// historical shared-target loop.
func charmCrossModeRun(h Host, c *Ctx, sa *cards.SA, names []string) bool {
	choices := strings.Split(sa.Params["Choices"], ",")
	for i := range choices {
		choices[i] = strings.TrimSpace(choices[i])
	}
	if status, _ := CharmCrossModeShape(c.SVars, choices); status != CharmUniqueSupported {
		return false
	}
	var tbmIdx []int
	for i, name := range names {
		if sub := cards.ResolveSVar(c.SVars, name); sub != nil && strings.TrimSpace(sub.Params["ValidTgts"]) != "" {
			tbmIdx = append(tbmIdx, i)
		}
	}
	k := len(tbmIdx)
	if k == 0 || len(c.Targets) < k {
		// The running mode set carries target-bearing modes the placement ask
		// could not serve (an insufficient-candidate fallback, or a spell-side
		// single-target ask): keep the shared list, exactly the historical
		// narrowing, rather than inventing an assignment the ask never made.
		return false
	}
	// Assignment: the j-th target-bearing mode of the RUNNING list takes
	// targets[len(targets)-k+j]. On a full run that is targets[j] — the
	// combined ask's answer order, which is the chosen-mode order. On a
	// suffix continuation the remaining target-bearing modes are the last
	// ones of the original order, so the last k targets are theirs.
	base := len(c.Targets) - k
	ti := 0
	for i, name := range names {
		sub := cards.ResolveSVar(c.SVars, name)
		if sub == nil {
			continue
		}
		if ti < k && tbmIdx[ti] == i {
			saved := c.Targets
			savedOffered := c.OfferedSA
			c.Targets = []state.Target{c.Targets[base+ti]}
			// The combined placement ask covered THIS mode's targeting (its
			// assignment is positional); mark it so the generic ValidTgts$
			// pre-ask does not re-pose the cross-mode question per mode --
			// both on the initial pass (where the resolution-level marker's
			// bool would also suppress it) and on a charm_rest resume, where
			// the resume ctx carries only the FIRST chosen mode as OfferedSA
			// (task mvts1).
			c.OfferedSA = sub
			Resolve(h, c, sub)
			c.OfferedSA = savedOffered
			c.Targets = saved
			ti++
		} else {
			Resolve(h, c, sub)
		}
		if h.Suspended() {
			// The mode's own chain posed a mid-resolution ask: stop here. The
			// remaining modes resume through SuspendCharmRest's continuation
			// once the answer lands — never while the suspension is live (the
			// historical loop ran them immediately, before the answered mode
			// had even completed).
			if rest := names[i+1:]; len(rest) > 0 {
				h.SuspendCharmRest(sa, rest)
			}
			return true
		}
	}
	return true
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
		if charmCrossModeRun(h, c, sa, names) {
			return
		}
		// Task mvts1, two guards the generic ValidTgts$ pre-ask needs here.
		//
		// Coverage: a placement-announced modal resolution asked every CHOSEN
		// target-bearing mode's targeting in its placement ask (the combined
		// per-mode ask), but a resume ctx carries only the FIRST of them as
		// Ctx.OfferedSA (rules' offeredTargetSA returns the first
		// target-bearing chosen mode). modalOffered detects that derivation
		// -- OfferedSA set and NOT the Charm root itself -- and marks each
		// target-bearing mode as covered while it dispatches, so the pre-ask
		// cannot re-pose the placement question per mode. A mid-resolution
		// Charm (its own KModes answered) has no modal derivation -- its
		// OfferedSA is nil or the root's own covered SA -- and its
		// target-bearing modes keep their real asks.
		modalOffered := c.OfferedSA != nil && c.OfferedSA.Line != sa.Line
		for i, name := range names {
			if sub := cards.ResolveSVar(c.SVars, name); sub != nil {
				savedOffered := c.OfferedSA
				if modalOffered && strings.TrimSpace(sub.Params["ValidTgts"]) != "" {
					c.OfferedSA = sub
				}
				Resolve(h, c, sub)
				c.OfferedSA = savedOffered
			}
			if h.Suspended() {
				// A mode's own chain posed a mid-resolution ask: never run the
				// remaining modes while a decision is pending (Engine.ask
				// panics on the overwrite). Report the rest as a charm-rest
				// continuation, the same report the cross-mode runner makes,
				// so they run once the answer lands.
				if rest := names[i+1:]; len(rest) > 0 {
					h.SuspendCharmRest(sa, rest)
				}
				return
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

// effVote records one Note per voting player. Two shapes:
//
//   - the fixed-list shape ("Will of the Planeswalkers", Expropriate):
//     Choices$ names an SVar per ballot option, each player votes for the
//     first (the deterministic stand-in), Notes record it, and the WINNING
//     option's SVar runs. A tie runs VoteTiedAbility$ when the SA carries
//     one, else the first tied option. Before this the fixed-list shape
//     resolved nothing at all, so a Path of the Ghosthunter vote recorded
//     its Notes and then did nothing -- the "chosen outcome" the brief
//     expected to hit Planeswalk/ChaosEnsues never ran. The per-player
//     vote CHOICE is still the deterministic no-ask stand-in (every voter
//     takes the first option), so the outcome resolution is exact for
//     today's model and a real ask slots in behind the same tally.
//   - the card-ballot shape (Council's Judgment): VoteCard$ is a permanent
//     filter, so the ballot is the battlefield permanents matching it
//     (matched from the spell's controller: "a nonland permanent YOU don't
//     control"), each Defined$ player votes, and every permanent with the
//     most votes or tied for most lands in the resolution's Remembered set
//     for VoteSubAbility$ (DBExile's ChangeZone Defined$ Remembered).
//
// The per-player vote CHOICE itself is still the deterministic no-ask
// stand-in (every voter takes the ballot's first option, so the first
// eligible permanent always wins unanimously): a real per-player vote ask
// needs a resume arm of its own and stays in the approximations table.
// Both VoteCard$ and VoteSubAbility$ are genuinely read on the ballot path.
func effVote(h Host, c *Ctx, sa *cards.SA) {
	if ballot := strings.TrimSpace(sa.Params["VoteCard"]); ballot != "" {
		effCardVote(h, c, sa, ballot)
		return
	}
	choices := voteChoiceNames(sa)
	voters := Defined(h, c, sa)
	// The deterministic stand-in: every voter takes the first option. The
	// tally is general anyway so a future real per-player ask only has to
	// fill counts; today counts[0] == len(voters) and every other entry 0.
	counts := make([]int, len(choices))
	for _, t := range voters {
		label := ""
		if len(choices) > 0 {
			label = choices[0]
			counts[0]++
		}
		h.Emit(events.Event{Kind: events.Note, Player: PlayerOf(h, c, t), Text: "votes for " + label})
	}
	if len(choices) == 0 || len(voters) == 0 {
		return
	}
	// The winner is the option with the most votes (ties: the first such
	// option). When the top count is shared, VoteTiedAbility$ runs instead
	// for the shapes that spell one (the Path cycle's DBChaos).
	best, tied := voteWinner(counts)
	name := choices[best]
	if tied {
		if alt := strings.TrimSpace(sa.Params["VoteTiedAbility"]); alt != "" {
			name = alt
		}
	}
	if sub := cards.ResolveSVar(c.SVars, name); sub != nil {
		Resolve(h, c, sub)
	}
}

// voteWinner returns the index of the highest count and whether that count is
// shared by more than one option. It is a separate function (rather than
// inline in effVote) so the tie branch is testable on its own: the current
// deterministic stand-in gives every vote to option 0, so a real tie cannot
// arise from a live resolution yet, and an untested branch would be dead code
// waiting to rot. The first highest index wins the tie, matching the
// oracle's "if X gets more votes" over "or the vote is tied" ordering.
func voteWinner(counts []int) (int, bool) {
	if len(counts) == 0 {
		return 0, false
	}
	best := 0
	for i, n := range counts {
		if n > counts[best] {
			best = i
		}
	}
	tied := 0
	for _, n := range counts {
		if n == counts[best] {
			tied++
		}
	}
	return best, tied > 1
}

// voteChoiceNames splits a Vote's Choices$ into its SVar names, trimmed and
// with empty entries dropped. Shared by both vote shapes so the option list
// the tally indexes is parsed one way.
func voteChoiceNames(sa *cards.SA) []string {
	raw := sa.Params["Choices"]
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// effCardVote is effVote's card-ballot half: the battlefield permanents
// VoteCard$ admits are the options, each voting player takes the ballot's
// first option (the deterministic stand-in), and the most-voted -- every
// member of the tie -- is remembered for VoteSubAbility$, which runs once
// at the end (Council's Judgment's "exile each permanent with the most
// votes or tied for most votes").
func effCardVote(h Host, c *Ctx, sa *cards.SA, ballot string) {
	g := h.Game()
	var options []state.ObjID
	for i := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(i)) {
			if o := g.Obj(id); o != nil && MatchesSpecFrom(g, ballot, id, c.Controller, c.Source) {
				options = append(options, id)
			}
		}
	}
	counts := map[state.ObjID]int{}
	max := 0
	for _, t := range Defined(h, c, sa) {
		label := "nothing"
		if len(options) > 0 {
			if o := g.Obj(options[0]); o != nil && o.Face() != nil {
				label = o.Face().Name
			}
			counts[options[0]]++
			if counts[options[0]] > max {
				max = counts[options[0]]
			}
		}
		h.Emit(events.Event{Kind: events.Note, Player: PlayerOf(h, c, t), Text: "votes for " + label})
	}
	if max > 0 {
		for _, id := range options {
			if counts[id] == max {
				c.Remembered = append(c.Remembered, state.Target{Obj: id})
			}
		}
	}
	if sub := strings.TrimSpace(sa.Params["VoteSubAbility"]); sub != "" {
		if resolved := cards.ResolveSVar(c.SVars, sub); resolved != nil {
			Resolve(h, c, resolved)
		}
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
	// RestrictFromZone$/RestrictFromValid$ (Karn Liberated's [-14]): the
	// objects the restart would KEEP. Forge's RestartGame discards everything
	// in RestrictFromZone that matches RestrictFromValid and carries the rest
	// into the restarted game — Karn keeps exactly the non-Aura permanents
	// exiled with him (his ReturnFromExile sub-ability acts on that keep-set).
	// A full restart needs game-loop machinery this engine does not have (see
	// the draw degradation below), but the keep-set is real game state this
	// build can name, so the log records it instead of leaving both keys
	// silently inert.
	if zonesRaw := strings.TrimSpace(sa.Params["RestrictFromZone"]); zonesRaw != "" {
		if spec := strings.TrimSpace(sa.Params["RestrictFromValid"]); spec != "" {
			zones, all, valid := ParseZones(zonesRaw)
			g := h.Game()
			if !valid {
				all = true
			}
			if all {
				zones = []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield,
					state.ZGraveyard, state.ZExile, state.ZStack, state.ZCommand}
			}
			var kept []string
			for _, z := range zones {
				for _, p := range g.AliveFrom(0) {
					for _, id := range append([]state.ObjID(nil), g.Zone(z, p)...) {
						// RestrictFromValid$ names what the restart DISCARDS; the
						// complement inside the named zone is what it keeps.
						if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
							if o := g.Obj(id); o != nil && o.Face() != nil {
								kept = append(kept, o.Face().Name)
							}
						}
					}
				}
			}
			if len(kept) > 0 {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
					Text: "restart would keep in " + zonesRaw + ": " + strings.Join(kept, ", ")})
			}
		}
	}
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

var manaRuneNormalizer = strings.NewReplacer("{", "", "}", "", " ", "")

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
	// Special LastNotedType (Jeweled Amulet: "Add one mana of CARDNAME's
	// last noted type"): the production resolves to the colour the source's
	// last RememberCostMana$ activation paid with (events.Choose's
	// "noted-mana" marker folded into state.Object.LastNotedMana). With no
	// note yet the executor fails closed — the loud Note and no mana the
	// unhandled-Produced arm emits — which for the Amulet is unreachable
	// (its production cost removes the charge counter the noted activation
	// created).
	if strings.EqualFold(produced, "Special LastNotedType") {
		noted := ""
		if o := h.Game().Obj(c.Source); o != nil {
			noted = strings.TrimSpace(o.LastNotedMana)
		}
		if noted == "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unhandled Produced$ Special LastNotedType: no mana noted yet"})
			return
		}
		produced = noted
	}
	// Special EachColorAmong_Valid <spec> (Faeburrow Elder, Tarnation Vista's
	// second ability: "For each color among [matching] permanents you control,
	// add one mana of that color"): a deterministic BATCH, not a choice. The
	// matching permanents' colours (ColorsOf: mana cost or Colors: line,
	// Devoid-aware) are unioned and rendered in fixed WUBRG order, and the
	// tail's per-rune loop adds one unit per distinct colour. The spec is
	// evaluated over the battlefield exactly as ManaReflectedCandidates
	// evaluates its Valid$: MatchesSpecFrom with the resolving source as
	// Self and its controller as You. An empty colour set is a legitimate
	// deterministic no-op ("for each color" over none adds nothing) -- no
	// Note, no mana, like ChangeNum$ 0 Dig. Every OTHER Special selector
	// (EachColorAmong_ExiledWith, EnchantedManaCost, DoubleManaInPool,
	// EachColoredManaSymbol_Milled) still falls through to the rune gate's
	// loud Note below.
	if sel, ok := strings.CutPrefix(produced, "Special EachColorAmong_Valid "); ok {
		syms := eachColorAmongValid(h, c, strings.TrimSpace(sel))
		if syms == "" {
			return
		}
		produced = syms
	}
	produced = strings.TrimSpace(strings.TrimPrefix(produced, "Combo "))
	// Strip braces and spaces, then validate every remaining rune before any
	// of them reaches the pool: ComboChosen/ChosenColor/Special ... values
	// that do not name plain mana symbols fail closed instead of splitting
	// into garbage.
	runes := manaRuneNormalizer.Replace(produced)
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
	// RestrictValid$ (Master of Dark Rites' "Spend this mana only to cast
	// Vampire, Cleric, and/or Demon spells", Eldrazi Temple, Cavern of
	// Souls, Giada, Shrine of the Forsaken Gods): the produced mana carries
	// its spend restriction on the ManaAdd event itself, so the pool retains
	// the provenance per colour slot and the payment path
	// (manaAvailableFor / restrictValidMatches) can admit it only to
	// matching payments — the same event-level provenance the ManaReflected
	// family and the Tazri batch already ride. A class the payment path
	// cannot evaluate (anything but Spell./Activated.) is still retained --
	// it matches no payment, so the mana is never spendable, the
	// fail-closed direction.
	restriction := strings.TrimSpace(sa.Params["RestrictValid"])
	// AddsNoCounter$ (Cavern of Souls' "that spell can't be countered",
	// Boseiju, Delighted Halfling — 3 corpus files): the produced mana carries
	// its can't-be-countered provenance on the same ManaAdd restriction batch
	// the payment path already reads, so a cast that spends one of these units
	// is marked can't-be-countered at payment time (rules/stack.go captures
	// the consumption, rules/cast.go folds state.FlagNoCounter into the
	// pay-time CastInfo). "True" is the plain flag; Forge's conditional
	// "!Permanent" (Boseiju's instant-or-sorcery mana) is recognised as the
	// NotPermanent condition, evaluated against the paying spell's face. Any
	// other value is a loud Note and NO protection — an unrecognised condition
	// must not silently promise something the engine cannot model.
	noCounter := ""
	switch strings.TrimSpace(sa.Params["AddsNoCounter"]) {
	case "":
	case "True":
		noCounter = "True"
	case "!Permanent":
		noCounter = "NotPermanent"
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unhandled AddsNoCounter$ " + strings.TrimSpace(sa.Params["AddsNoCounter"]) + "; the mana is ordinary"})
	}
	// CR 107.4h: mana produced by a SNOW permanent is snow mana. A snow unit
	// is tagged in the pool event itself — Counter "S<colour>" — so the pool
	// slot and the parallel snow tally move through one event and a replay
	// derives both identically. The {S} pips a cost may carry are paid only
	// from that tally (rules/mana.go's resolveMana).
	snow := false
	if o := h.Game().Obj(c.Source); o != nil && o.Face() != nil {
		for _, t := range o.Face().Types {
			if t == "Snow" {
				snow = true
				break
			}
		}
	}
	for _, p := range ManaRecipients(h, c, sa) {
		for _, r := range runes {
			counter := string(r)
			if snow {
				counter = "S" + counter
			}
			ev := events.Event{Kind: events.ManaAdd, Player: p,
				Counter: counter, Amount: amt}
			if noCounter != "" {
				ev.Text = events.ManaRestrictionTextNC(restriction, c.Source, noCounter)
			} else if restriction != "" {
				ev.Text = events.ManaRestrictionText(restriction, c.Source)
			}
			h.Emit(ev)
		}
	}
}

// eachColorAmongValid resolves a Produced$ Special EachColorAmong_Valid <spec>
// selector: the union of the matching battlefield permanents' colours, in the
// controller-relative battlefield scan order (AliveFrom(0) x zone order, the
// same scan ManaReflectedCandidates uses), rendered in fixed WUBRG order via
// ColorMask's table. Colourless contributes nothing ("each color" never
// includes colourless). The spec is evaluated with the resolving source as
// Self and its controller as You, so a bare Permanent.YouCtrl always matches
// the resolving permanent itself while it is on the battlefield. No map
// iteration reaches the result: the union is a bitmask and the output order
// is the fixed WUBRG table.
func eachColorAmongValid(h Host, c *Ctx, spec string) string {
	g := h.Game()
	var mask ColorMask
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				mask |= ColorMaskOf(g.Obj(id))
			}
		}
	}
	return mask.String()
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
