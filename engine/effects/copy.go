package effects

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("CopySpellAbility", effCopySpellAbility)
}

// effCopySpellAbility duplicates the spell named by Defined$ (Task 17). Two
// shapes exist in the corpus:
//
//   - Defined$ Parent: the spell currently resolving (c.Source) — Chain
//     Lightning's own copy clause.
//   - Defined$ TriggeredSpellAbility: the trigger's remembered object, i.e.
//     the spell whose cast FIRED the trigger — Storm (engine/cards/keywords.go
//     expands kw:Storm into exactly this).
//
// Amount$ copies are placed on the stack by StackCopy events (default 1),
// each copy keeping its targets unless the creating SA declares
// MayChooseTarget$ True (CR 707.10c: that copy's controller may choose new
// targets). The permission rides the StackCopy event's Amount discriminator,
// which engine/rules/stack.go's resolveTop reads as a one-shot target election over
// the copy's own target requirement — so an external copier (Mirari, Cloven
// Casting, a Storm or Replicate copy) grants it too. Copies keep their
// targets only when the parameter is absent or False.
//
// NonLegendary$ True (The Sixth Doctor's Time Lord's Prerogative, and the
// corpus's six-carrier CopySpellAbility family: Osgood Operation Double, Iron
// Man Bleeding Edge, Jackal Genius Geneticist, Storm of Saruman, The Clone
// Saga) drops the Legendary supertype from the copy's characteristics, so a
// copied legendary PERMANENT spell resolves into a token CR 704.5j must not
// gather against its original. The rider rides the StackCopy event's Counter
// (a field the fold otherwise leaves empty) and is folded into the mint as
// Object.CopyNonLegendary, which rules' typeCharacteristics applies as a
// layer-4 base strip -- no event field is added or reordered. An absent or
// False key adds no strip, but a copy of a previously stripped copy inherits
// that copiable characteristic (CR 707.2).
//
// UnlessCost$ (Chain Lightning, String of Disappearances) rides the ONE
// shared unless gate (effects.Resolve's unlessProceed dispatch, shared by
// every API): the payer is UnlessPayer$'s resolved target (default the
// target's controller), the pay/decline labels live in poseUnlessAsk's
// CopySpellAbility arm, and the copy loop below runs, or not, exactly once
// per the gate's orientation (an affordable "pay" answer charged the cost).
// A host that cannot ask (an effects-package test
// double) keeps the deterministic decline.
func effCopySpellAbility(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	addTypes := CopySpellAbilityOf(sa).AddTypes
	// Optional$ True (Sevinne's Reclamation's "if this spell was cast from a
	// graveyard, you may copy this spell", the corpus's wider may-copy
	// family, and the Optional$+UnlessCost$ carriers Wandering Archaic and
	// Chain of Silence) makes the copy itself a may effect: the copy's
	// controller is asked a yes/no before any copy is made, and a decline
	// makes none; the election is answered in place via AskTape. An absent
	// key leaves the historical unconditional copy. A host that cannot ask
	// (an effects-package double, fuzz) keeps the deterministic pre-ask
	// behaviour -- the copy is made -- the no-host stand-in the election's
	// own branch implements below (the ask's bracket is Min == Max == 1, so
	// Clamp and the bot answer option 0 = yes).

	// Resolve which spell to copy. For a trigger the remembered entry is the
	// cast spell (the first object entry); for a direct Parent copy it is the
	// currently resolving spell itself.
	var spell state.ObjID
	// validStackSpells is the PLURAL result of a Defined$ ValidStack copy arm
	// (CR 707.10a, "copy each"): every stack object the spec admits, in
	// stack-arena order, each copied Amount$ times. It is nil for every other
	// Defined form, whose established single-spell shape is unchanged.
	var validStackSpells []state.ObjID
	switch effCopySpellAbilityCodes.Code(string(DefinedRefOf(sa).Text)) {
	case effCopySpellAbilityTriggeredSpellAbility:
		// The activation arm (abcopy1): the trigger context's TriggerAbility
		// names the minted ability wrapper -- an AbilityPush's Obj is the
		// source permanent, so Remembered alone cannot identify it -- and the
		// wrapper is on the stack, so the zone guard below passes and the copy
		// actually resolves. Role absent (the spell arm, where Remembered IS
		// the cast spell, and hand-built contexts) keeps the remembered entry.
		if id := c.TriggerAbility; id != 0 {
			spell = id
		} else {
			for _, t := range c.Remembered {
				if !t.IsPlayer && t.Obj != 0 {
					spell = t.Obj
					break
				}
			}
		}
	case effCopySpellAbilityParent:
		// An explicit Parent copy copies the resolving spell itself.
		spell = c.Source
	default: // "Targeted" and any unset/other name
		// An SA that names targets copies its TARGET: every corpus
		// CopySpellAbility line without Defined$ (66, Flare of Duplication
		// and the whole may-choose family) carries ValidTgts$, and Forge's
		// own default define for CopySpellAbility is the targeted spell. The
		// old unset default (the resolving spell itself) made such a spell
		// copy ITSELF — Flare's copy is again a Flare whose copy is again a
		// Flare — an unbounded self-copy loop no game could finish. With no
		// object target recorded (a fizzled ask), the copy does nothing.
		for _, t := range c.Targets {
			if !t.IsPlayer && t.Obj != 0 {
				spell = t.Obj
				break
			}
		}
		if spell == 0 {
			// Defined$ ValidStack <spec> (Ulalek, Fused Atrocity's "copy all
			// spells you control" and its SubAbility$'s "copy all other
			// activated and triggered abilities you control"): the one Defined
			// form this arm cannot read off c.Targets -- a ValidStack spec
			// resolves from the STACK, not from the trigger's targets. Route
			// it through the shared ValidStack resolver every other Defined
			// consumer uses, but ONLY when some comma token actually names a
			// stack kind (state.StackKindTokenOf): the parser's no-usable-token
			// degradation to Spell-only must not leak in here -- a genuinely
			// unknown token would otherwise widen to "all spells you control"
			// and copy every spell on the stack. Fail closed instead. The
			// `Ability` base IS a stack kind now (task abcopy3: Activated+
			// Triggered, never Spell), and its `otherAbility` qualifier
			// excludes the resolving wrapper via Ctx.ResolvingObj, so the
			// sub-copy cannot copy itself; the arm is PLURAL -- every admitted
			// object is copied, so Ulalek's "copy all spells you control" and
			// "copy all other activated and triggered abilities" place one copy
			// per match instead of only the first. The family exclusion is
			// untouched by this: validStackAdmits still drops the resolving
			// wrapper AND every instance or copy sharing (Source, Ability), the
			// recorded livelock guard -- an id-only exclusion would let a copy
			// of the wrapper be copied again and ask its pay question forever.
			spec := DefinedRefOf(sa).Text
			if stackSpec, ok := strings.CutPrefix(spec, "ValidStack"); ok {
				for tok := range strings.SplitSeq(strings.TrimSpace(stackSpec), ",") {
					if _, known := state.StackKindTokenOf(strings.TrimSpace(tok)); known {
						if ts, knownAll := knownDefinedTargets(h, c, spec); knownAll {
							for _, t := range ts {
								if t.IsPlayer || t.Obj == 0 {
									continue
								}
								if o := g.Obj(t.Obj); o == nil || o.Zone != state.ZStack {
									continue
								}
								validStackSpells = append(validStackSpells, t.Obj)
							}
						}
						break
					}
				}
			}
		}
		if spell == 0 && len(validStackSpells) == 0 {
			return
		}
	}
	if spell != 0 {
		// The source spell must actually be on the stack; a copy of something
		// that already left it is a no-op, the same totality stance as every
		// other effect primitive. The plural ValidStack arm above already
		// applied this filter per match.
		if o := g.Obj(spell); o == nil || o.Zone != state.ZStack {
			return
		}
		if !spellCopyAllowed(h, spell) {
			return
		}
	}
	if len(validStackSpells) > 0 {
		allowed := validStackSpells[:0]
		for _, id := range validStackSpells {
			if spellCopyAllowed(h, id) {
				allowed = append(allowed, id)
			}
		}
		validStackSpells = allowed
		if len(validStackSpells) == 0 {
			return
		}
	}

	// Controller$ (Chain Lightning's "If the player does, they may copy this
	// spell" -- CR 707.10's copy-ownership half): the copy belongs to the
	// player the parameter names, not the resolving controller. The target-
	// derived family resolves against the copied spell's own targets (the
	// shared c.Targets a Parent copy inherits); an unknown selector keeps
	// the historical resolving-controller default rather than inventing a
	// binding (measured corpus: only the TargetedOrController spelling
	// reaches a registered copy on a measured path). Resolved BEFORE the
	// Optional$ election below, because on the corpus's combined carriers
	// (Wandering Archaic's Controller$ You, Chain of Silence's Controller$
	// TargetedController) the may-copy election is the addressed player's,
	// not the resolving controller's.
	controller := c.Controller
	if spec := strings.TrimSpace(sa.ParamStr(cards.PKController)); spec != "" {
		if p, ok := copyControllerFor(g, c, spec); ok {
			controller = p
		}
	}

	// Optional$ True (Sevinne's Reclamation's "you may copy this spell", the
	// corpus's wider may-copy family, and the Optional$+UnlessCost$ carriers
	// Wandering Archaic and Chain of Silence): pose the may-copy election once
	// the copy is known to be possible (a resolvable, still-on-stack spell),
	// so a fizzled copy clause asks nothing. A decline ("no") returns without
	// emitting any StackCopy; an answered "yes" falls through to the ordinary
	// copy path.
	//
	// An SA that ALSO carries UnlessCost$ composes SEQUENTIALLY, not
	// exclusively: the shared unless gate (engine/effects/unless.go's unlessProceed,
	// called by Resolve before this dispatch) resolves its pay/decline first,
	// and its orientation decides only whether this BODY runs at all --
	// Wandering Archaic's unswitched "they may pay {2}. If they don't, you may
	// copy" runs the body on the DECLINE, Chain of Silence's UnlessSwitched$
	// "may sacrifice a land. If the player does, they may copy" runs it on the
	// PAY. By the time this dispatch runs the unless question is fully
	// answered -- so the may-copy election is the SECOND ask in Forge's own sequence: posing it
	// duplicates nothing and inverts nothing (the round-1 read that scoped
	// this arm to UnlessCost$-free SAs was wrong, findings-r2).
	if strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKOptional)), "True") {
		d := &decision.Decision{Player: controller, Kind: decision.KChoose, Min: 1, Max: 1,
			Source: c.Source, ResumeKind: "copy_optional", ResumeSA: sa,
			CopyOfCopy: copyOfCopy(g, spell),
			Prompt:     "Copy it?",
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Label: "Yes — copy", Player: controller},
				{Index: 1, Kind: "no", Label: "No", Player: controller},
			}}
		// Answered in place: copy on a yes, make none otherwise. With no
		// answer (an effects-package double, a fuzz run) the deterministic
		// pre-ask stand-in the doc comment above records holds: the copy is
		// made.
		if ans, ok := AskTape(h, d); ok && (len(ans) == 0 || ans[0].Kind != "yes") {
			return
		}
	}
	// IgnoreFreeze$ True (Ulalek, Fused Atrocity's copy trigger; Forge's
	// generic copy template carries it too): Forge MagicStack's frozen flag
	// blocks adding to the stack while it holds, and a copy SA carrying this
	// key is exempt. This engine has no stack freeze -- nothing in the
	// implemented ruleset suspends stack additions -- so there is no gate to
	// relax; the recognition read documents the parameter so the census never
	// flags it unread (review sol2: the earlier empty if-block was dropped).
	_ = sa.ParamStr(cards.PKIgnoreFreeze)
	mayChoose := strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKMayChooseTarget)), "True")
	// NonLegendary$ True (The Sixth Doctor's "copy it, except the copy isn't
	// legendary", and the corpus's six-carrier CopySpellAbility family:
	// Osgood Operation Double, Iron Man Bleeding Edge, Jackal Genius
	// Geneticist, Storm of Saruman, The Clone Saga). The copy's
	// characteristics drop the Legendary supertype, so a copied legendary
	// PERMANENT spell resolves into a token CR 704.5j must not gather against
	// its original. The rider rides the StackCopy event's Counter -- the
	// field StackCopy otherwise leaves empty -- and events.Apply's fold
	// turns it into the object marker rules' typeCharacteristics strips the
	// supertype from, the same event-sourced shape ClonePermanent's Counter
	// riders take. No event field is added or reordered; an absent or False
	// key adds no strip, but a copy of a stripped copy inherits the strip.
	nonLegendary := strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKNonLegendary)), "True")
	// SetLoyalty$ (the corpus's one carrier is Ob Nixilis, the Adversary's
	// Casualty:X script rider): the copy's starting loyalty, delivered
	// RESOLVED -- the creating trigger's payload already substituted the
	// casualty amount for the script's `Casualty` value, so the value here
	// is a number. The rider composes into the StackCopy event's Counter
	// payload (events.StackCopyCounter), which the entry fold
	// (events.EntryCounterGrants) places instead of the printed face value.
	// An absent key leaves the printed starting loyalty; an unparseable
	// value cannot arise from the payload path and is ignored.
	var riders events.StackCopyCounter
	riders.NonLegendary = nonLegendary
	if raw := strings.TrimSpace(sa.ParamStr(cards.PKSetLoyalty)); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			riders.Loyalty, riders.HasLoyalty = int32(n), true
		}
	}
	payload := events.StackCopyCounterString(riders)
	// RememberCopies$ True (Shiko and Narset, Unified's "copy that spell ...
	// If you don't copy a spell this way, draw a card"; Chef's Kiss's "the
	// spell and the copy"; Tempt with Mayhem's per-copier count): Forge's
	// CopySpellAbilityEffect.resolve ends with card.addRemembered(copies) for
	// EVERY copy it actually made. Read once here and threaded into both
	// emit sites below; an absent/False key leaves the remembered set
	// untouched, exactly the historical behaviour.
	//
	// RememberNewCard$ True (Spinerock Tyrant, Mizzix's Replica Rider,
	// Choreographed Sparks, Gandalf, Nalfeshnee, Spider-Verse: the follow-up
	// Pump/Animate reads Defined$ Remembered) is Forge's same rider spelled
	// for the "new card" the copy is; it records the copy identically.
	rememberCopies := strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKRememberCopies)), "True") ||
		strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKRememberNewCard)), "True")
	// DefinedTarget$ (Feather, Radiant Arbiter's DefinedTarget$ ChosenCard,
	// Ivy, Gleeful Spellthief's DefinedTarget$ Self): Forge's
	// CopySpellAbilityEffect makes ONE copy per defined target, each with
	// its target REPLACED by that entry (the StackCopy event's IDs override
	// the inherited list, engine/events/event.go) -- "for each of those creatures,
	// copy that spell. The copy targets that creature." The copy count is
	// the defined set's size, never Amount$: no corpus carrier combines the
	// param with an Amount$ (they name none), and the param is exactly the
	// override of the copy-keeps-targets stand-in below. An unresolvable
	// value (Zevlor, Elturel Exile's OppNonTriggeredSpellAbilityTargetsOr-
	// Controller) keeps the historical single inherited-target copy under
	// one loud Note -- the override applies only where the engine can
	// resolve what it names.
	targets, defined := copyDefinedTargets(h, c, sa)
	if spec := strings.TrimSpace(sa.Params["CopyForEachCanTarget"]); spec != "" && spell != 0 && !defined {
		if copyForEachCanTarget(h, c, sa, spell, controller, spec, rememberCopies, payload, addTypes) {
			return
		}
	}
	switch {
	case defined && len(targets) > 0:
		// One copy per defined target, each carrying that entry alone in its
		// IDs. Amount$ is deliberately not consulted on this route (no corpus
		// carrier combines the two; Forge's definedTarget branch ignores it
		// too).
		for _, t := range targets {
			ev := events.Event{Kind: events.StackCopy, Obj: spell, Player: controller, IDs: []state.ObjID{t.Obj}, Counter: payload}
			emitCopy(h, c, rememberCopies, ev, addTypes)
		}
	case defined:
		// A resolvable param whose set is empty (nothing chosen): no copy,
		// no Note -- the resolution's sub-abilities (the cleanup chain) still
		// run.
	default:
		if spec := DefinedOf(sa).Target.Text; spec != "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "copy: DefinedTarget$ " + spec + " not resolved; copy keeps its targets"})
		}
		n := int(Num(h, c, sa, "Amount", 1))
		copies := validStackSpells
		if len(copies) == 0 {
			copies = []state.ObjID{spell}
		}
		for _, sp := range copies {
			for i := 0; i < n; i++ {
				ev := events.Event{Kind: events.StackCopy, Obj: sp, Player: controller, Counter: payload}
				if mayChoose {
					// CR 707.10c: the copy's controller may choose new targets. The
					// permission rides the StackCopy event (Amount 1), so it is
					// recorded per copy instance and replayed; engine/rules/stack.go's
					// resolveTop poses the election and the TargetsChosen fold
					// consumes it. No Note: the election is now real, not a
					// stand-in.
					ev.Amount = 1
				}
				emitCopy(h, c, rememberCopies, ev, addTypes)
			}
		}
	}
}

