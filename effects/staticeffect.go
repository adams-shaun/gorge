package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// StaticEffect$ <name> is the ChangeZone move rider that applies a continuous
// type/keyword/colour change to the moved card ("return it to the battlefield
// ... It's a Spirit Detective"): the value names an SVar on the resolution's
// table whose body is a Mode$ Continuous static line, and the move registers
// that static as a live continuous effect on each card it moved. The layer
// machinery the registration feeds is the same walk the S: static scanner
// feeds (rules/layers.go): layer 4 types/strips, layer 5 colours, layer 6
// keywords/ability grants/removals, layer 7b base P/T.
//
// Scoping is the registration's own, not the body's Affected$ evaluated
// against a remembered set: Forge's Animate bodies scope with Affected$
// Card.IsRemembered -- the cards the move remembered -- and the move path
// already KNOWS that set exactly (the ids it moved), so each moved card gets
// its own registration with Source = the card and Affects = Card.Self, the
// same one-shot shape registerAnimateEffects uses. A body whose Affected$
// names anything else is registered verbatim (Source = the moved card, You =
// the resolving controller): a spec this build cannot evaluate fails closed
// in the ordinary filter grammar and applies to nobody.
//
// Lifetime is the ordinary source-leaves rule (CR 611.2c): with no Duration$
// the grant is active while the moved permanent stays on the battlefield and
// ends when it leaves. A Duration$ the body does carry is honoured where this
// build can read it (Permanent, UntilEOT/end-of-turn, the next-turn spellings
// AddContinuous turns into a real turn boundary) and named in the unread Note
// where it cannot.
//
// Replay: the registration is part of the resolving SA's execution (it emits
// only ClockTick events through AddContinuous, the same channel effEffect
// uses), so a replay re-executing the trigger re-registers the identical
// effects at identical clock stamps.

// staticGrant is one parsed StaticEffect$ body: the layers it asks for and
// the parameters this build does not read.
type staticGrant struct {
	types               []string
	allCreatureTypes    bool
	removeCreatureTypes bool
	removeCardTypes     bool
	addColors           []string
	setColors           []string // SetColor$ overwrites (layer 5)
	addKeywords         []string
	abilities           []string // AddAbility$/AddAbilities$ SVar names
	removeAbilities     bool
	powerExpr, toughExp string
	hasPower, hasTough  bool
	duration            string
	untilEOT            bool
	permanent           bool
	affectedZone        string
	unread              []string
}

