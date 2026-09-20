package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("CopyPermanent", effCopyPermanent) }

// effCopyPermanent implements DB$ CopyPermanent (task copyp1): create a token
// that is a copy of the object each resolved source names. The mint is one
// events.CopyToken + one MoveZone per copy -- the MyriadCopy precedent, whose
// two-event shape keeps the token's battlefield entry a
// ChangesZone-matchable event every "a creature enters" trigger observes.
//
// The copy carries the copied card's PRINTED characteristics (Card + FaceIdx
// snapshot in the event fold), never the original's counters, attachments,
// damage or current combat state -- CR 706.2's copied-permanent rule, the
// same reading MyriadCopy's comment records.
//
// Source resolution, in order:
//
//   - Populate$ True with no Defined$/ValidTgts$: "a creature token you
//     control" (CR 701.27a), scanned through the ordinary Valid filter
//     grammar. A single eligible token is copied exactly; several keep the
//     deterministic first-candidate stand-in under one Note (the R-9 no-host
//     contract -- the controller's real choice needs an ask this primitive
//     cannot pose); zero eligible tokens is a legitimate no-op (populate does
//     nothing), never a Note.
//   - Defined$: resolved through knownDefinedTargets, FAIL-CLOSED -- an
//     unresolvable selector (ChosenMap, TriggeredSpellAbilityTargets, ...)
//     is one Note and NO mint, never a silent fall-through to the source or
//     the chosen targets (a wrong-copy is worse than no-copy here).
//   - ValidTgts$ (no Defined$): the answered target ask (Ctx.Targets / the
//     mvts1 PickedTargets arm) -- the Flamerush Rider shape.
//   - neither: no copy (defensive; the measured corpus carries no such line
//     outside the mint-blocked families above).
//
// Riders: TokenTapped$ True and TokenAttacking$ True (literal-True arms; the
// TokenAttacking$ True-no-defender and non-True selector values follow
// effToken's degrade -- the copy enters but does not attack, one Note),
// RememberTokens$ True (append each mint to Ctx.Remembered + the event-backed
// remember, so a chained SubAbility$ reads it), AtEOT$ Exile/Sacrifice
// (the builtin delayed-trigger bodies -- __kwWarpExile/__kwEncoreSacrifice --
// registered on the token itself, so Defined$ Self in those bodies IS the
// copy: "exile/sacrifice it at the beginning of the next end step"), AtEOT$
// ExileCombat (the CopyTokenExileCombat bit: the copy is flagged IsMyriad and
// the existing MyriadCleanup sweep exiles it at end of combat), and
// Controller$ You/Targeted*/Remembered*/TriggeredCardController/Opponent.
// Any other Controller$ or AtEOT$ value is one loud Note per call.
func effCopyPermanent(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()

	// One loud Note per call naming every skipped family (never per mint --
	// a NumCopies$ 2 copy must not say it twice). Literally keyed reads only:
	// the param census rejects a dynamic Params key it cannot attribute.
	//
	//   - The mint-blocking families are the source-selection parameters
	//     whose copy is a mid-call choice this build cannot pose from an
	//     effect (the mvts1 sub-ask machinery only covers a body's own
	//     ValidTgts$): the SA carrying one mints NOTHING rather than silently
	//     falling back to a wrong source. Measured population over the 249
	//     raw DB$ CopyPermanent lines: Choices$ 9, DefinedName$ 8, Pawprint$ 1,
	//     RandomCopied$+RandomNum$ 1 (one RandomCopied line),
	//     ValidSupportedCopy$ 1.
	//   - The unblocking families are the characteristic modifications this
	//     round does not implement (brief scope item 5): the copy (when its
	//     source IS reachable) still mints unmodified. AddTypes$, SetPower$,
	//     SetToughness$ and SetColor$ ARE implemented (the Embalm/Eternalize
	//     family and the wider mod census); the remaining modifications
	//     (NonLegendary$, SetCreatureTypes$, RemoveCardTypes$, RemoveSubTypes$,
	//     RemoveCreatureTypes$, AddKeywords$, AddTriggers$, AddSVars$,
	//     PumpKeywords$, AddAbilities$, RemoveKeywords$, WithDifferentNames$,
	//     AttachedTo$, Chooser$) are noted and the copy keeps the original's
	//     printed characteristics. Measured population over the 249 raw
	//     DB$ CopyPermanent lines: AddTypes$ 39, SetPower$ 36, SetToughness$
	//     36, AddKeywords$ 16, SetColor$ 11, NonLegendary$ 20, and
	//     SetCreatureTypes$/RemoveCardTypes$/RemoveSubTypes$/AddTriggers$/
	//     AddSVars$/PumpKeywords$/AddAbilities$/RemoveKeywords$/
	//     WithDifferentNames$/AttachedTo$/Chooser$ in smaller counts.
	var skipped []string
	blocked := false
	note := func(label string) { skipped = append(skipped, label) }
	if _, ok := sa.Params["Choices"]; ok {
		note("Choices$")
		blocked = true
	}
	if _, ok := sa.Params["DefinedName"]; ok {
		note("DefinedName$")
		blocked = true
	}
	if _, ok := sa.Params["Pawprint"]; ok {
		note("Pawprint$")
		blocked = true
	}
	if _, ok := sa.Params["RandomCopied"]; ok {
		note("RandomCopied$")
		blocked = true
	}
	if _, ok := sa.Params["RandomNum"]; ok {
		note("RandomNum$")
		blocked = true
	}
	if _, ok := sa.Params["ValidSupportedCopy"]; ok {
		note("ValidSupportedCopy$")
		blocked = true
	}
	if _, ok := sa.Params["SetCreatureTypes"]; ok {
		note("SetCreatureTypes$")
	}
	if _, ok := sa.Params["RemoveCardTypes"]; ok {
		note("RemoveCardTypes$")
	}
	if _, ok := sa.Params["RemoveSubTypes"]; ok {
		note("RemoveSubTypes$")
	}
	if _, ok := sa.Params["RemoveCreatureTypes"]; ok {
		note("RemoveCreatureTypes$")
	}
	if _, ok := sa.Params["AddKeywords"]; ok {
		note("AddKeywords$")
	}
	if _, ok := sa.Params["AddTriggers"]; ok {
		note("AddTriggers$")
	}
	if _, ok := sa.Params["AddSVars"]; ok {
		note("AddSVars$")
	}
	if _, ok := sa.Params["PumpKeywords"]; ok {
		note("PumpKeywords$")
	}
	if _, ok := sa.Params["AddAbilities"]; ok {
		note("AddAbilities$")
	}
	if _, ok := sa.Params["RemoveKeywords"]; ok {
		note("RemoveKeywords$")
	}
	if _, ok := sa.Params["NonLegendary"]; ok {
		note("NonLegendary$")
	}
	if _, ok := sa.Params["WithDifferentNames"]; ok {
		note("WithDifferentNames$")
	}
	if _, ok := sa.Params["AttachedTo"]; ok {
		note("AttachedTo$")
	}
	if _, ok := sa.Params["Chooser"]; ok {
		note("Chooser$")
	}
	if len(skipped) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "CopyPermanent does not implement " + strings.Join(skipped, ", ") +
				"; the copy keeps the original's printed characteristics"})
	}
	if blocked {
		// The source itself is unreachable: nothing to copy.
		return
	}

	// AtEOT$ that resolves to a shape this round does not implement: the copy
	// still mints and simply stays (one Note per call, never a silent
	// mislaid expiry).
	atEOT := strings.TrimSpace(sa.Params["AtEOT"])
	if atEOT != "" && atEOT != "Exile" && atEOT != "Sacrifice" && atEOT != "ExileCombat" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "AtEOT$ " + atEOT + " is not implemented; the copy stays on the battlefield"})
	}

	// Characteristic modifications (the Embalm/Eternalize family and the
	// wider CopyPermanent mod census): AddTypes$, SetColor$, SetPower$ and
	// SetToughness$. Each is applied as a tracked continuous effect sourced
	// to the minted token itself (the effToken TokenPower$/TokenToughness$
	// precedent), after the mint, so a replay re-derives the identical
	// characteristics from the same AddContinuous calls. A value this build
	// cannot resolve is one loud Note per call and the modification is
	// skipped -- never a silent wrong characteristic.
	var addTypes []string
	if raw, ok := sa.Params["AddTypes"]; ok {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				addTypes = append(addTypes, t)
			}
		}
		if len(addTypes) == 0 {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "AddTypes$ " + strings.TrimSpace(raw) + " resolved to no type; no type added"})
		}
	}
	var addColors []string
	setColor := false
	if raw, ok := sa.Params["SetColor"]; ok {
		cols, parsed := colorLetters(raw)
		if parsed {
			setColor = true
			addColors = cols
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "SetColor$ " + strings.TrimSpace(raw) + " is not a colour; the copy keeps its printed colours"})
		}
	}
	var setPow, setTgh int32
	var hasSetPow, hasSetTgh bool
	if raw, ok := sa.Params["SetPower"]; ok {
		if v, resolved := NumResolved(h, c, sa, "SetPower", 0); resolved {
			setPow, hasSetPow = v, true
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "SetPower$ " + strings.TrimSpace(raw) + " is not resolvable; the copy keeps its printed power"})
		}
	}
	if raw, ok := sa.Params["SetToughness"]; ok {
		if v, resolved := NumResolved(h, c, sa, "SetToughness", 0); resolved {
			setTgh, hasSetTgh = v, true
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "SetToughness$ " + strings.TrimSpace(raw) + " is not resolvable; the copy keeps its printed toughness"})
		}
	}

	// Entry-state riders.
	tapped := false
	if v := strings.TrimSpace(sa.Params["TokenTapped"]); v != "" {
		if strings.EqualFold(v, "True") {
			tapped = true
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "TokenTapped$ " + v + " is not implemented; the copy enters untapped"})
		}
	}
	var attacking bool
	var defender state.PlayerID
	if attack := strings.TrimSpace(sa.Params["TokenAttacking"]); attack != "" {
		switch {
		case strings.EqualFold(attack, "True") && c.DefendingPlayer.IsPlayer:
			attacking = true
			defender = c.DefendingPlayer.Player
		case strings.EqualFold(attack, "True"):
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "TokenAttacking$ with no defending player in context; the copy enters but does not attack"})
		default:
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "TokenAttacking$ " + attack + " is not implemented; the copy enters but does not attack"})
		}
	}

	// Copy count: a literal or resolvable NumCopies$ is honoured; an
	// unresolvable one (X/Y/Wins without a binding) is one copy under a Note.
	n := int32(1)
	if raw, ok := sa.Params["NumCopies"]; ok {
		if v, resolved := NumResolved(h, c, sa, "NumCopies", 1); resolved {
			n = v
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "NumCopies$ " + strings.TrimSpace(raw) + " is not implemented; one copy"})
		}
	}
	if n <= 0 {
		return
	}

	// Copy source.
	spec := strings.TrimSpace(sa.Params["Defined"])
	_, hasTgts := sa.Params["ValidTgts"]
	populate := strings.EqualFold(strings.TrimSpace(sa.Params["Populate"]), "True")
	var targets []state.Target
	switch {
	case populate && spec == "" && !hasTgts:
		sub := *sa
		sub.Params = map[string]string{"Defined": "Valid Creature.token+YouCtrl"}
		cands := Defined(h, c, &sub)
		if len(cands) > 1 {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "Populate$ with several eligible creature tokens copies the first (no engine host to ask)"})
		}
		if len(cands) == 0 {
			// CR 701.27a: with no creature token you control, populate does
			// nothing. A legitimate no-op, not a defect.
			return
		}
		targets = cands[:1]
	case spec != "":
		ts, ok := knownDefinedTargets(h, c, spec)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "CopyPermanent source " + spec + " is not resolvable; no copy"})
			return
		}
		targets = ts
	case hasTgts:
		targets = Defined(h, c, sa)
	default:
		return
	}

	// Controller$ of the copy.
	owner := c.Controller
	switch strings.TrimSpace(sa.Params["Controller"]) {
	case "", "You":
	case "Targeted", "TargetedController", "TargetedPlayer":
		if ps := controllersOf(g, targets); len(ps) > 0 {
			owner = ps[0].Player
		}
	case "Remembered", "RememberedController":
		if ps := controllersOf(g, c.Remembered); len(ps) > 0 {
			owner = ps[0].Player
		}
	case "TriggeredCardController":
		if p, ok := TriggeredCardController(g, c.TriggerContext, c.Remembered); ok {
			owner = p
		}
	case "Opponent":
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				owner = p
				break
			}
		}
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "Controller$ " + strings.TrimSpace(sa.Params["Controller"]) +
				" is not implemented; the copy is controlled by the resolving controller"})
	}

	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberTokens"]), "True")
	var amount int32
	if tapped {
		amount |= events.CopyTokenTapped
	}
	if attacking {
		amount |= events.CopyTokenAttacking
	}
	if atEOT == "ExileCombat" {
		amount |= events.CopyTokenExileCombat
	}
	var ids []state.ObjID
	if attacking {
		ids = []state.ObjID{state.ObjID(defender)}
	}

	for _, t := range targets {
		if t.IsPlayer {
			continue
		}
		if g.Obj(t.Obj) == nil {
			continue
		}
		for i := int32(0); i < n; i++ {
			// want is the ID the mint will get if Apply's CopyToken case
			// actually mints one (state.Game.AddObject assigns NextID then
			// increments it) -- the effToken/effMyriad prediction pattern.
			want := g.NextID
			h.Emit(events.Event{Kind: events.CopyToken, Obj: t.Obj, Player: owner,
				Amount: amount, IDs: ids})
			if g.Obj(want) == nil {
				continue
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: want,
				From: state.ZLibrary, To: state.ZBattlefield})
			// Characteristic modifications, scoped to the copy itself
			// (Affects Card.Self, Source the token). Permanent so the effect
			// outlives its one-shot resolution and lasts as long as the token;
			// the layer system re-derives them from the same calls on replay.
			if len(addTypes) > 0 {
				h.AddContinuous(state.ContinuousEffect{
					Source: want, Controller: owner, Affects: "Card.Self",
					Layer: state.LType, AddTypes: addTypes, Permanent: true,
				})
			}
			if setColor {
				h.AddContinuous(state.ContinuousEffect{
					Source: want, Controller: owner, Affects: "Card.Self",
					Layer: state.LColor, AddColors: addColors, OverwriteColors: true, Permanent: true,
				})
			}
			if hasSetPow || hasSetTgh {
				pow, tgh := int32(0), int32(0)
				if f := g.Obj(want).Face(); f != nil {
					pow, tgh = int32(f.Power()), int32(f.Toughness())
				}
				if hasSetPow {
					pow = setPow
				}
				if hasSetTgh {
					tgh = setTgh
				}
				h.AddContinuous(state.ContinuousEffect{
					Source: want, Controller: owner, Affects: "Card.Self",
					Layer: state.LPT, Sub: state.SubSet,
					SetPower: pow, SetToughness: tgh, HasSet: true, Permanent: true,
				})
			}
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: want})
				eventRemember(h, c, want)
			}
			switch atEOT {
			case "Exile":
				// The registration's source IS the token, so the builtin
				// body's Defined$ Self resolves to it -- the dash/warp
				// precedent. TrackSource rides the __kwWarp prefix, so a copy
				// that left the battlefield and returned as a new incarnation
				// is not exiled by a stale promise (the same one-shot consume
				// warp's end-step exile already had).
				h.Emit(events.Event{Kind: events.DelayedRegister, Obj: want,
					Player: owner, Step: state.StepEnd, Counter: "__kwWarpExile"})
			case "Sacrifice":
				// __kwEncoreSacrifice is exactly the body this needs
				// ("DB$ Sacrifice | Defined$ Self"); the token is its own
				// registration source. Untracked: a sacrificed-then-returned
				// copy keeps the promise, the same semantics encore's group
				// registration holds.
				h.Emit(events.Event{Kind: events.DelayedRegister, Obj: want,
					Player: owner, Step: state.StepEnd, Counter: "__kwEncoreSacrifice"})
			}
		}
	}
}