// spellCopyAllowed asks the rules host whether the candidate spell's own
// static permits copying. Effects test doubles without this optional rules
// seam retain their established behavior.
func spellCopyAllowed(h Host, id state.ObjID) bool {
	checker, ok := h.(interface{ SpellCopyAllowed(state.ObjID) bool })
	return !ok || checker.SpellCopyAllowed(id)
}

// emitCopy emits one StackCopy event. When remember or addTypes is set it
// routes through the mint-returning Host.EmitStackCopy: remember appends every
// minted copy object to the resolution's remembered set -- both halves,
// exactly as engine/effects/token.go's RememberTokens$ rider does for a token
// mint: the chain local Ctx.Remembered (read by Defined$ Remembered and by the
// ConditionDefined$ Remembered gate) and the source object's persistent
// event-backed list via eventRemember (Forge's card.addRemembered). A copy that
// the fold did not actually mint (an early break -- the source left the stack)
// returns no id and is not remembered, mirroring Forge's own early returns. An
// absent or False RememberCopies$ leaves remember false.
//
// addTypes is CopySpellAbility's AddTypes$ list. Every minted copy gets one
// CopyExcept carrying the copied face's type line plus those words (CR
// 707.9a/b), the same event Clone/CopyPermanent use for their copy exceptions,
// so every type reader sees the addition without a layer-4 special case. The
// payload is settled against the COPIED SPELL (ev.Obj), not the mint: when the
// source is itself a copy its Face() is already the earlier CopyExcept basis,
// so a copy of a copy inherits AddTypes$ (CR 707.2). An absent key leaves the
// historical plain Emit.
func emitCopy(h Host, c *Ctx, remember bool, ev events.Event, addTypes []string) {
	if !remember && len(addTypes) == 0 {
		h.Emit(ev)
		return
	}
	for _, id := range h.EmitStackCopy(ev) {
		if remember {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
			eventRemember(h, c, id)
		}
		if len(addTypes) > 0 {
			if x := copyExceptions(h.Game().Obj(ev.Obj), copyExceptSpec{addTypes: addTypes}); !x.Empty() {
				h.Emit(events.Event{Kind: events.CopyExcept, Obj: id, Text: x.Text()})
			}
		}
	}
}

