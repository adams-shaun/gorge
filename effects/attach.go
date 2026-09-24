package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Attach", effAttach)
	// kw:For Mirrodin (CR 702.159) expands to exactly the Living Weapon
	// shape in cards/keywords.go -- an enters-the-battlefield trigger that
	// mints a token, remembers it and chains the Attach above -- so it is
	// supported by the same code paths and must be registered here or the
	// report's coverage still counts every carrier as missing a primitive.
	RegisterNonAPI("kw:Equip", "kw:Enchant", "kw:Living Weapon", "kw:For Mirrodin", "kw:Reconfigure", "kw:Fortify")
}

// emitAttach publishes "obj becomes attached to bearer" as events.Attach,
// first publishing events.Unattached when obj was already attached to a
// DIFFERENT permanent. CR 701.3b makes "becomes unattached" a real event
// (the Mode$ Unattached family: Captain's Hook, Grafted Exoskeleton, Grafted
// Wargear, Stitcher's Graft), and an Attach overwriting state.Object.AttachedTo
// would otherwise drop the former bearer's detach entirely -- leaving the
// Grafted Exoskeleton trigger silent when its Equipment is re-equipped from
// one creature to another. Every Attach emit in this package goes through
// here, so a future re-attaching site cannot reintroduce the omission.
//
// The former bearer is carried on Unattached.IDs[0], the same field
// rules/attach.go's attachmentSBAs detach arms use and the referent
// rules/trigger_match.go's triggerRemembered reads for
// Defined$ TriggeredObjectLKICopy. A fresh attach (AttachedTo == 0) or a
// redundant re-attach to the SAME bearer emits no Unattached: nothing became
// unattached, so no trigger may fire.
func emitAttach(h Host, obj, bearer state.ObjID) {
	if o := h.Game().Obj(obj); o != nil && o.AttachedTo != 0 && o.AttachedTo != bearer {
		h.Emit(events.Event{Kind: events.Unattached, Obj: obj,
			IDs:  []state.ObjID{o.AttachedTo},
			Text: "reattached to a new permanent"})
	}
	h.Emit(events.Event{Kind: events.Attach, Obj: obj, IDs: []state.ObjID{bearer}})
}

// Attachable reports whether obj may legally be attached to target. Task 14
// leaves this always-true: the full check includes "the target is not
// protected from the attachment's colours" (CR 702.16e for being attached
// despite protection). That half is NOT added here, and is deliberately NOT
// a host hook on this interface: the protection tasks keep enforcement
// inside rules' emit, where an Attach event is re-checked against its
// target, and effects.Host must never host a protection decision. The hook
// exists so effAttach never hard-codes "always attach" — the one decision
// the attachment SBA does not own on its own.
func Attachable(g *state.Game, obj state.ObjID, target state.ObjID) bool {
	_ = g
	return true
}

// attachSpecAdmitsOffBattlefield is effAttach's CR 704.5m counterpart for the
// graveyard-enchant Aura family (Animate Dead, Dance of the Dead): the
// attaching object's current enchant spec -- at cast time the printed face,
// because the Aura is still on the stack and layer-6 grants live rules-side --
// admits an off-battlefield bearer exactly when the spec matches the bearer
// AND carries an inZone<X> word whose X names the bearer's current zone. The
// zone word is mandatory: MatchesSpecFrom's bare type words are zone-blind
// (spec `Creature` matches a graveyard bear), so without it every Aura would
// be attachable to every zone. An attaching object with no Enchant keyword
// (Equip's kw:Equip expansion, Living Weapon) fails closed to the
// battlefield-only rule.
func attachSpecAdmitsOffBattlefield(g *state.Game, attachObj state.ObjID, tg *state.Object, controller state.PlayerID) bool {
	o := g.Obj(attachObj)
	if o == nil || o.Face() == nil || tg == nil {
		return false
	}
	param, ok := o.Face().KeywordParam("Enchant")
	if !ok || strings.TrimSpace(param) == "" {
		return false
	}
	spec, _, _ := strings.Cut(param, ":")
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return false
	}
	zoneNamed := false
	for word := range strings.SplitSeq(spec, ".") {
		z, has := strings.CutPrefix(word, "inZone")
		if !has {
			continue
		}
		if zn, known := ParseZoneWord(z); known && zn == tg.Zone {
			zoneNamed = true
		}
	}
	if !zoneNamed {
		return false
	}
	return MatchesSpecFrom(g, spec, tg.ID, controller, attachObj)
}