// staticEffectTypeList parses an additive TYPE parameter: the comma-separated
// grammar whose elements may carry the " & " conjunction (rules' statList).
// A "ChosenType" element stays verbatim here; the caller resolves it against
// the moved card's own recorded choice exactly as the static scanner's
// resolveChosenTypes does.
func staticEffectTypeList(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		for _, part := range strings.Split(strings.TrimSpace(v), " & ") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// parseStaticEffectGrant parses one Mode$ Continuous body into the layers it
// grants, together with the body's Affected$ spec. Colour values
// colourLetters cannot fully parse fail closed (the Animate Colors$
// direction): the layer is skipped and the parameter named in unread rather
// than an empty parse overwriting the card's colours.
func parseStaticEffectGrant(params map[string]string) (staticGrant, string, bool) {
	var g staticGrant
	if !strings.EqualFold(strings.TrimSpace(params["Mode"]), "Continuous") {
		return g, "", false
	}
	if raw := strings.TrimSpace(params["AddTypes"]); raw != "" {
		g.types = append(g.types, staticEffectTypeList(raw)...)
	}
	if raw := strings.TrimSpace(params["AddType"]); raw != "" {
		g.types = append(g.types, staticEffectTypeList(raw)...)
	}
	g.allCreatureTypes = strings.EqualFold(strings.TrimSpace(params["AddAllCreatureTypes"]), "True")
	g.removeCreatureTypes = strings.EqualFold(strings.TrimSpace(params["RemoveCreatureTypes"]), "True")
	g.removeCardTypes = strings.EqualFold(strings.TrimSpace(params["RemoveCardTypes"]), "True")
	if raw, isAdd := params["AddColor"]; isAdd || params["AddColors"] != "" {
		if !isAdd {
			raw = params["AddColors"]
		}
		if cols, ok := colorLetters(strings.TrimSpace(raw)); ok {
			g.addColors = cols
		} else {
			g.unread = append(g.unread, "AddColor$ "+strings.TrimSpace(raw))
		}
	}
	if raw, isSet := params["SetColor"]; isSet || params["SetColors"] != "" {
		if !isSet {
			raw = params["SetColors"]
		}
		if cols, ok := colorLetters(strings.TrimSpace(raw)); ok {
			g.setColors = cols
		} else {
			g.unread = append(g.unread, "SetColor$ "+strings.TrimSpace(raw))
		}
	}
	g.addKeywords = cards.SplitKeywordList(params["AddKeyword"])
	for _, key := range []string{"AddAbility", "AddAbilities"} {
		for _, nm := range strings.Split(params[key], ",") {
			if nm = strings.TrimSpace(nm); nm != "" {
				g.abilities = append(g.abilities, nm)
			}
		}
	}
	g.removeAbilities = strings.EqualFold(strings.TrimSpace(params["RemoveAllAbilities"]), "True")
	if _, has := params["SetPower"]; has {
		g.powerExpr, g.hasPower = strings.TrimSpace(params["SetPower"]), true
	}
	if _, has := params["SetToughness"]; has {
		g.toughExp, g.hasTough = strings.TrimSpace(params["SetToughness"]), true
	}
	g.duration = strings.TrimSpace(params["Duration"])
	g.affectedZone = strings.TrimSpace(params["AffectedZone"])
	switch {
	case g.duration == "":
	case strings.EqualFold(g.duration, "Permanent"):
		g.permanent = true
	case strings.EqualFold(g.duration, "UntilEOT"), strings.EqualFold(g.duration, "EndOfTurn"):
		g.untilEOT = true
	case IsNextTurnDuration(g.duration):
		// a real turn boundary: AddContinuous resolves it at registration
	default:
		g.unread = append(g.unread, "Duration$ "+g.duration)
		g.duration = ""
	}
	for _, key := range []struct{ name, val string }{
		{"AddStaticAbility$", params["AddStaticAbility"]},
		{"AddTrigger$", params["AddTrigger"]},
		{"AddReplacementEffect$", params["AddReplacementEffect"]},
		{"SetName$", params["SetName"]},
		{"RemoveSubTypes$", params["RemoveSubTypes"]},
	} {
		if strings.TrimSpace(key.val) != "" {
			g.unread = append(g.unread, key.name)
		}
	}
	affects := strings.TrimSpace(params["Affected"])
	// Forge's Affected$ Card.IsRemembered (the one corpus spelling, 55/55)
	// means "the cards this move moved" -- the set the caller already holds
	// exactly -- so the registration pins each moved card (Source = the card,
	// Affects = Card.Self, the registerAnimateEffects shape). Any other spec
	// rides verbatim with Source = the moved card.
	if affects == "" || strings.EqualFold(affects, "Card.IsRemembered") {
		affects = "Card.Self"
	}
	return g, affects, true
}

// applyStaticEffect registers the StaticEffect$ rider of a ChangeZone move
// that landed on the battlefield. Called per moved card (or once with the
// whole moved batch -- the registration is per card either way), after the
// move's own riders (entry counters, control change, tapped entry, attach)
// have been emitted, so the grant applies to the card exactly as it entered.
func applyStaticEffect(h Host, c *Ctx, sa *cards.SA, to state.Zone, moved []state.ObjID) {
	if to != state.ZBattlefield || len(moved) == 0 {
		return
	}
	name := strings.TrimSpace(sa.Params["StaticEffect"])
	if name == "" {
		return
	}
	// StaticEffectCheckSVar$ / StaticEffectSVarCompare$ (Dance of the Manse:
	// the animate only when the cast's X is 6 or more): the named value
	// resolves through the ordinary Num grammar against the resolution's
	// SVar table and must satisfy the comparison. An unresolvable value
	// degrades to 0 (Num's convention) and a failed comparison is a resolved
	// condition being false -- both skip the registration silently; a
	// comparison op this parser does not know fails closed the same way.
	if gate := strings.TrimSpace(sa.Params["StaticEffectCheckSVar"]); gate != "" {
		op := strings.TrimSpace(sa.Params["StaticEffectSVarCompare"])
		if op == "" {
			op = "GT0"
		}
		value := Num(h, c, sa, "StaticEffectCheckSVar", 0)
		lop := "GT"
		th := 0
		if len(op) >= 2 {
			lop = strings.ToUpper(op[:2])
			if n, err := strconv.Atoi(op[2:]); err == nil {
				th = n
			} else {
				return // an unparseable threshold cannot be satisfied
			}
		} else {
			return
		}
		pass := false
		switch lop {
		case "GE":
			pass = value >= int32(th)
		case "GT":
			pass = value > int32(th)
		case "EQ":
			pass = value == int32(th)
		case "LE":
			pass = value <= int32(th)
		case "LT":
			pass = value < int32(th)
		}
		if !pass {
			return
		}
	}
	mode, params := parseStaticLine(c.SVars, name)
	if mode == "" && len(params) == 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "StaticEffect$ " + name + " names no SVar body; not registered"})
		return
	}
	g, affects, ok := parseStaticEffectGrant(params)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "StaticEffect$ " + name + " body is not a Mode$ Continuous static; not registered"})
		return
	}
	// The unread remainder is named ONCE per application, never per card.
	if len(g.unread) > 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "StaticEffect$ " + name + " unread: " + strings.Join(g.unread, "/")})
	}
	for _, id := range moved {
		registerStaticEffectGrant(h, c, id, affects, g)
	}
}

