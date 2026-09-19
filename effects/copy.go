package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
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
// each copy keeping its targets. MayChooseTarget$ True is a player's
// mid-resolution choice this build still cannot ask (stay-down in the M2r
// approximations list — switching the copy's targets is a later task), so
// the copies keep their targets and each records a Note saying so.
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
			// spells you control"): the one Defined form this arm cannot read
			// off c.Targets -- a ValidStack spec resolves from the STACK, not
			// from the trigger's targets. Route it through the shared ValidStack
			// resolver every other Defined consumer uses, but ONLY when some
			// comma token actually names a stack kind (state.StackKindTokenOf):
			// the parser's no-usable-token degradation to Spell-only must not
			// leak in here -- an unparsed "Ability" token (Ulalek's sub-copy's
			// `Ability.YouCtrl+otherAbility`) would otherwise widen to "all
			// spells you control" and copy every spell on the stack. Fail
			// closed instead; that sub-copy half stays the recorded stand-in.
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
	for n := Num(h, c, sa, "Amount", 1); n > 0; n-- {
		h.Emit(events.Event{Kind: events.StackCopy, Obj: spell, Player: controller})
		if mayChoose {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "copy keeps its targets"})
		}
	}
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