// copyForEachCanTarget is CR 707.10d's "copy that spell for each other object
// it could target" (CopyForEachCanTarget$ <spec>): one copy per legal target
// of the spell's targeting clause that the spec admits (the copier is "You")
// and the spell does not already target, each copy's first target replaced by
// its candidate through the StackCopy IDs override, in ObjID order.
// ChooseOnlyOne$ True (Beamsplitter Mage) asks the copier for one candidate
// and makes one copy. A player candidate cannot ride StackCopy's object-only
// IDs, so it yields no copy under one loud Note. It reports false when the
// host cannot enumerate candidates or the spell has a target this override
// cannot carry, leaving the historical single inherited-target copy.
func copyForEachCanTarget(h Host, c *Ctx, sa *cards.SA, spell state.ObjID, controller state.PlayerID, spec string, remember bool, payload string, addTypes []string) bool {
	ch, ok := h.(copyTargetCandidatesHost)
	if !ok {
		return false
	}
	g := h.Game()
	so := g.Obj(spell)
	if so == nil || len(so.Targets) == 0 {
		return false
	}
	for _, t := range so.Targets {
		if t.IsPlayer {
			return false
		}
	}
	var cands []state.ObjID
	playerCand := false
	for _, t := range ch.CopyTargetCandidates(controller, spell) {
		if t.IsPlayer {
			if MatchesPlayerSpec(g, spec, t.Player, controller) {
				playerCand = true
			}
			continue
		}
		if t.Obj == so.Targets[0].Obj || !MatchesSpecCtx(g, spec, t.Obj, c.SpecContext(controller)) {
			continue
		}
		cands = append(cands, t.Obj)
	}
	slices.Sort(cands)
	if playerCand {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "copy: CopyForEachCanTarget$ player candidates not copied (a copy cannot override a target with a player)"})
	}
	if len(cands) > 0 && strings.EqualFold(strings.TrimSpace(sa.Params["ChooseOnlyOne"]), "True") {
		d := &decision.Decision{Player: controller, Kind: decision.KChoose, Min: 1, Max: 1,
			Source: c.Source, ResumeKind: "copy_for_each_can_target", ResumeSA: sa,
			Prompt: "Choose a target for the copy"}
		for _, id := range cands {
			name := "a permanent"
			if o := g.Obj(id); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "copy_target", Label: name, Obj: id, Player: controller})
		}
		// No answer (an effects-package double): the first candidate, the
		// pick the bot's clamp fallback answers.
		cands = cands[:1]
		if ans, ok := AskTape(h, d); ok && len(ans) > 0 {
			cands = []state.ObjID{ans[0].Obj}
		}
	}
	for _, cand := range cands {
		ids := []state.ObjID{cand}
		for _, t := range so.Targets[1:] {
			ids = append(ids, t.Obj)
		}
		emitCopy(h, c, remember, events.Event{Kind: events.StackCopy, Obj: spell, Player: controller, IDs: ids, Counter: payload}, addTypes)
	}
	return true
}

