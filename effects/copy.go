package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
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
//     the spell whose cast FIRED the trigger — Storm (cards/keywords.go
//     expands kw:Storm into exactly this).
//
// Amount$ copies are placed on the stack by StackCopy events (default 1),
// each copy keeping its targets unless the creating SA declares
// MayChooseTarget$ True (CR 707.10c: that copy's controller may choose new
// targets). The permission rides the StackCopy event's Amount discriminator,
// which rules/stack.go's resolveTop reads as a one-shot target election over
// the copy's own target requirement — so an external copier (Mirari, Cloven
// Casting, a Storm or Replicate copy) grants it too. Copies keep their
// targets only when the parameter is absent or False.
//
// UnlessCost$ (Chain Lightning, String of Disappearances) rides the ONE
// shared unless gate (effects.Resolve's unlessProceed dispatch, shared by
// every API): the payer is UnlessPayer$'s resolved target (default the
// target's controller), the pay/decline labels live in poseUnlessAsk's
// CopySpellAbility arm, and the copy loop below runs, or not, exactly once
// per the gate's orientation (rules' unless_pay resume arm charged the cost
// on an affordable "pay"). A host that cannot ask (an effects-package test
// double) keeps the deterministic decline.
func effCopySpellAbility(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	// Optional$ True (Sevinne's Reclamation's "if this spell was cast from a
	// graveyard, you may copy this spell", and the corpus's wider may-copy
	// family) makes the copy itself a may effect: the resolving player is
	// asked a yes/no before any copy is made, and a decline makes none. The
	// answered election rides Ctx.CopyOpt (the same runtime-continuation
	// class as AttachOpt), consumed and cleared here so a nested
	// CopySpellAbility poses its own ask (fx42 scoping). An absent key leaves
	// the historical unconditional copy. A host that cannot ask (an
	// effects-package double, fuzz) keeps the deterministic pre-ask
	// behaviour -- the copy is made -- the same R-9 fallback the other
	// yes/no asks carry (the ask's bracket is Min == Max == 1, so Clamp and
	// the bot answer option 0 = yes).
	copyOpt := c.CopyOpt
	c.CopyOpt = ""
	// Resolve which spell to copy. For a trigger the remembered entry is the
	// cast spell (the first object entry); for a direct Parent copy it is the
	// currently resolving spell itself.
	var spell state.ObjID
	switch strings.TrimSpace(sa.Params["Defined"]) {
	case "TriggeredSpellAbility":
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
	case "Parent":
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
			// sub-copy cannot copy itself; the arm stays SINGLE-TARGET --
			// with several matches it copies only the first (stack-arena
			// order), the recorded plural-copy stand-in.
			spec := strings.TrimSpace(sa.Params["Defined"])
			if stackSpec, ok := strings.CutPrefix(spec, "ValidStack"); ok {
				for _, tok := range strings.Split(strings.TrimSpace(stackSpec), ",") {
					if _, known := state.StackKindTokenOf(strings.TrimSpace(tok)); known {
						if ts, knownAll := knownDefinedTargets(h, c, spec); knownAll {
							for _, t := range ts {
								if !t.IsPlayer && t.Obj != 0 {
									spell = t.Obj
									break
								}
							}
						}
						break
					}
				}
			}
		}
		if spell == 0 {
			return
		}
	}
	if spell == 0 {
		return
	}
	// The source spell must actually be on the stack; a copy of something
	// that already left it is a no-op, the same totality stance as every
	// other effect primitive.
	if o := g.Obj(spell); o == nil || o.Zone != state.ZStack {
		return
	}

	// Optional$ True: pose the may-copy election once the copy is known to be
	// possible (a resolvable, still-on-stack spell), so a fizzled copy clause
	// asks nothing. A decline ("no") returns without emitting any StackCopy;
	// an answered "yes" falls through to the ordinary copy path. A first
	// pass (copyOpt == "") with a host that cannot ask falls through to the
	// unconditional copy, the R-9 stand-in documented above.
	//
	// An SA that ALSO carries UnlessCost$ (Wandering Archaic, the Chain of
	// Silence/Acid/Smog cycle) already has its copy election: the shared
	// unless gate poses that pay/decline and its orientation decides the body
	// (effects/unless.go's unlessProceed, called by Resolve before this
	// dispatch). Posing a second yes/no on top would put a mandatory-looking
	// ask before the pay offer and invert or duplicate the unless decision,
	// so the optional read is scoped to SAs with no UnlessCost$ -- the pure
	// may-copy family (Sevinne's Reclamation, Parnesse, Curse of Echoes, the
	// rest named in the ticket's census).
	if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") &&
		strings.TrimSpace(sa.Params["UnlessCost"]) == "" {
		switch copyOpt {
		case "yes":
			// Answered yes: fall through to the copy below.
		case "no":
			return
		default:
			d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
				Source: c.Source, ResumeKind: "copy_optional", ResumeSA: sa,
				ResumeRemembered: copyTargets(c.Remembered),
				Prompt:           "Copy it?",
				Options: []decision.Option{
					{Index: 0, Kind: "yes", Label: "Yes — copy", Player: c.Controller},
					{Index: 1, Kind: "no", Label: "No", Player: c.Controller},
				}}
			_ = Ask(h, d)
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
	// reaches a registered copy on a measured path).
	controller := c.Controller
	if spec := strings.TrimSpace(sa.Params["Controller"]); spec != "" {
		if p, ok := copyControllerFor(g, c, spec); ok {
			controller = p
		}
	}
	// IgnoreFreeze$ True (Ulalek, Fused Atrocity's copy trigger; Forge's
	// generic copy template carries it too): Forge MagicStack's frozen flag
	// blocks adding to the stack while it holds, and a copy SA carrying this
	// key is exempt. This engine has no stack freeze -- nothing in the
	// implemented ruleset suspends stack additions -- so there is no gate to
	// relax; the recognition read documents the parameter so the census never
	// flags it unread (review sol2: the earlier empty if-block was dropped).
	_ = sa.Params["IgnoreFreeze"]
	mayChoose := strings.EqualFold(strings.TrimSpace(sa.Params["MayChooseTarget"]), "True")
	// DefinedTarget$ (Feather, Radiant Arbiter's DefinedTarget$ ChosenCard,
	// Ivy, Gleeful Spellthief's DefinedTarget$ Self): Forge's
	// CopySpellAbilityEffect makes ONE copy per defined target, each with
	// its target REPLACED by that entry (the StackCopy event's IDs override
	// the inherited list, events/event.go) -- "for each of those creatures,
	// copy that spell. The copy targets that creature." The copy count is
	// the defined set's size, never Amount$: no corpus carrier combines the
	// param with an Amount$ (they name none), and the param is exactly the
	// override of the copy-keeps-targets stand-in below. An unresolvable
	// value (Zevlor, Elturel Exile's OppNonTriggeredSpellAbilityTargetsOr-
	// Controller) keeps the historical single inherited-target copy under
	// one loud Note -- the override applies only where the engine can
	// resolve what it names.
	targets, defined := copyDefinedTargets(h, c, sa)
	switch {
	case defined && len(targets) > 0:
		// One copy per defined target, each carrying that entry alone in its
		// IDs. Amount$ is deliberately not consulted on this route (no corpus
		// carrier combines the two; Forge's definedTarget branch ignores it
		// too).
		for _, t := range targets {
			h.Emit(events.Event{Kind: events.StackCopy, Obj: spell, Player: controller, IDs: []state.ObjID{t.Obj}})
		}
	case defined:
		// A resolvable param whose set is empty (nothing chosen): no copy,
		// no Note -- the resolution's sub-abilities (the cleanup chain) still
		// run.
	default:
		if spec := strings.TrimSpace(sa.Params["DefinedTarget"]); spec != "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "copy: DefinedTarget$ " + spec + " not resolved; copy keeps its targets"})
		}
		n := int(Num(h, c, sa, "Amount", 1))
		for i := 0; i < n; i++ {
			ev := events.Event{Kind: events.StackCopy, Obj: spell, Player: controller}
			if mayChoose {
				// CR 707.10c: the copy's controller may choose new targets. The
				// permission rides the StackCopy event (Amount 1), so it is
				// recorded per copy instance and replayed; rules/stack.go's
				// resolveTop poses the election and the TargetsChosen fold
				// consumes it. No Note: the election is now real, not a
				// stand-in.
				ev.Amount = 1
			}
			h.Emit(ev)
		}
	}
}

