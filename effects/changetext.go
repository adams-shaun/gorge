package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// api:ChangeText (Forge's ChangeTextEffect) and its sibling api:ExchangeTextBox
// are the CR 612 text-changing family. A card's rules TEXT is what these
// effects change; the engine's one rendering of an object's current text is
// rules.Engine.Text (Derived.Text), which applies every registered layer-3
// TextSet/TextFrom effect in timestamp order. Neither primitive rewrites the
// compiled face: both register a state.ContinuousEffect whose text fields the
// layer walk folds in, so a changed text is replay-derived exactly like a
// SetName or a colour change, never a mutation of shared card data.
func init() {
	Register("ChangeText", effChangeText)
	Register("ExchangeTextBox", effExchangeTextBox)
}

// textChangeDuration resolves a ChangeText/ExchangeTextBox Duration$ into the
// ContinuousEffect timing flags. ChangeText's Forge default is Permanent
// ("This effect lasts indefinitely" on the corpus's reminders); an absent
// ExchangeTextBox duration is instead source-scoped (the exchanged boxes are
// the objects' own while both remain), which is the same lifetime
// `Duration$ AsLongAsInPlay` names.
func textChangeDuration(dur string, absentPermanent bool) (permanent, untilEOT bool) {
	switch strings.ToLower(strings.TrimSpace(dur)) {
	case "":
		return absentPermanent, false
	case "permanent":
		return true, false
	case "aslongasinplay", "aslongascontrolled", "untilendofcombat":
		return false, false
	case "untilendofyourturn":
		// UntilYourNextTurn/UntilTheEndOfYourNextTurn are handled by
		// AddContinuous's turn boundary; UntilEndOfYourTurn is the ordinary
		// end-of-turn cleanup.
		return false, true
	default:
		return false, true
	}
}

// textSubstitutionWords splits a ChangeColorWord$/ChangeTypeWord$ value into
// its [from, to] halves. ok is false when the value is not exactly two words,
// which the caller reports loudly rather than guessing a substitution.
func textSubstitutionWords(raw string) (from, to string, ok bool) {
	parts := strings.Fields(raw)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// isTextChooser reports whether a ChangeText word token names a chooser (the
// chooser picks the word) rather than a literal printed word. The corpus's
// chooser spellings are Choose (a colour word), ChooseCreatureType and
// ChooseBasicLandType.
func isTextChooser(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "choose", "choosecreaturetype", "choosebasiclandtype":
		return true
	}
	return false
}

// textChooserLabels returns the option labels a chooser token ranges over. The
// lists are the same ones the ChooseType/ChooseColor primitives offer (the
// owner-scoped creature list through Host.TypeChoices, the Basic Land list from
// effects/type_choices.go, the colour names from choose.go), so a ChangeText
// ask and a ChooseType ask can never disagree about a category's vocabulary.
// forbidden (ForbiddenNewTypes$) removes labels a card declares ineligible.
func textChooserLabels(h Host, chooser state.PlayerID, token, forbidden string) []string {
	var labels []string
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "choose":
		for _, cl := range chooseColorLabels {
			labels = append(labels, strings.ToLower(cl.name))
		}
	case "choosecreaturetype":
		for _, o := range h.TypeChoices(chooser, "Creature") {
			labels = append(labels, o.Label)
		}
	case "choosebasiclandtype":
		labels = append(labels, chooseBasicLandTypes...)
	default:
		return nil
	}
	if strings.TrimSpace(forbidden) == "" {
		return labels
	}
	forbiddenSet := map[string]bool{}
	for _, f := range strings.FieldsFunc(forbidden, func(r rune) bool { return r == ',' || r == ' ' }) {
		forbiddenSet[strings.ToLower(f)] = true
	}
	out := labels[:0]
	for _, l := range labels {
		if !forbiddenSet[strings.ToLower(l)] {
			out = append(out, l)
		}
	}
	return out
}