// copyDefinedTargets resolves a CopySpellAbility DefinedTarget$ parameter to
// the per-copy target list. true (defined) means the param is present and
// every value this build recognises resolved: the copies follow the defined
// entries one-for-one and the caller never falls back to the inherited
// targets. false covers both the absent param and an unresolvable value, so
// the caller can keep the historical copy-keeps-targets shape (under the
// loud Note) instead of silently dropping the param.
func copyDefinedTargets(h Host, c *Ctx, sa *cards.SA) ([]state.Target, bool) {
	spec := DefinedOf(sa).Target.Text
	if spec == "" {
		return nil, false
	}
	g := h.Game()
	switch copyDefinedTargetsCodes.Code(string(spec)) {
	case copyDefinedTargetsChosenCard:
		// The ChooseCard chain's answered set (resolutionChosenCards is the
		// shared read Count$ChosenSize and Defined$ ChosenCard take, so the
		// copies can never disagree with either). Only object entries copy:
		// a ChooseCard's chosen PLAYERS are not "those creatures".
		var out []state.Target
		for _, t := range resolutionChosenCards(g, c) {
			if !t.IsPlayer && t.Obj != 0 {
				out = append(out, t)
			}
		}
		return out, true
	case copyDefinedTargetsSelf:
		// Ivy, Gleeful Spellthief: the copy targets the trigger's source
		// permanent ("The copy targets NICKNAME"). An absent source fails
		// the set empty -- defined, but nothing to target.
		if c.Source == 0 || g.Obj(c.Source) == nil {
			return nil, true
		}
		return []state.Target{{Obj: c.Source}}, true
	}
	return nil, false
}