// copyDefinedTargets resolves a CopySpellAbility DefinedTarget$ parameter to
// the per-copy target list. true (defined) means the param is present and
// every value this build recognises resolved: the copies follow the defined
// entries one-for-one and the caller never falls back to the inherited
// targets. false covers both the absent param and an unresolvable value, so
// the caller can keep the historical copy-keeps-targets shape (under the
// loud Note) instead of silently dropping the param.
func copyDefinedTargets(h Host, c *Ctx, sa *cards.SA) ([]state.Target, bool) {
	spec := strings.TrimSpace(sa.Params["DefinedTarget"])
	if spec == "" {
		return nil, false
	}
	g := h.Game()
	switch spec {
	case "ChosenCard":
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
	case "Self":
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
func copyControllerFor(g *state.Game, c *Ctx, spec string) (state.PlayerID, bool) {
	switch spec {
	case "TargetedOrController", "Targeted", "TargetedController", "TargetedPlayer",
		"ThisTargetedController", "ThisTargetedPlayer":
		for _, t := range c.Targets {
			if t.IsPlayer {
				return t.Player, true
			}
			if o := g.Obj(t.Obj); o != nil {
				return o.Controller, true
			}
		}
		return 0, false
	case "ChosenPlayer", "Player.Chosen":
		for _, t := range c.Chosen {
			if t.IsPlayer {
				return t.Player, true
			}
		}
		return 0, false
	case "You":
		return c.Controller, true
	}
	return 0, false
}