// registerStaticEffectGrant is the one per-card registration path: the same
// layer split registerAnimateEffects uses, with the static body's expression
// parameters (SetPower$/SetToughness$ SVar-capable) left for the layer walk
// to resolve against the grant's own SVar table.
func registerStaticEffectGrant(h Host, c *Ctx, id state.ObjID, affects string, g staticGrant) {
	// A "ChosenType" element resolves against the moved card's own recorded
	// "as this enters" choice (the static scanner's resolveChosenTypes
	// direction); a card with no recorded choice fails closed to no grant
	// rather than leaking the literal word onto the object.
	types := make([]string, 0, len(g.types))
	chosenMissing := false
	if o := h.Game().Obj(id); o != nil {
		for _, t := range g.types {
			if t != "ChosenType" {
				types = append(types, t)
				continue
			}
			if o.ChosenType == "" {
				chosenMissing = true
				continue
			}
			types = append(types, o.ChosenType)
		}
	}
	if len(types) > 0 || g.removeCardTypes || g.removeCreatureTypes || g.allCreatureTypes {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: affects, Controller: c.Controller,
			Layer:               state.LType,
			AddTypes:            types,
			AddAllCreatureTypes: g.allCreatureTypes,
			RemoveCreatureTypes: g.removeCreatureTypes,
			RemoveCardTypes:     g.removeCardTypes,
			AffectedZone:        g.affectedZone,
			Duration:            g.duration, Permanent: g.permanent, UntilEOT: g.untilEOT,
			SVars: c.SVars,
		})
	}
	if chosenMissing {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "StaticEffect$ ChosenType names a card with no recorded choice"})
	}
	if len(g.setColors) > 0 {
		// SetColor$ is the layer-5 overwrite (the scanner's direction): the
		// moved card's colours are exactly the named set.
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: affects, Controller: c.Controller,
			Layer:           state.LColor,
			AddColors:       g.setColors,
			OverwriteColors: true,
			AffectedZone:    g.affectedZone,
			Duration:        g.duration, Permanent: g.permanent, UntilEOT: g.untilEOT,
		})
	}
	if len(g.addColors) > 0 {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: affects, Controller: c.Controller,
			Layer:        state.LColor,
			AddColors:    g.addColors,
			AffectedZone: g.affectedZone,
			Duration:     g.duration, Permanent: g.permanent, UntilEOT: g.untilEOT,
		})
	}
	if len(g.addKeywords) > 0 {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: affects, Controller: c.Controller,
			Layer:       state.LAbilities,
			AddKeywords: g.addKeywords,
			Duration:    g.duration, Permanent: g.permanent, UntilEOT: g.untilEOT,
			SVars: c.SVars,
		})
	}
	if g.removeAbilities {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: affects, Controller: c.Controller,
			Layer:           state.LAbilities,
			RemoveAbilities: true,
			Duration:        g.duration, Permanent: g.permanent, UntilEOT: g.untilEOT,
		})
	}
	if len(g.abilities) > 0 {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: affects, Controller: c.Controller,
			Layer:        state.LAbilities,
			AddAbilities: g.abilities,
			Duration:     g.duration, Permanent: g.permanent, UntilEOT: g.untilEOT,
			SVars: c.SVars,
		})
	}
	if g.hasPower || g.hasTough {
		h.AddContinuous(state.ContinuousEffect{
			Source: id, Affects: affects, Controller: c.Controller,
			Layer: state.LPT, Sub: state.SubSet,
			SetPowerExpr:        g.powerExpr,
			SetToughnessExpr:    g.toughExp,
			SetPowerPresent:     g.hasPower,
			SetToughnessPresent: g.hasTough,
			StaticSet:           true,
			HasSet:              true,
			Duration:            g.duration, Permanent: g.permanent, UntilEOT: g.untilEOT,
			SVars: c.SVars,
		})
	}
}