// copyControllerFor resolves a CopySpellAbility Controller$ selector to the
// player who receives the copy. Only the target-derived family is modelled:
// TargetedOrController is the Forge spelling for "the target if it is a
// player, else the target's controller" (Chain Lightning's cycle), and the
// plain Targeted/TargetedController/TargetedPlayer forms read the same
// binding. A missing target (a fizzled ask) fails to ok=false and the caller
// keeps its controller.
// copyOfCopy reports whether the spell a may-copy election would copy is
// itself a stack copy (decision.Decision.CopyOfCopy: the chain-continuation
// marker the unattended bot declines).
func copyOfCopy(g *state.Game, spell state.ObjID) bool {
	o := g.Obj(spell)
	return o != nil && o.IsCopy
}

func copyControllerFor(g *state.Game, c *Ctx, spec string) (state.PlayerID, bool) {
	switch copyControllerForCodes.Code(string(spec)) {
	case copyControllerForTargeted:
		for _, t := range c.Targets {
			if t.IsPlayer {
				return t.Player, true
			}
			if o := g.Obj(t.Obj); o != nil {
				return o.Controller, true
			}
		}
		return 0, false
	case copyControllerForChosenPlayer:
		for _, t := range c.Chosen {
			if t.IsPlayer {
				return t.Player, true
			}
		}
		return 0, false
	case copyControllerForRemembered:
		// Tempt with Mayhem's per-opponent copy: the RepeatEach loop binds its
		// current subject (a player) as Remembered, and prior iterations' copy
		// objects are remembered too. A remembered PLAYER named directly wins
		// over an object's controller -- the same precedence Defined$'s
		// single-player read takes -- so the copy belongs to the current
		// subject, else to the first remembered object's controller. An absent
		// binding returns false and the caller keeps the resolving controller.
		for _, t := range c.Remembered {
			if t.IsPlayer {
				return t.Player, true
			}
		}
		for _, t := range c.Remembered {
			if o := g.Obj(t.Obj); o != nil {
				return o.Controller, true
			}
		}
		return 0, false
	case copyControllerForYou:
		return c.Controller, true
	case copyControllerForNextOpponentToYourLeft:
		// Barroom Brawl's "Then that player [the opponent to your left] may
		// copy this spell": the next living seat after the resolving
		// controller in turn order (this build has no teams). Before this arm
		// the unknown selector fell back to the resolving controller, so the
		// CASTER was offered its own copy, and the copy's copy, forever
		// (cardfuzz batch1 line 1).
		alive := g.AliveFrom(c.Controller)
		if len(alive) < 2 {
			return 0, false
		}
		return alive[1], true
	}
	return 0, false
}

