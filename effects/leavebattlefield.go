// The leave-the-battlefield rider family: Forge's `LeaveBattlefield$ <value>`
// parameter on a resolving Animate body ("If it would leave the battlefield,
// exile it instead of putting it anywhere else" -- Whip of Erebos, Kheru Lich
// Lord, Gruesome Encore, Storm Herald, the four `DB$ Animate` corpus carriers
// measured 2026-09-22), and the sibling `sVars$` grant on an Animate body
// (the named SVars the animated object carries).
//
// The rider's scope is the Animate site only. The remaining four corpus
// carriers -- two `DB$ Pump` (Moira and Teshar, Dreams of the Dead) and two
// ChangeZone (Isareth the Awakener, From the Catacombs) -- are split out to a
// follow-up ticket: the brief authorizes the Animate/Whip shape, and neither
// the brief nor a controller ruling covers the class-wide expansion. The
// `LeaveBattlefield$` value is therefore only ever read from an Animate SA
// here.
package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// registerLeaveExile is LeaveBattlefield$ Exile's registration for ONE
// object: an Effect-created Moved replacement in the effects/damage.go
// ReplaceDyingDefined$ shape -- Origin$ Battlefield + ValidCard$
// Card.IsRemembered scope the match to id's own battlefield departure, the
// body rewrites it as a battlefield->exile move (Defined$ ReplacedCard
// binds the departing object), and the ExileOnMoved$/Remembered pair is
// registerAnimateEffects' own lifetime idiom: the sweep ends the promise
// exactly when that departure is applied, so a card that returns to the
// battlefield later is a plain permanent again, not a re-armed promise (the
// same convention the animation grant's other halves keep).
//
// Lifetime is the granting body's own: an UntilEOT grant ends at cleanup,
// a Duration$ Permanent grant lasts indefinitely -- in both cases the move
// sweep above is what actually ends the promise on the departure itself.
// value "" (no rider) is the no-op; "Exile" is the implemented spelling;
// any other value emits ONE loud Note and registers nothing (the
// loud-unread convention -- the corpus carries no such value).
func registerLeaveExile(h Host, c *Ctx, id state.ObjID, value, dur string, permanent bool) {
	value = strings.TrimSpace(value)
	switch value {
	case "":
		return
	case "Exile":
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "LeaveBattlefield$ " + value + " is not implemented; ignored"})
		return
	}
	if h.Game().Obj(id) == nil {
		return
	}
	remembered, exileOn := leaveExileLifetime(id, value)
	h.AddContinuous(state.ContinuousEffect{
		Source: id, Affects: "Card.Self", Controller: c.Controller,
		Duration: dur, Permanent: permanent, UntilEOT: !permanent,
		Remembered:       remembered,
		ExileOnMoved:     exileOn,
		ReplacementEvent: "Moved",
		ReplacementParams: map[string]string{
			"Origin":    "Battlefield",
			"ValidCard": "Card.IsRemembered",
		},
		ReplacementBody: "DB$ ChangeZone | Defined$ ReplacedCard | Origin$ Battlefield | Destination$ Exile",
	})
}

// registerAnimateStaticAbilities registers staticAbilities$ bodies on the
// animated object. The Animate parameter is a list of SVar names, not a
// Mode$ Continuous body: it is the same static table used by Effect's
// StaticAbilities$ path. The restriction registration is deliberately
// source-scoped to the affected object, so Card.Self (the common Forge
// spelling) cannot accidentally apply to every matching permanent.
func registerAnimateStaticAbilities(h Host, c *Ctx, id state.ObjID, names []string, dur string, permanent bool, exileOn string, remembered []state.ObjID) {
	for _, name := range names {
		mode, params := parseStaticLine(c.SVars, name)
		if mode == "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "Animate staticAbilities$ " + name + " has no static body; ignored"})
			continue
		}
		if mode != "CantSacrifice" && mode != "CantBlockUnless" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "Animate staticAbilities$ " + name + " mode " + mode + " is not implemented; ignored"})
			continue
		}
		if mode == "CantSacrifice" && !CantSacrificeRestrictionParamsReadable(params) {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "Animate staticAbilities$ " + name + " mode " + mode + " is not implemented; ignored"})
			continue
		}
		if mode == "CantBlockUnless" && !CantBlockUnlessRestrictionParamsReadable(params) {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "Animate staticAbilities$ " + name + " mode " + mode + " is not implemented; ignored"})
			continue
		}
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Controller: c.Controller,
			// Next-turn durations are resolved by AddContinuous's UntilTurn
			// boundary; marking them UntilEOT would discard the restriction at
			// the current cleanup before that boundary is reached.
			UntilEOT: !permanent && !IsNextTurnDuration(dur), Duration: dur, Permanent: permanent,
			ExileOnMoved: exileOn, Remembered: remembered,
			Restriction: mode, RestrictParams: params,
			// The granting face's SVar table: a delivered CantBlockUnless body's
			// Cost$ may name an SVar on it (Whipgrass Entangler's
			// WhipgrassClericNum), which rules' block-prop consultation resolves
			// exactly as a printed static's face table is resolved.
			RestrictSVars: c.SVars,
		})
	}
}