// effAttach implements "Attach": it fastens obj (Object$ Self by default --
// Aura's SP$ Attach is cast with Object$ Self so the STILL-ON-THE-STACK aura
// is the object being attached, Living Weapon's SVar is also Object$ Self
// with Defined$ Remembered naming the germ) onto the first Defined$ target
// that is a legal point of attachment: an object currently on the
// battlefield, not obj itself, and one Attachable accepts. The very first
// legal target wins, which is what makes the living-weapon shape work (the
// freshly minted germ is Remembered[0]).
//
// When no Defined$ target qualifies -- a player target (a player is never an
// attachment point), a non-battlefield object, obj itself, or nothing legal
// at all -- it refuses with a Note rather than emitting an Attach. The
// refusal is how the effect stays deterministic and observable while it has
// nothing legal to do.
//
// Choices$ names the pool the RESOLVING CONTROLLER picks the unspecified
// side from: with no Object$ key it names the OBJECT to attach (Goldwardens'
// Gambit's "you may attach an Equipment you control to it",
// unexpected_request's same shape); with Object$ present it names the
// DESTINATION (Breath of Fury's "attach CARDNAME to a creature you
// control"). The pool is the battlefield sweep the filter admits, evaluated
// with the resolving controller as You; the choice poses a real KChoose
// (Min 0 when Optional$ True -- picking nothing is the decline -- else Min
// 1), the answered ids ride Ctx.AttachChoice (consumed and cleared, fx42
// scoping), and the asking pass's resolved destination list rides the ask so
// a RepeatEach body's Defined$ Imprinted binding -- which does not survive a
// suspension -- never has to be re-derived. A mandatory Min-1 pool with
// exactly one candidate takes it without an ask (the strict-supersets
// convention: a decision nobody could answer differently is never emitted);
// an Optional pool with nothing eligible declines silently, and a mandatory
// one falls to the same Note refusal an illegal destination gets.
//
// An Optional$ True Attach WITHOUT Choices$ (Ajani's Chosen's "you may attach
// it to the token", Cori-Steel Cutter's "you may attach this Equipment to
// it") asks its controller a yes/no KChoose before attaching, through the
// shared Ask boundary with the "attach_optional" resume arm; a decline emits
// no Attach and no refusal Note, and the chained SubAbility$ still runs (the
// suspension plumbing in effects.Resolve owns the chain). A host that
// cannot ask takes the deterministic decline stand-in (R-9); botpolicy's
// clamp fallback answers option 0, which is the "yes" option, so bots
// attach. With NO legal target the existing Note refusal fires and no ask
// is ever posed (a decision nobody could answer differently).
//
// RememberAttached$ True (all seven corpus carriers) puts the permanent just
// attached into the ability's Remembered, both halves -- the ctx list the
// chained SubAbility / the RepeatEach loop's folding reads (the filter
// exclusion Goldwardens' next iteration's !IsRemembered needs) and the
// source's event-backed persistent list eventRemember writes (the same
// two-half discipline RememberTokens$ on effToken and RememberTargets$ on
// effPumpAll apply).
func effAttach(h Host, c *Ctx, sa *cards.SA) {
	// kw:Reconfigure's unattach half (cards/kw_reconfigure.go): the minted
	// ability carries Unattach$ True and resolves to the no-IDs Attach
	// event -- the detach encoding rules/attach.go's CR 704.5n SBA and the
	// bestowed detach already fold. An unattached source (the offer gate in
	// rules/legal.go withholds the ability, so this is only reachable on a
	// stale or malformed answer) refuses with the same Note convention the
	// illegal-destination refusals use, deterministically and observably.
	if strings.EqualFold(strings.TrimSpace(sa.Params["Unattach"]), "True") {
		if src := h.Game().Obj(c.Source); src == nil || src.AttachedTo == 0 {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot unattach: not attached"})
			return
		}
		h.Emit(events.Event{Kind: events.Attach, Obj: c.Source})
		return
	}
	// fx42 scoping: consume and clear the answered attach_choice fields at
	// the top, so a nested Attach in the same chain poses its own ask.
	answered := c.AttachChoice
	answerDests := c.AttachDests
	answeredDone := c.AttachChoiceDone
	c.AttachChoice, c.AttachChoiceDone, c.AttachDests = nil, false, nil

	obj := c.Source
	switch sa.Params["Object"] {
	case "", "Self":
		// Equip's kw:Equip expansion, Enchant's kw:Enchant and Living
		// Weapon's Object$ Self all name the source: today's default, kept
		// byte-identical.
	case "Remembered":
		if ts := objectsOf(c.Remembered); len(ts) > 0 {
			obj = ts[0].Obj
		}
	default:
		// TriggeredCardLKICopy (Ajani's Chosen -- the ENTERING Aura, not the
		// source), Targeted and every other object spec the shared resolver
		// definedSpec already supports. A spec it cannot resolve keeps the
		// today default (obj = c.Source).
		if ts, ok := definedSpec(h, c, sa.Params["Object"]); ok {
			if os := objectsOf(ts); len(os) > 0 {
				obj = os[0].Obj
			}
		}
	}
	// An Aura with Enchant:Player attaches to a seat, not a permanent.
	// The ordinary destination walker below intentionally accepts only
	// battlefield objects; keep other Attach bodies on that existing path.
	if sa.Params["Keyword"] == "Enchant" && sa.Params["Object"] == "Self" {
		if aura := h.Game().Obj(obj); aura != nil && aura.Face() != nil {
			if param, ok := aura.Face().KeywordParam("Enchant"); ok {
				spec, _, _ := strings.Cut(param, ":")
				if spec == "Player" {
					for _, dest := range Defined(h, c, sa) {
						if dest.IsPlayer && int(dest.Player) < len(h.Game().Players) && !h.Game().Players[dest.Player].Lost {
							h.Emit(events.Event{Kind: events.Attach, Obj: obj, Player: dest.Player, Text: "attach to player"})
							return
						}
					}
					h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no legal player"})
					return
				}
			}
		}
	}
	var legalT []state.Target
	destCandidatesFor := func(attachObj state.ObjID) []state.Target {
		var out []state.Target
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				continue
			}
			target := t.Obj
			if target == attachObj {
				continue
			}
			tg := h.Game().Obj(target)
			if tg == nil {
				continue
			}
			if tg.Zone != state.ZBattlefield {
				// The graveyard-enchant Aura family (Animate Dead, Dance of the
				// Dead): the attaching object's CURRENT enchant spec (at cast
				// time the printed face -- the Aura is still on the stack, so
				// no rules-side layer read is available here) zone-positively
				// admits an off-battlefield bearer. Equip and every non-Aura
				// attach keep the battlefield-only rule and the Note refusal.
				if !attachSpecAdmitsOffBattlefield(h.Game(), attachObj, tg, c.Controller) {
					continue
				}
			} else if !Attachable(h.Game(), attachObj, target) {
				continue
			}
			out = append(out, t)
		}
		return out
	}
	// attachTo emits the Attach event and, when RememberAttached$ True, the
	// two-half remember (see the function comment).
	rememberAttached := strings.EqualFold(strings.TrimSpace(sa.Params["RememberAttached"]), "True")
	attachTo := func(target state.ObjID) {
		emitAttach(h, obj, target)
		if rememberAttached {
			c.Remembered = append(c.Remembered, state.Target{Obj: obj})
			eventRemember(h, c, obj)
		}
	}
	// A Choices$ pool the controller picks from: the battlefield sweep the
	// filter admits, evaluated with the resolving controller as You. With
	// Object$ present the pool is the DESTINATION side; without it, the
	// OBJECT side. One sweep serves both the asking pass and the answered
	// re-entry (a pure read: no event, and a suspension between the two
	// changes no state, so the sweep is the same both times).
	//
	// Known limitation: the sweep is battlefield-only -- ChoiceZone$ (e.g.
	// SVar:DBAttach:DB$ Attach | Choices$ Instant | ChoiceZone$ Graveyard)
	// and Chooser$ (e.g. Chooser$ TriggeredCardController) are unread, so a
	// card carrying either takes this branch, finds its wrong-zone or
	// wrong-chooser pool empty, and emits the deterministic "cannot attach:
	// no legal target" refusal -- silently inert, not working.
	if spec := strings.TrimSpace(sa.Params["Choices"]); spec != "" {
		pool := battlefieldValidTargets(h, c, spec)
		// The answered attach_choice re-entry (fx42 scoping: already consumed
		// and cleared at the top). With no Object$ the answer names the
		// OBJECT to attach and the asking pass's resolved destination list
		// rode Ctx.AttachDests; with Object$ present it names the
		// DESTINATION. An empty answer is the Min-0 Optional decline -- or a
		// malformed mandatory answer, whose conservative read is the same
		// no-attach -- and the chain continues via Resolve either way.
		if answeredDone {
			if len(answered) == 0 {
				return
			}
			if _, hasObject := sa.Params["Object"]; hasObject {
				// Object$ present: the answer names the DESTINATION. It is
				// re-checked against the legal destination list recomputed
				// here (the same rejections the asking pass applies), so a
				// stale or malformed answer -- including one naming obj
				// itself, which a raw battlefield sweep CAN admit -- is
				// refused with no Attach (the malformed-answer conservative
				// read) and the chain continues via Resolve.
				legal := false
				for _, t := range pool {
					if t.Obj == obj {
						continue
					}
					if !Attachable(h.Game(), obj, t.Obj) {
						continue
					}
					if t.Obj == answered[0] {
						legal = true
					}
				}
				if !legal {
					return
				}
				attachTo(answered[0])
				return
			}
			// No Object$: the answer names the OBJECT to attach, and the
			// asking pass's resolved destination list rode Ctx.AttachDests
			// (a RepeatEach body's Defined$ Imprinted binding does not
			// survive the suspension, so the re-entry never re-derives it).
			if len(answerDests) == 0 {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no legal target"})
				return
			}
			obj = answered[0]
			attachTo(answerDests[0])
			return
		}
		if _, hasObject := sa.Params["Object"]; hasObject {
			var dest []state.ObjID
			for _, t := range pool {
				if t.Obj == obj {
					continue
				}
				if !Attachable(h.Game(), obj, t.Obj) {
					continue
				}
				dest = append(dest, t.Obj)
			}
			if len(dest) == 0 {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no legal target"})
				return
			}
			max := 1
			if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
				max = 0
			}
			if max == 1 && len(dest) == 1 {
				attachTo(dest[0])
				return
			}
			d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: max, Max: 1,
				Source: c.Source, ResumeKind: "attach_choice", ResumeSA: sa,
				ResumeRemembered: copyTargets(c.Remembered),
				Prompt:           choicePrompt(sa)}
			// The options are the LEGAL destinations, not the raw pool sweep:
			// the pool can hold objects destCandidates' rejection would refuse,
			// including obj itself (aura_graft's Choices$ Permanent admits the
			// attaching Aura, which IS a battlefield Permanent), and offering
			// the object as its own attachment point would self-attach on a
			// bot's option-0 answer (AttachChoice carries Option.Obj, so the
			// re-entry's attachTo(answered[0]) would emit Attach{IDs:[obj]}).
			// Indexing over dest keeps the auto-take above and the re-entry in
			// agreement -- the source can never be offered or selected.
			for i, t := range dest {
				d.Options = append(d.Options, decision.Option{Index: i, Kind: "card", Obj: t, Player: c.Controller})
			}
			_ = Ask(h, d)
			return
		}
		// Object-side pool (Goldwardens' Gambit, unexpected_request, and
		// Yuffie, Materia Hunter): each Choices$ object is a possible object
		// to attach, so destinations must be checked against those candidates,
		// not against the resolving source. Keep only destinations legal for
		// every offered object; the chosen candidate can then use the saved
		// destination list safely after the ask suspends resolution.
		optional := strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True")
		var eligiblePool []state.Target
		for _, candidate := range pool {
			candidateDests := destCandidatesFor(candidate.Obj)
			if len(candidateDests) == 0 {
				continue
			}
			eligiblePool = append(eligiblePool, candidate)
			if len(eligiblePool) == 1 {
				legalT = candidateDests
				continue
			}
			var common []state.Target
			for _, dest := range legalT {
				for _, candidateDest := range candidateDests {
					if candidateDest.Obj == dest.Obj {
						common = append(common, dest)
						break
					}
				}
			}
			legalT = common
		}
		pool = eligiblePool
		if len(pool) == 0 {
			if optional {
				return
			}
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no eligible choice"})
			return
		}
		var legal []state.ObjID
		for _, t := range legalT {
			legal = append(legal, t.Obj)
		}
		if len(legal) == 0 {
			// The existing refusal convention: no legal destination, no ask
			// (a decision nobody could answer differently), one Note.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no legal target"})
			return
		}
		min := 1
		if optional {
			min = 0
		}
		if min == 1 && len(pool) == 1 {
			obj = pool[0].Obj
			attachTo(legal[0])
			return
		}
		d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: min, Max: 1,
			Source: c.Source, ResumeKind: "attach_choice", ResumeSA: sa,
			ResumeRemembered: copyTargets(c.Remembered),
			// The resolved destination list rides the ask: a RepeatEach
			// body's Defined$ Imprinted binding does not survive the
			// suspension, so the re-entry never re-derives it.
			ResumeChoices: copyTargets(legalT),
			Prompt:        choicePrompt(sa)}
		for i, t := range pool {
			d.Options = append(d.Options, decision.Option{Index: i, Kind: "card", Obj: t.Obj, Player: c.Controller})
		}
		_ = Ask(h, d)
		return
	}
	// No Choices$: the destination is the first legal Defined$ target.
	legalT = destCandidatesFor(obj)
	var legal []state.ObjID
	for _, t := range legalT {
		legal = append(legal, t.Obj)
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
		// fx42 scoping: consume and clear the answered election at the top,
		// so a nested Attach in the same chain poses its own ask.
		ans := c.AttachOpt
		c.AttachOpt = ""
		switch {
		case ans == "yes":
			// Answered "attach": fall through to the ordinary attach loop.
		case ans != "":
			// Answered "no" (or any non-affirmative marker): the decline. No
			// Attach, no refusal Note; the chain continues via Resolve.
			return
		default:
			if len(legal) == 0 {
				break // unanswered AND nothing legal: the Note refusal below.
			}
			d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
				Source: c.Source, ResumeKind: "attach_optional", ResumeSA: sa,
				ResumeRemembered: copyTargets(c.Remembered),
				Prompt:           "Attach it?",
				Options: []decision.Option{
					{Index: 0, Kind: "yes", Label: "Yes — attach", Player: c.Controller},
					{Index: 1, Kind: "no", Label: "No", Player: c.Controller},
				}}
			// AskAsked suspends; the answer re-enters with Ctx.AttachOpt set.
			// AskNoHost is the deterministic decline stand-in (R-9) — the
			// same class the search_mayshuffle confirm falls back to (the
			// clamp-answered bot path below answers option 0 = "yes").
			_ = Ask(h, d)
			return
		}
	}
	for _, target := range legal {
		attachTo(target)
		return
	}
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no legal target"})
}

// choicePrompt is a Choices$ Attach's ask prompt: the script's ChoiceTitle$
// when it names one, else the generic default ChooseCard's ask falls back to.
func choicePrompt(sa *cards.SA) string {
	if p := strings.TrimSpace(sa.Params["ChoiceTitle"]); p != "" {
		return p
	}
	return "Choose card"
}