type effCopySpellAbilityCode uint16

const (
	effCopySpellAbilityTriggeredSpellAbility effCopySpellAbilityCode = iota + 1
	effCopySpellAbilityParent
)

var effCopySpellAbilityCodes = state.NewStrCodes(
	state.StrEntry[effCopySpellAbilityCode]{Key: "TriggeredSpellAbility", Val: effCopySpellAbilityTriggeredSpellAbility},
	state.StrEntry[effCopySpellAbilityCode]{Key: "Parent", Val: effCopySpellAbilityParent},
)

type copyDefinedTargetsCode uint16

const (
	copyDefinedTargetsChosenCard copyDefinedTargetsCode = iota + 1
	copyDefinedTargetsSelf
)

var copyDefinedTargetsCodes = state.NewStrCodes(
	state.StrEntry[copyDefinedTargetsCode]{Key: "ChosenCard", Val: copyDefinedTargetsChosenCard},
	state.StrEntry[copyDefinedTargetsCode]{Key: "Self", Val: copyDefinedTargetsSelf},
)

type copyControllerForCode uint16

const (
	copyControllerForTargeted copyControllerForCode = iota + 1
	copyControllerForChosenPlayer
	copyControllerForRemembered
	copyControllerForYou
	copyControllerForNextOpponentToYourLeft
)

