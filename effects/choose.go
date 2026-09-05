package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

// ChooseType and ChooseNumber record an "as this enters" choice on the
// source. The real choice is asked by rules at cast time and recorded with a
// Choose event before this ever resolves (plan ruling R-6), so with a
// choice already present these do nothing. Without one -- a script that uses
// them outside an ETB replacement -- they record the deterministic fallback
// (the first creature type the controller owns / 0) rather than asking,
// which M2b's mid-resolution decisions replace.
func init() {
	Register("ChooseType", effChooseType)
	Register("ChooseNumber", effChooseNumber)
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
// creature subtype at all.
func effChooseType(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	if o := g.Obj(c.Source); o != nil && o.ChosenType != "" {
		return
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

// cardTypeWords and superTypeWords partition a face's Types list: anything
// that is not one of these is a creature subtype. Both fixtures and the
// corpus spell these capitalized exactly; comparing against the set (rather
// than, say, "the non-Creature entries") is what lets "Creature Human Cleric"
// name Human and Cleric while "Legendary Creature" names nothing.
var (
	cardTypeWords = map[string]bool{
		"Artifact": true, "Battle": true, "Conspiracy": true, "Creature": true,
		"Dungeon": true, "Enchantment": true, "Instant": true, "Land": true,
		"Phenomenon": true, "Plane": true, "Planeswalker": true, "Scheme": true,
		"Sorcery": true, "Tribal": true, "Vanguard": true,
	}
	superTypeWords = map[string]bool{
		"Basic": true, "Eladamri": true, "Host": true, "Legendary": true,
		"Ongoing": true, "Snow": true, "World": true,
	}
)

// CreatureTypeWords reports whether a Type token is a creature subtype
// (i.e. neither a card type nor a supertype), so the rules package can build
// a kept-in-sync option list for a cast-time ChooseType without duplicating
// the vocabulary. Exported because the rules package's etbChoices stage owns
// the type option list and must agree byte-for-byte with effChooseType's
// fallback.
func CreatureTypeWords(t string) bool { return !cardTypeWords[t] && !superTypeWords[t] }
