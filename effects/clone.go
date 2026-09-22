package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Clone", effClone) }

// effClone implements DB$ Clone (api:Clone), CR 613.1a's layer-1 copy: an
// EXISTING permanent becomes a copy of another object. It is the ONE
// primitive both clone routes call -- the standalone "CARDNAME becomes a copy
// of target creature" family (Vesuvan Doppelganger, Lazav, Body Double) and,
// once the ETB-copy replacement ticket lands, the "you may have it enter as a
// copy" family (Vizier of Many Faces), whose body is the same DB$ Clone with
// CloneTarget$ ReplacedCard.
//
// Two operands, matching Forge's CloneEffect:
//
//   - the copy SOURCE, i.e. the object whose characteristics are copied:
//     Defined$ when present, else the SA's own chosen targets (the
//     ValidTgts$ "copy target creature" shape), else a Choices$ pick.
//   - the BECOME operand, i.e. the object that turns into the copy:
//     CloneTarget$ when present, else the SA's own source (Self) -- "this
//     permanent becomes a copy".
//
// The copy itself is one events.ClonePermanent event, folded in Apply onto
// the target object's CopyFace basis. Routing the basis through
// state.Object.Face() is what makes every reader in the tree (name, types,
// keywords, colours, P/T, abilities, triggers, statics, mana production) see
// the copied characteristics by construction (CR 707.2) instead of each call
// site having to consult the layer system.
//
// The characteristic EXCEPTIONS are separate continuous effects at their own
// CR 613 layers, registered against the become object, so the copied face
// stays the source's printed face and the walk settles the exceptions in
// order: AddTypes$/RemoveCardTypes$/RemoveCreatureTypes$ are layer 4,
// SetColor$ is layer 5, AddKeywords$ is layer 6, SetPower$/SetToughness$ are
// layer 7b. NewName$ rides the event (the copy's name) and GainThisAbility$
// True keeps the original object's own abilities and SVar table on the copy.
//
// Duration$ is honoured through the ordinary continuous-effect lifetime: a
// permanent copy (no Duration$, or Permanent) is cleared by the become
// object leaving the battlefield (CR 400.7, Move clears the basis); an
// UntilEndOfTurn copy is cleared at that cleanup (EndOfTurnCleanup's
// clone sweep); UntilYourNextTurn / UntilTheEndOfYourNextTurn use the
// engine's turn boundary; UntilUnattached clears when the become object is no
// longer attached (the clone sweep's attached check). A duration this build
// cannot place gets one loud Note and the copy lasts until the object leaves
// the battlefield.
func effClone(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()

	// The answered Optional$ may-copy election, consumed and cleared at the
	// top of the walk (the fx42 scoping discipline): a nested Clone cannot
	// inherit the outer answer.
	cloneAns := c.Clone
	cloneDone := c.CloneDone
	c.Clone, c.CloneDone = "", false

	// Copy SOURCE.
	var source []state.Target
	spec := strings.TrimSpace(sa.Params["Defined"])
	switch {
	case spec != "":
		ts, ok := knownDefinedTargets(h, c, spec)
		if !ok {
			// Fail closed: a source this build cannot resolve is one loud Note
			// and NO copy, never a silent fall-through to a wrong object (the
			// CopyPermanent convention -- a wrong copy is worse than none).
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "Clone source " + spec + " is not resolvable; no copy"})
			return
		}
		source = ts
	case strings.TrimSpace(sa.Params["Choices"]) != "":
		// Choices$ <filter> is Forge's mid-resolution chooser for the copy
		// source. This build poses the deterministic first-eligible
		// battlefield pick under one Note (the R-9 no-host contract; the
		// real per-player ask is the overlap the ETB-copy ticket carries).
		cs, ok := cloneChoiceSource(h, c, strings.TrimSpace(sa.Params["Choices"]))
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "Clone Choices$ " + strings.TrimSpace(sa.Params["Choices"]) +
					" has no eligible object; no copy"})
			return
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "Clone Choices$ picks the first eligible object (no engine host to ask)"})
		source = cs
	default:
		// No Defined$/Choices$: the SA's own chosen target is the object to
		// copy (the "target creature you control becomes a copy of target
		// creature" family has one target being both source and become).
		for _, t := range c.Targets {
			if !t.IsPlayer {
				source = append(source, t)
			}
		}
		if len(source) == 0 {
			return
		}
	}

	// BECOME operand(s).
	become, ok := cloneBecome(h, c, sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "Clone CloneTarget$ " + strings.TrimSpace(sa.Params["CloneTarget"]) +
				" is not resolvable; no copy"})
		return
	}
	if len(become) == 0 {
		return
	}

	// Optional$ True: the copier -- the resolving controller, who for every
	// corpus carrier is also the become object's controller -- takes the real
	// may-copy election (ticket api-clone-trigger-copy; Sarkhan Soul Aflame's
	// "you may have CARDNAME become a copy of it"). The ask re-enters the
	// whole walk with Ctx.Clone/CloneDone set; the answered decline returns
	// without copying. A no-host run (an effects test double, a fuzz run)
	// keeps the deterministic take stand-in the pre-election build shipped,
	// byte-identical (the same convention the optional-discard family
	// records) -- a "may" that cannot ask never wedges.
	if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
		if !cloneDone {
			prompt := "You may have a permanent become a copy?"
			if ob := g.Obj(become[0].Obj); ob != nil && ob.Face() != nil {
				prompt = "You may have " + ob.Face().Name + " become a copy?"
			}
			d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
				Source:     c.Source,
				ResumeKind: "clone", ResumeSA: sa,
				Prompt: prompt,
				Options: []decision.Option{
					{Index: 0, Kind: "yes", Label: "Yes — make the copy", Player: c.Controller},
					{Index: 1, Kind: "no", Label: "No", Player: c.Controller},
				}}
			if Ask(h, d) == AskAsked {
				return // resolution suspended; the answer re-enters with Ctx.Clone set.
			}
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "Clone Optional$ resolved as take (no engine host to ask)"})
		} else if cloneAns != "yes" {
			// The answered decline: no copy. The decision_made event already
			// carries the answer, so nothing else is emitted.
			return
		}
	}

	// Collect the modifier registrations once; every become object shares
	// them. An unreadable modifier is one Note per call (never per object).
	addTypes := splitAmp(strings.TrimSpace(sa.Params["AddTypes"]))
	addKeywords := cards.SplitKeywordList(sa.Params["AddKeywords"])
	newName := strings.TrimSpace(sa.Params["NewName"])
	gainThisAbility := strings.EqualFold(strings.TrimSpace(sa.Params["GainThisAbility"]), "True")
	removeCardTypes := strings.EqualFold(strings.TrimSpace(sa.Params["RemoveCardTypes"]), "True")
	removeCreatureTypes := strings.EqualFold(strings.TrimSpace(sa.Params["RemoveCreatureTypes"]), "True")
	setPowerPresent, setPower := clonePT(h, c, sa, "SetPower")
	setToughPresent, setTough := clonePT(h, c, sa, "SetToughness")
	colorSpec := strings.TrimSpace(sa.Params["SetColor"])
	var setColors []string
	var setColorPresent bool
	if colorSpec != "" {
		letters, ok := colorLetters(colorSpec)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "Clone SetColor$ " + colorSpec + " is not a colour this build can set; the copy keeps its colours"})
		} else {
			// SetColor$ is an overwrite (CR 613.1e "becomes"); an empty parse
			// (Colorless) is an overwrite to colourless, which the layer walk
			// honours through OverwriteColors with an empty AddColors. The
			// PRESENCE bit is tracked separately from the letters for exactly
			// that case: keying the registration on len(setColors) would make
			// SetColor$ Colorless a silent no-op.
			setColors = letters
			setColorPresent = true
		}
	}
	// Purely inert riders: one loud Note naming each, the copy proceeds
	// without them (the digUntilParamValue convention: the key is the
	// helper's own parameter, every call site a string literal).
	var unread []string
	for _, key := range cloneUnreadModifiers {
		if v := cloneParamValue(sa, key); v != "" {
			unread = append(unread, key+"$ "+v)
		}
	}
	// The `!cloneDone` guard the first cut carried here was WRONG: with a
	// real host the initial pass always returns at the Ask above, so these
	// diagnostics can only ever fire on the ANSWERED-YES re-entry (the
	// decline path returned before this point) -- gating them on
	// `!cloneDone` silenced them for exactly the carriers that ask
	// (findings-r2 MAJOR; 7 corpus Optional$+AddSVars$ lines incl. Kimahri,
	// Vesuvan Doppelganger, Lazav). The no-host path keeps cloneDone=false,
	// so it still emits once.
	if len(unread) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "Clone does not read: " + strings.Join(unread, ", ")})
	}

	dur := strings.TrimSpace(sa.Params["Duration"])
	// permanent is the "no Duration$/Permanent" classification; every clone
	// effect is registered with Permanent=false (see reg below), so the flag
	// itself is not carried onto the effects -- the no-duration case is simply
	// a unit with no expiry field, kept until the become object leaves.
	_, untilEOT, untilTurn, untilUnattached, durNote := cloneDuration(dur)
	// Same shape as the unread-modifier Note above: reachable only on the
	// answered-yes re-entry (real host) or the no-host pass, never
	// duplicated.
	if durNote != "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller, Text: durNote})
	}

	for _, t := range source {
		if t.IsPlayer {
			continue
		}
		srcObj := g.Obj(t.Obj)
		if srcObj == nil || srcObj.Face() == nil {
			continue
		}
		for _, b := range become {
			if b.IsPlayer {
				continue
			}
			obj := g.Obj(b.Obj)
			if obj == nil || obj.Zone != state.ZBattlefield {
				continue
			}
			// One ClonePermanent event per (source, become) pair; the fold
			// snapshots the source's printed face onto the become object.
			ev := events.Event{Kind: events.ClonePermanent, Obj: b.Obj,
				IDs: []state.ObjID{t.Obj}, Player: c.Controller, Text: newName}
			if gainThisAbility {
				ev.Counter = "gain-this-ability"
			}
			h.Emit(ev)

			// Modifier layers, scoped to the become object (Card.Self with
			// Source = its own id, the effPump convention). The lifetime is
			// ALWAYS the source-leaves rule (Permanent=false): CR 400.7 makes
			// the object a new object the instant it leaves the battlefield, so
			// the copy and its modifiers must not follow it. active() drops the
			// unit on the source-leaves check and effectMoveSweep removes it from
			// e.continuous when the become object leaves (the CR 611.2a
			// "Permanent" flag would keep it applying to a re-entered object).
			reg := func(ce state.ContinuousEffect) {
				ce.Source = b.Obj
				ce.Affects = "Card.Self"
				ce.Controller = c.Controller
				ce.Duration = dur
				ce.Permanent = false
				ce.UntilEOT = untilEOT
				ce.UntilTurn = untilTurn
				ce.CloneTarget = b.Obj
				h.AddContinuous(ce)
			}
			if len(addTypes) > 0 || removeCardTypes || removeCreatureTypes {
				reg(state.ContinuousEffect{Layer: state.LType, AddTypes: addTypes,
					RemoveCardTypes: removeCardTypes, RemoveCreatureTypes: removeCreatureTypes})
			}
			if setColorPresent {
				reg(state.ContinuousEffect{Layer: state.LColor, AddColors: setColors, OverwriteColors: true})
			}
			if len(addKeywords) > 0 {
				reg(state.ContinuousEffect{Layer: state.LAbilities, AddKeywords: addKeywords})
			}
			if setPowerPresent || setToughPresent {
				reg(state.ContinuousEffect{Layer: state.LPT, Sub: state.SubSet, HasSet: true,
					SetPower: setPower, SetToughness: setTough,
					SetPowerPresent: setPowerPresent, SetToughnessPresent: setToughPresent,
					StaticSet: true})
			}
			// The layer-1 LCopy MARKER owns the copy's lifetime. It is always
			// registered (even when no modifier effect is), so rules' clone
			// sweep has exactly one owner per copy to expire and can drop the
			// marker's sibling effects with it. UntilUnattached is enforced by
			// EndOfTurnCleanup's attached check, which reads the marker's
			// Duration; every other duration rides UntilEOT/UntilTurn or the
			// source-leaves rule.
			//
			// The marker also CARRIES the copy (source id, NewName$,
			// GainThisAbility$) so that expiring one unit on an object that
			// carries ANOTHER live unit re-bases the object onto the
			// survivor instead of clearing the shared CopyFace basis.
			_ = untilUnattached
			reg(state.ContinuousEffect{Layer: state.LCopy, CloneSource: t.Obj,
				CloneName: newName, CloneGainThisAbility: gainThisAbility})
		}
	}
}