// registerSVarGrants registers a body's sVars$ grant (the named SVars the
// animated object carries for the animation's own lifetime): one
// ContinuousEffect with AddSVars, read back through Engine.GrantedSVar /
// grantedSVarsFor (rules/layers.go's AddSVar$ machinery, which pileSVars
// layers under the printed table). It is reached only from the Animate site
// (registerAnimateEffects). A body whose own value is a nested
// "SVar:<Name>:<Value>" declaration (Whip of Erebos's
// SVar:WhipMustAttack:SVar:MustAttack:True) is granted under BOTH names --
// the verbatim Forge grant and the expanded marker the nested form
// declares -- one level per body, the corpus's own depth; no corpus body
// nests deeper. The granted marker bodies today (MustAttack, MustBeBlocked,
// EndOfTurnLeavePlay, HasAttackEffect) are Forge AI-preference flags with no
// rules reader in this build -- Whip's oracle text carries no attack clause,
// so the grant must NOT harden into a rules-level must-attack static -- but
// the grant itself is real and event-backed, so any future reader resolves
// it. A named SVar missing from the granting face's table emits one loud
// Note and grants nothing.
func registerSVarGrants(h Host, c *Ctx, id state.ObjID, names []string, leaveValue, dur string, permanent bool) {
	if len(names) == 0 {
		return
	}
	added := map[string]string{}
	for _, n := range names {
		body, ok := c.SVars[n]
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "sVars$ " + n + " has no SVar body on the granting face; ignored"})
			continue
		}
		added[n] = body
		if name, value, ok := parseNestedSVar(body); ok {
			added[name] = value
		}
	}
	if len(added) == 0 {
		return
	}
	if h.Game().Obj(id) == nil {
		return
	}
	// The move-driven lifetime (registerAnimateEffects' idiom), applied only
	// when the granting body also declared `LeaveBattlefield$ Exile`: the
	// grant ends when the animated object leaves the battlefield, so the
	// granted marker is not carried by a plain permanent again. A body with
	// no leave clause keeps the historic lifetime (the documented Permanent
	// asymmetry), so this read changes no existing carrier's behaviour.
	remembered, exileOn := leaveExileLifetime(id, leaveValue)
	h.AddContinuous(state.ContinuousEffect{
		Source: id, Affects: "Card.Self", Controller: c.Controller,
		Duration: dur, Permanent: permanent, UntilEOT: !permanent,
		Remembered:   remembered,
		ExileOnMoved: exileOn,
		AddSVars:     added,
	})
}

// leaveExileLifetime returns the move-driven lifetime an object's grants must
// carry when its granting body declared `LeaveBattlefield$ Exile`: Remembered
// holds the object itself and ExileOnMoved names the battlefield, so
// rules/layers.go's effectMoveSweep ends EVERY half of the grant the instant
// that object leaves the battlefield (its departure is exactly what the
// promise substitutes). It is the ONE place the pair is built, so no caller
// can register a grant that outlives the departure and re-arms on a later
// re-entry (CR 400.7): a card that returns to the battlefield is a plain
// permanent again. Both results are nil/"" for every other body.
func leaveExileLifetime(id state.ObjID, value string) (remembered []state.ObjID, exileOn string) {
	if !strings.EqualFold(strings.TrimSpace(value), "Exile") {
		return nil, ""
	}
	return []state.ObjID{id}, "Battlefield"
}

// parseNestedSVar parses the "SVar:<Name>:<Value>" body an sVars$ grant's
// own value carries (the nested declaration Whip of Erebos's
// SVar:WhipMustAttack:SVar:MustAttack:True marks). ok is false for any other
// shape.
func parseNestedSVar(raw string) (name, value string, ok bool) {
	rest, ok := strings.CutPrefix(raw, "SVar:")
	if !ok {
		return "", "", false
	}
	name, value, ok = strings.Cut(rest, ":")
	if !ok || name == "" {
		return "", "", false
	}
	return name, value, true
}
