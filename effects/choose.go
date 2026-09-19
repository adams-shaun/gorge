package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
)

// ChooseType, ChooseNumber and ChooseColor record a choice on the source.
// With the choice already present (the cast-time "as this enters" ask in
// rules/cast.go's etbAsk recorded it with a Choose event before this ever
// resolves, plan ruling R-6) these do nothing. Without one -- a script that
// uses them at RESOLUTION time -- ChooseType poses a real KChoose ask
// through Host.TypeChoices (task ct1; the suspension re-enters through
// rules' "choosetype" resume arm and Ctx.ChosenType), falling back to the
// deterministic pick below only when the host cannot ask or the option list
// is empty. ChooseNumber and ChooseColor remain silent fallbacks (0 /
// first-WUBRG "W") -- the sibling stand-ins the ledger tracks.
func init() {
	Register("ChooseType", effChooseType)
	Register("ChooseNumber", effChooseNumber)
	Register("ChooseColor", effChooseColor)
}

// effChooseColor records a colour choice. With the source already carrying a
// ChosenColor (the cast-time Choose "color" event set it) it is a no-op;
// without one it records the deterministic first-WUBRG "W" -- the same
// silent-fallback convention effChooseType applies, never a louder variant.
// A SP$/AB$ ChooseColor mid-resolution ask (Wash Out, Nyx Lotus's devotion
// ability) therefore resolves to W deterministically instead of the old
// "unimplemented API" note: a silent-er degradation the ledger tracks.
func effChooseColor(h Host, c *Ctx, _ *cards.SA) {
	if o := h.Game().Obj(c.Source); o != nil && o.ChosenColor != "" {
		return
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "color", Text: "W"})
}

// effChooseNumber records a number choice. With the source already carrying a
// non-zero ChosenNumber (the cast-time Choose event set it), it is a no-op;
// otherwise it records the deterministic fallback 0. The non-zero guard is
// why a cast-flow choice of x=0 can never be re-asked but also means a
// legitimate "chosen 0 outside an ETB" is indistinguishable from "never
// asked" -- both fall back to recording 0, which is the same value anyway, so
// the ambiguity is unobservable.
func effChooseNumber(h Host, c *Ctx, sa *cards.SA) {
	if o := h.Game().Obj(c.Source); o != nil && o.ChosenNumber != 0 {
		return
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "number", Amount: 0})
}

// effChooseType records a creature-type choice. With the source already
// carrying a ChosenType it is a no-op (the cast-time ask pre-recorded it);
// on the re-entry after its own ask was answered it emits exactly the Choose
// event the fallback emits, with the answered type (Ctx.ChosenType, consumed
// and cleared -- fx42). On the first pass it poses a real KChoose ask over
// Host.TypeChoices' option list when two or more types are offerable, so the
// chooser picks; with zero or one offerable type the choice is forced (or
// empty) and the single legal answer equals the fallback's deterministic
// pick, so no ask is posed (the effDiscard strict-supersets convention). A
// host that cannot ask falls through to the same fallback with no extra
// Note (R-9). Type$ (Herald's Horn, Urza's Incubator, Roaming Throne, Three
// Tree City) names the CATEGORY the choice ranges over: "Creature" (the
// corpus's dominant value) and an absent Type$ ask over the creature-type
// list; any other category (Basic Land, Card, Land, Planeswalker, ...) has
// no option builder in this build and keeps the loud Note plus the
// creature-type fallback rather than being offered a list that cannot
// answer the question.
func effChooseType(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	if o := g.Obj(c.Source); o != nil && o.ChosenType != "" {
		return
	}
	cat := strings.TrimSpace(sa.Params["Type"])
	if cat != "" && !strings.EqualFold(cat, "Creature") {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ChooseType Type$ " + cat + " is not a category this engine can ask; the choice falls back to creature types"})
	} else if answered := c.ChosenType; answered != "" {
		// The "choosetype" resume arm's answer: emit the same Choose event the
		// fallback emits, with the answered type, so events.Apply records
		// o.ChosenType exactly the way every downstream reader already reads.
		c.ChosenType = ""
		h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "type", Text: answered})
		return
	} else {
		chooser := c.Controller
		if ts := Defined(h, c, sa); len(ts) > 0 && ts[0].IsPlayer {
			chooser = ts[0].Player
		}
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 1, Max: 1,
			ResumeKind: "choosetype", ResumeSA: sa, Prompt: "Choose a creature type", Source: c.Source}
		d.Options = h.TypeChoices(chooser, cat)
		if len(d.Options) > 1 && Ask(h, d) == AskAsked {
			return
		}
	}
	var fallback string
	for i := range g.Objs {
		o := &g.Objs[i]
		if fallback != "" || o.Controller != c.Controller {
			continue
		}
		f := o.Face()
		if f == nil || !hasType(o, "Creature") {
			continue
		}
		for _, t := range f.Types {
			if CreatureTypeWords(t) {
				fallback = t
				break
			}
		}
	}
	if fallback == "" {
		fallback = "Human"
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "type", Text: fallback})
}

// CreatureTypeWords reports whether a Type token is a creature subtype. It
// shares the positive vocabulary Changeling uses, so a cast-time type choice
// cannot offer a spell, plane, or planeswalker subtype as a creature type.
func CreatureTypeWords(t string) bool { return creatureSubtypeWords[t] }