// effChangeText implements api:ChangeText: it registers a layer-3 substitution
// (TextFrom -> TextTo) on each affected object for the effect's duration. The
// substitution pair comes from ChangeColorWord$ or ChangeTypeWord$; either
// half may be a literal word (Vampire, Wall) or a chooser token, in which case
// the chooser is asked for it. The two halves are asked ONE AT A TIME (a
// re-entry after the first answer poses the second), each through the ordinary
// KChoose boundary, so the ask validates through the same machinery every
// other mid-resolution choice uses; Ctx.ChangeTextFrom/ChangeTextTo carry the
// answers across re-entry and are cleared once the pair is resolved (fx42).
func effChangeText(h Host, c *Ctx, sa *cards.SA) {
	raw := strings.TrimSpace(sa.Params["ChangeColorWord"])
	if raw == "" {
		raw = strings.TrimSpace(sa.Params["ChangeTypeWord"])
	}
	fromTok, toTok, ok := textSubstitutionWords(raw)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "api:ChangeText " + raw + " is not a two-word substitution this engine can apply; nothing changed"})
		return
	}
	chooser := c.Controller
	if ts := Defined(h, c, sa); len(ts) > 0 && ts[0].IsPlayer {
		chooser = ts[0].Player
	}
	forbidden := strings.TrimSpace(sa.Params["ForbiddenNewTypes"])

	from := c.ChangeTextFrom
	if !isTextChooser(fromTok) {
		from = fromTok
	}
	to := c.ChangeTextTo
	if !isTextChooser(toTok) {
		to = toTok
	}
	fromNeeds := isTextChooser(fromTok) && from == ""
	toNeeds := isTextChooser(toTok) && to == ""
	// One single combined ask for every half that still needs a word, so the
	// resolution suspends exactly once however many halves are chosen (the
	// two-half Choose Choose shape and the one-half ChooseCreatureType Vampire
	// shape both complete on one answer). A resumed answer is recognised by
	// either transport being non-empty, so a malformed answer that set only
	// one half falls back deterministically rather than re-asking forever.
	if (fromNeeds || toNeeds) && c.ChangeTextFrom == "" && c.ChangeTextTo == "" {
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose, ResumeKind: "changetext",
			ResumeSA: sa, Prompt: "Choose the text word(s)", Source: c.Source}
		if fromNeeds {
			labels := textChooserLabels(h, chooser, fromTok, "")
			for _, l := range labels {
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "changetext_from", Label: l})
			}
		}
		if toNeeds {
			labels := textChooserLabels(h, chooser, toTok, forbidden)
			for _, l := range labels {
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "changetext_to", Label: l})
			}
		}
		n := 0
		if fromNeeds {
			n++
		}
		if toNeeds {
			n++
		}
		d.Min, d.Max = n, n
		if Ask(h, d) == AskAsked {
			return
		}
	}
	if fromNeeds {
		from = deterministicTextWord(textChooserLabels(h, chooser, fromTok, ""))
	}
	if toNeeds {
		to = deterministicTextWord(textChooserLabels(h, chooser, toTok, forbidden))
	}
	// The pair is resolved: clear the transports so a nested ChangeText cannot
	// inherit this answer (fx42), then register.
	c.ChangeTextFrom, c.ChangeTextTo = "", ""

	if from == "" || to == "" {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "api:ChangeText " + raw + " resolved no substitution word; nothing changed"})
		return
	}
	if forbiddenAdmits(forbidden, to) {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "api:ChangeText refuses the forbidden new text \"" + to + "\"; nothing changed"})
		return
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		registerTextSubstitution(h, c, t.Obj, from, to, sa.Params["Duration"])
	}
}

// deterministicTextWord is the R-9 no-host / empty-list stand-in: the first
// offered word in the list's own deterministic order (the lists are fixed or
// sorted), so a no-host run still records a concrete substitution rather than
// inventing an off-list word.
func deterministicTextWord(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return labels[0]
}

