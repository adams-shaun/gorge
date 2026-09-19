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
	RegisterNonAPI("kw:Equip", "kw:Enchant", "kw:Living Weapon")
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
// An Optional$ True Attach (Ajani's Chosen's "you may attach it to the
// token", Cori-Steel Cutter's "you may attach this Equipment to it") asks
// its controller a yes/no KChoose before attaching, through the shared Ask
// boundary with the "attach_optional" resume arm; a decline emits no Attach
// and no refusal Note, and the chained SubAbility$ still runs (the
// suspension plumbing in effects.Resolve owns the chain). A host that
// cannot ask takes the deterministic decline stand-in (R-9); botpolicy's
// clamp fallback answers option 0, which is the "yes" option, so bots
// attach. With NO legal target the existing Note refusal fires and no ask
// is ever posed (a decision nobody could answer differently).
func effAttach(h Host, c *Ctx, sa *cards.SA) {
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
	var legal []state.ObjID
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		target := t.Obj
		if target == obj {
			continue
		}
		tg := h.Game().Obj(target)
		if tg == nil || tg.Zone != state.ZBattlefield {
			continue
		}
		if !Attachable(h.Game(), obj, target) {
			continue
		}
		legal = append(legal, target)
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
		h.Emit(events.Event{Kind: events.Attach, Obj: obj, IDs: []state.ObjID{target}})
		return
	}
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no legal target"})
}