// cloneUnreadModifiers are DB$ Clone modifier parameters this build records
// but does not act on. Each present one lands in the single combined
// loud Note per clone call so the parameter census stays honest; measured
// corpus populations at FORGE_REF:
// AddTriggers$ 3, AddStaticAbilities$ 1, AddAbilities$ 1, SetCreatureTypes$ 1,
// RemoveSubTypes$ 1, NonLegendary$ 6, AddSVars$ (read only through
// GainThisAbility's merged SVar table) 9, AttachedTo$/CopyFromChosenName$/
// CloneZone$/FaceDown$ the remaining singletons.
// IntoPlayTapped$ is in this list deliberately. It means "the copy ENTERS
// tapped", which only has a referent on the ETB-replacement route (Vesuva,
// Echoing Deeps, Callidus Assassin -- every measured carrier is an
// ETBReplacement body). On the STANDALONE route this build ships, the become
// object is already on the battlefield and nothing is entering, so tapping it
// would be an invented cost. No standalone corpus carrier passes the
// parameter, so it is recorded and inert until the ETB-copy ticket lands and
// can read it against real entry provenance.
var cloneUnreadModifiers = []string{
	"AddTriggers", "AddStaticAbilities", "AddAbilities", "AddSVars",
	"SetCreatureTypes", "RemoveSubTypes", "NonLegendary", "AttachedTo",
	"CopyFromChosenName", "CloneZone", "FaceDown", "KeepFacedown",
	"IntoPlayTapped",
}

