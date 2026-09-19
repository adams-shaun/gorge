package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

// ChooseType, ChooseNumber and ChooseColor record an "as this enters"
// choice on the source. The real choice is asked by rules at cast time and
// recorded with a Choose event before this ever resolves (plan ruling R-6),
// so with a choice already present these do nothing. Without one -- a script
// that uses them outside an ETB replacement -- they record the deterministic
// fallback (the first creature type the controller owns / 0 / first-WUBRG
// "W") rather than asking, which M2b's mid-resolution decisions replace.
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
// carrying a ChosenType it is a no-op; without one, it names the first
// creature subtype of the controller's own objects (in object-ID order,
// i.e. deterministic), falling back to "Human" when the controller owns no
// creature subtype at all. Type$ (Herald's Horn, Urza's Incubator, Roaming
// Throne, Three Tree City) names the CATEGORY the choice ranges over:
// "Creature" (the corpus's dominant value, 125 ChooseType lines) is exactly
// the creature-type list this fallback and the cast-time option list build;
// any other category (Basic, Card, Land, ColorOrType, ...) has no option
// builder in this build and is recorded loudly rather than silently offered
// a creature-type list that cannot answer the question.
func effChooseType(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	if o := g.Obj(c.Source); o != nil && o.ChosenType != "" {
		return
	}
	if cat := strings.TrimSpace(sa.Params["Type"]); cat != "" && !strings.EqualFold(cat, "Creature") {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ChooseType Type$ " + cat + " is not a category this engine can ask; the choice falls back to creature types"})
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