// forbiddenAdmits reports whether ForbiddenNewTypes$ forbids word (a
// case-insensitive comma/space list). It is the fail-closed guard a literal
// replacement word passes through: an ask already filters the option list, but
// a literal `ChangeTypeWord$ ChooseCreatureType Wall` with
// `ForbiddenNewTypes$ Wall` must be refused too.
func forbiddenAdmits(forbidden, word string) bool {
	if strings.TrimSpace(forbidden) == "" || word == "" {
		return false
	}
	for _, f := range strings.FieldsFunc(forbidden, func(r rune) bool { return r == ',' || r == ' ' }) {
		if strings.EqualFold(f, word) {
			return true
		}
	}
	return false
}

// registerTextSubstitution registers one layer-3 from->to substitution on id.
// A Duration$ Permanent change ends when id leaves the battlefield (its
// ExileOnMoved$ + Remembered pair), so a blink returns the printed text; every
// other duration is honoured by continuousLive/EndOfTurnCleanup. Source is id
// itself with Affects Card.Self, the same shape Pump/Animate use to bind a
// continuous effect to a chosen object.
func registerTextSubstitution(h Host, c *Ctx, id state.ObjID, from, to, dur string) {
	permanent, untilEOT := textChangeDuration(dur, true)
	ce := state.ContinuousEffect{
		Source: id, Affects: "Card.Self", Controller: c.Controller,
		Layer: state.LText, TextFrom: from, TextTo: to,
		Duration: dur, Permanent: permanent, UntilEOT: untilEOT,
	}
	if permanent {
		if o := h.Game().Obj(id); o != nil {
			if w := ZoneWord(o.Zone); w != "" {
				ce.ExileOnMoved = w
				ce.Remembered = []state.ObjID{id}
			}
		}
	}
	h.AddContinuous(ce)
}

// effExchangeTextBox implements api:ExchangeTextBox: two objects swap their
// rules text for the effect's duration. Each object gets a layer-3 TextSet
// holding the OTHER's current derived text (Host.ObjectText, CR 613.1d), so
// rules.Engine.Text renders the exchange EXACTLY as the two boxes read at
// resolution -- including any earlier ChangeText substitution on either side.
// Source and Affects use the same per-object binding as ChangeText. A single
// object (or a player target) is a no-op with a loud Note: an exchange needs a
// pair.
func effExchangeTextBox(h Host, c *Ctx, sa *cards.SA) {
	var objs []state.ObjID
	for _, t := range Defined(h, c, sa) {
		if !t.IsPlayer && t.Obj != 0 && h.Game().Obj(t.Obj) != nil {
			objs = append(objs, t.Obj)
		}
	}
	if len(objs) != 2 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "api:ExchangeTextBox needs exactly two objects; nothing exchanged"})
		return
	}
	a, b := objs[0], objs[1]
	oa, ob := h.Game().Obj(a), h.Game().Obj(b)
	if oa == nil || oa.Face() == nil || ob == nil || ob.Face() == nil {
		return
	}
	// CR 612.1: the boxes exchanged are the objects' text AS THEY EXIST at
	// resolution, so each side's CURRENT derived text is captured (a prior
	// ChangeText substitution on either object is carried across), not the
	// printed Oracle. Reading both before registering either keeps the
	// capture independent of this effect's own registrations.
	textA, textB := h.ObjectText(oa), h.ObjectText(ob)
	registerTextSet(h, c, a, textB, sa.Params["Duration"])
	registerTextSet(h, c, b, textA, sa.Params["Duration"])
}

// registerTextSet registers one layer-3 TextSet (an outright text replacement)
// on id, with the same lifetime discipline as registerTextSubstitution.
func registerTextSet(h Host, c *Ctx, id state.ObjID, text, dur string) {
	permanent, untilEOT := textChangeDuration(dur, false)
	ce := state.ContinuousEffect{
		Source: id, DurationSource: c.Source, Affects: "Card.Self", Controller: c.Controller,
		Layer: state.LText, TextSet: text,
		Duration: dur, Permanent: permanent, UntilEOT: untilEOT,
	}
	if permanent {
		if o := h.Game().Obj(id); o != nil {
			if w := ZoneWord(o.Zone); w != "" {
				ce.ExileOnMoved = w
				ce.Remembered = []state.ObjID{id}
			}
		}
	}
	h.AddContinuous(ce)
}