// cloneParamValue is the unread-modifier keys' trimmed value read (the
// paramcensus's dynamic-key rule: the key is this helper's own parameter,
// and every call site passes a string literal). Empty means absent-or-False.
func cloneParamValue(sa *cards.SA, key string) string {
	v := strings.TrimSpace(sa.Params[key])
	if strings.EqualFold(v, "False") {
		return ""
	}
	return v
}

// cloneBecome resolves CloneTarget$. Absent means Self (the resolving
// ability's own source object) -- "this permanent becomes a copy". The
// named forms reuse the same Defined$ referent grammar the source half uses.
func cloneBecome(h Host, c *Ctx, sa *cards.SA) ([]state.Target, bool) {
	spec := strings.TrimSpace(sa.Params["CloneTarget"])
	if spec == "" {
		if c.Source == 0 {
			return nil, true
		}
		return []state.Target{{Obj: c.Source}}, true
	}
	if strings.EqualFold(spec, "Self") {
		return []state.Target{{Obj: c.Source}}, true
	}
	// CloneTarget$ Valid <spec>: every battlefield object the filter admits
	// (the "each other creature you control becomes a copy" shape), through
	// the same battlefield sweep Defined's Valid form uses.
	if rest, ok := strings.CutPrefix(spec, "Valid "); ok {
		return battlefieldValidTargets(h, c, strings.TrimSpace(rest)), true
	}
	return knownDefinedTargets(h, c, spec)
}