var copyControllerForCodes = state.NewStrCodes(
	state.StrEntry[copyControllerForCode]{Key: "TargetedOrController", Val: copyControllerForTargeted},
	state.StrEntry[copyControllerForCode]{Key: "Targeted", Val: copyControllerForTargeted},
	state.StrEntry[copyControllerForCode]{Key: "TargetedController", Val: copyControllerForTargeted},
	state.StrEntry[copyControllerForCode]{Key: "TargetedPlayer", Val: copyControllerForTargeted},
	state.StrEntry[copyControllerForCode]{Key: "ThisTargetedController", Val: copyControllerForTargeted},
	state.StrEntry[copyControllerForCode]{Key: "ThisTargetedPlayer", Val: copyControllerForTargeted},
	state.StrEntry[copyControllerForCode]{Key: "ChosenPlayer", Val: copyControllerForChosenPlayer},
	state.StrEntry[copyControllerForCode]{Key: "Player.Chosen", Val: copyControllerForChosenPlayer},
	state.StrEntry[copyControllerForCode]{Key: "Remembered", Val: copyControllerForRemembered},
	state.StrEntry[copyControllerForCode]{Key: "RememberedController", Val: copyControllerForRemembered},
	state.StrEntry[copyControllerForCode]{Key: "You", Val: copyControllerForYou},
	state.StrEntry[copyControllerForCode]{Key: "NextOpponentToYourLeft", Val: copyControllerForNextOpponentToYourLeft},
	state.StrEntry[copyControllerForCode]{Key: "NextPlayerToYourLeft", Val: copyControllerForNextOpponentToYourLeft},
)
