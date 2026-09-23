package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// ChooseType, ChooseNumber and ChooseColor record a choice on the source.
// With the choice already present (the cast-time "as this enters" ask in
// rules/cast.go's etbAsk recorded it with a Choose event before this ever
// resolves, plan ruling R-6) these do nothing. Without one -- a script that
// uses them at RESOLUTION time -- ChooseType poses a real KChoose ask over
// its Type$ CATEGORY's option list (task ct1; effects/type_choices.go is the
// one home for the non-creature lists, and the suspension re-enters through
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

// effChooseType records a type choice. With the source already carrying a
// ChosenType it is a no-op (the cast-time ask pre-recorded it); on the
// re-entry after its own ask was answered it emits exactly the Choose event
// the fallback emits, with the answered type (Ctx.ChosenType, consumed and
// cleared -- fx42). On the first pass it poses a real KChoose ask over the
// option list its Type$ CATEGORY ranges over when two or more options are
// offerable, so the chooser picks; with zero or one offerable option the
// choice is forced (or empty) and the single legal answer equals the
// fallback's deterministic pick, so no ask is posed (the effDiscard
// strict-supersets convention). A host that cannot ask falls through to the
// same fallback with no extra Note (R-9).
//
// Type$ (Herald's Horn's Creature, Realmwright's Basic Land, Archon of
// Valor's Reach's Card, Deification's Planeswalker, Apex Observatory's
// Shared, Aswan Jaguar's CreatureInTargetedDeck) names the CATEGORY the
// choice ranges over. Every category this build can enumerate now offers its
// REAL list -- effects/type_choices.go is the one home -- so the resolution
// ask and the as-enters ask (rules/cast.go's etbOptions) cannot disagree. Only
// a category this build still cannot name keeps the loud Note plus a
// deterministic fallback, and that fallback is drawn from the category's own
// list when one exists (never a nonsensical creature type).
func effChooseType(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	if o := g.Obj(c.Source); o != nil && o.ChosenType != "" {
		return
	}
	cat := strings.TrimSpace(sa.Params["Type"])
	if answered := c.ChosenType; answered != "" {
		// The "choosetype" resume arm's answer: emit the same Choose event the
		// fallback emits, with the answered type, so events.Apply records
		// o.ChosenType exactly the way every downstream reader already reads.
		c.ChosenType = ""
		h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "type", Text: answered})
		return
	}
	chooser := c.Controller
	if ts := Defined(h, c, sa); len(ts) > 0 && ts[0].IsPlayer {
		chooser = ts[0].Player
	}
	labels, known := chooseTypeLabels(h, c, sa, chooser, cat)
	if !known {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ChooseType Type$ " + cat + " is not a category this engine can ask; the choice falls back to creature types"})
	}
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 1, Max: 1,
		ResumeKind: "choosetype", ResumeSA: sa, Prompt: chooseTypePrompt(cat), Source: c.Source}
	for _, label := range labels {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "type", Label: label})
	}
	if len(d.Options) > 1 && Ask(h, d) == AskAsked {
		return
	}
	// The no-ask fallback. A category with an option list records that list's
	// deterministic first entry; a category whose list is empty (an
	// unresolvable Shared or CreatureInTargetedDeck context, or an unknown
	// category) keeps the historical creature-type scan, so no category ever
	// records a nonsense value from ANOTHER category.
	var fallback string
	if len(d.Options) > 0 && !isCreatureCategory(cat) {
		fallback = d.Options[0].Label
	}
	if fallback == "" {
		fallback = creatureTypeFallback(g, c.Controller)
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: c.Source, Counter: "type", Text: fallback})
}

// chooseTypeLabels returns the option labels for the resolving ChooseType's
// Type$ category, and whether the category is one this build can enumerate.
// Creature (and an absent Type$) is the owner-scoped list the engine already
// built; the context-scoped Shared and CreatureInTargetedDeck categories read
// the resolving effect's own state; every other enumerable category reads the
// static list effects/type_choices.go defines. A category with no list (and
// not the four above) reports known=false, which is the loud-Note path.
func chooseTypeLabels(h Host, c *Ctx, sa *cards.SA, chooser state.PlayerID, cat string) ([]string, bool) {
	if isCreatureCategory(cat) {
		return optionLabels(h.TypeChoices(chooser, cat)), true
	}
	switch strings.ToLower(cat) {
	case "shared":
		return SharedTypeLabels(h.Game(), c.Source), true
	case "creatureintargeteddeck":
		return CreatureInTargetedDeckLabels(h.Game(), c.Targets), true
	}
	if labels := TypeChoiceLabels(cat, sa.Params["ValidTypes"], sa.Params["InvalidTypes"]); labels != nil {
		return labels, true
	}
	return nil, false
}

// optionLabels reads the labels off an option list (the Host.TypeChoices
// creature list), preserving order.
func optionLabels(opts []decision.Option) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, o.Label)
	}
	return out
}

// isCreatureCategory reports whether a Type$ value is an absent category or
// "Creature" -- the two spellings that ask over the creature-type list.
func isCreatureCategory(cat string) bool {
	return cat == "" || strings.EqualFold(cat, "Creature")
}

// chooseTypePrompt names the category for a client prompt.
func chooseTypePrompt(cat string) string {
	switch strings.ToLower(cat) {
	case "", "creature", "creatureintargeteddeck":
		return "Choose a creature type"
	case "basic land", "land", "nonbasic land":
		return "Choose a land type"
	case "card", "shared":
		return "Choose a card type"
	case "planeswalker":
		return "Choose a planeswalker type"
	}
	return "Choose a type"
}

// creatureTypeFallback is the historical deterministic creature-type
// fallback: the first creature subtype of a creature the controller controls,
// in object order, or "Human" when they control none. It is reached only when
// a category has no option list of its own, so a non-creature category is
// never recorded as a creature type unless its own context was unreadable.
func creatureTypeFallback(g *state.Game, controller state.PlayerID) string {
	fallback := ""
	for i := range g.Objs {
		o := &g.Objs[i]
		if fallback != "" || o.Controller != controller {
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
	return fallback
}

// CreatureTypeWords reports whether a Type token is a creature subtype. It
// shares the positive vocabulary Changeling uses, so a cast-time type choice
// cannot offer a spell, plane, or planeswalker subtype as a creature type.
func CreatureTypeWords(t string) bool { return creatureSubtypeWords[t] }