// cloneChoiceSource resolves a Choices$ <filter> pick to the first eligible
// battlefield object in deterministic scan order. That is the R-9 no-host
// stand-in for the real per-player choice; ok is false when nothing matches.
func cloneChoiceSource(h Host, c *Ctx, spec string) ([]state.Target, bool) {
	g := h.Game()
	filter := spec
	if !strings.Contains(filter, ".") && !strings.HasPrefix(filter, "Card") {
		// A bare type word is a Card-basis filter ("Creature.Other" is
		// already a basis; "Creature" alone is not).
		filter = "Card." + filter
	}
	sc := c.SpecContext(c.Controller)
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil {
				continue
			}
			if MatchesObjectCtx(g, filter, o, sc) {
				return []state.Target{{Obj: id}}, true
			}
		}
	}
	return nil, false
}

// clonePT reads a SetPower$/SetToughness$ modifier through the shared numeric
// grammar (literal, X, or an SVar name). present reports whether the
// parameter was given at all, so a setter that names only one characteristic
// leaves the other alone (the continuous-effect StaticSet contract).
func clonePT(h Host, c *Ctx, sa *cards.SA, key string) (present bool, value int32) {
	if _, ok := sa.Params[key]; !ok {
		return false, 0
	}
	return true, Num(h, c, sa, key, 0)
}

// cloneDuration maps a DB$ Clone Duration$ value onto the continuous-effect
// lifetime fields. untilTurn is left zero for the AddContinuous call to fill
// from the live rotation (the UntilYourNextTurn path). durNote, when
// non-empty, is the one loud Note for a duration this build cannot place.
//
// An UNKNOWN duration is deliberately NOT permanent: it gets the
// source-leaves lifetime (until the become object leaves the battlefield)
// rather than lasting for the rest of the game, so a value this build cannot
// place never silently over-extends a copy. Measured corpus values at
// FORGE_REF (raw `DB$ Clone` lines): UntilEndOfTurn 33, UntilYourNextTurn 5,
// UntilUnattached 5, one each of UntilTargetedUntaps, UntilNextEndStep,
// UntilHostLeavesPlay, UntilFacedown and EOT; 105 lines carry no Duration$
// (a permanent copy).
func cloneDuration(dur string) (permanent, untilEOT bool, untilTurn int32, untilUnattached bool, note string) {
	switch strings.ToLower(strings.TrimSpace(dur)) {
	case "", "permanent":
		return true, false, 0, false, ""
	case "untilendofcombat":
		// durationTiming's combat scope: dropped by EndOfTurnCleanup on the
		// same turn (the engine's UntilEndOfCombat reclamation).
		return false, false, 0, false, ""
	case "untileadofturn", "untilendofturn", "eot":
		return false, true, 0, false, ""
	case "untilyournextturn", "untiltheendofyournextturn":
		// AddContinuous computes the real turn boundary from Duration.
		return false, false, 0, false, ""
	case "untilyournextendstep", "untilnextendstep":
		// The engine's until-next-end-step window is this turn's cleanup, the
		// same mapping effects.effectUntilEOT uses for this spelling (the one
		// corpus carrier is niko_light_of_hope).
		return false, true, 0, false, ""
	case "untilunattached":
		return false, false, 0, true, ""
	case "untilhostleavesplay":
		// Exactly the source-leaves lifetime the default arm gives an unknown
		// duration, so no Note is needed (secret_invasion).
		return false, false, 0, false, ""
	case "untilfacedown":
		return false, false, 0, false,
			"Clone Duration$ UntilFacedown is approximated as until the copy leaves the battlefield (no turn-face-down expiry)"
	case "untiltargeteduntaps":
		return false, false, 0, false,
			"Clone Duration$ UntilTargetedUntaps is approximated as until the copy leaves the battlefield (no untap-tracked expiry)"
	default:
		return false, false, 0, false,
			"Clone Duration$ " + dur + " is not implemented; the copy lasts until the object leaves the battlefield"
	}
}

// splitAmp splits a Forge "&"-compound type list ("Shapeshifter & Rogue").
func splitAmp(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "&")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
