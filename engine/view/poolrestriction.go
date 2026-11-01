package view

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// This file renders a restricted-mana batch for the seat's pool readout. It
// deliberately does NOT re-implement the payment grammar: whether a payment
// may spend a batch is decided in engine/rules/ (restrictValidMatches), and engine/view/
// does not import rules/. What lives here is a light formatter for the
// Forge RestrictValid$ shapes the corpus carries (TestEveryCorpusRestrictionReadsAsPlainWords),
// with a plain generic sentence for a shape it does not recognise, so a script
// string never reaches a player.

// restrictionFallback states a restriction whose shape the formatter does not
// model.
const restrictionFallback = "spend only as its source allows"

// poolRestrictions projects a seat's restricted batches. nil (an absent JSON
// key) when the seat holds none, or when none of its batches carry a spend
// limit -- a batch with an empty Valid (an AddsNoCounter$-only batch,
// Boseiju) imposes no spend restriction and certifies nothing about why a
// cast was refused, so it is not annotated. The returned slice preserves the
// state's batch order (deterministic, never a map range).
func poolRestrictions(buf []PoolRestrictionView, g *state.Game, rs []state.ManaRestriction, memo *textMemo) []PoolRestrictionView {
	out := buf[:0] // the previous projection's storage (ProjectInto's reuse)
	for _, r := range rs {
		text := restrictionText(g, r.Source, r.Valid, memo)
		if text == "" {
			continue
		}
		out = append(out, PoolRestrictionView{Color: r.Color, Amount: r.Amount, Text: text})
	}
	if len(out) == 0 {
		return nil // no restricted batch: absent, as an append from nil leaves it
	}
	return out
}

// restrictionText is humanizeRestrictValid memoised in memo (when non-nil):
// the sentence is a pure function of the Valid$ string and the source's
// chosen type, the one game fact humanizeSpec reads.
func restrictionText(g *state.Game, source state.ObjID, valid string, memo *textMemo) string {
	if memo == nil {
		return humanizeRestrictValid(g, source, valid)
	}
	slot, out, ok := memo.lookup(memoRestrict, valid, sourceChosenType(g, source))
	if ok {
		return out
	}
	return slot.store(memoRestrict, valid, sourceChosenType(g, source), humanizeRestrictValid(g, source, valid))
}

// sourceChosenType is the restriction source's chosen type ("" when there is
// no such object), as humanizeSpec reads it.
func sourceChosenType(g *state.Game, source state.ObjID) string {
	if g != nil && source != 0 {
		if o := g.Obj(source); o != nil {
			return o.ChosenType
		}
	}
	return ""
}

// humanizeRestrictValid renders a batch's RestrictValid$ as a sentence, or ""
// when there is no restriction to state. A comma-separated Valid$ is OR
// semantics (engine/rules/stack.go's restrictValidMatches), so the alternatives are
// joined with " or ". If ANY alternative is a shape the formatter does not
// model, the whole string falls back to restrictionFallback: naming only some
// alternatives would read as the whole restriction.
func humanizeRestrictValid(g *state.Game, source state.ObjID, valid string) string {
	text, ok := restrictSentence(g, source, valid)
	if !ok {
		return restrictionFallback
	}
	return text
}

// restrictSentence is humanizeRestrictValid's sentence, with ok false for a
// shape it does not model.
func restrictSentence(g *state.Game, source state.ObjID, valid string) (string, bool) {
	valid = strings.TrimSpace(valid)
	switch valid {
	case "":
		return "", true
	case "CantCastNonArtifactSpells":
		return "can't be spent to cast nonartifact spells", true
	case "CantCastSpellFromHand":
		return "can't be spent to cast spells from your hand", true
	case "CantPayGenericCosts":
		return "can't be spent on generic costs", true
	}
	terms := strings.Split(valid, ",")
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		p, ok := humanizeRestrictTerm(g, source, strings.TrimSpace(term))
		if !ok {
			return "", false
		}
		parts = append(parts, p)
	}
	return "spend only to " + strings.Join(parts, " or "), true
}

// humanizeRestrictTerm humanises one comma-separated alternative. ok is false
// for a shape it does not model, so the caller can fall back honestly.
func humanizeRestrictTerm(g *state.Game, source state.ObjID, term string) (string, bool) {
	switch term {
	case "":
		return "", false
	case "Spell":
		return "cast a spell", true
	case "Activated", "nonSpell":
		return "activate an ability", true
	case "CumulativeUpkeep":
		return "pay cumulative upkeep", true
	case "CostContainsX":
		return "pay a cost with {X}", true
	case "CostContainsC":
		return "pay a cost with {C}", true
	case "Spell.Disturb":
		return "pay a disturb cost", true
	case "Activated.Equip":
		return "equip", true
	case "Activated.ClassLevelUp":
		return "level up a Class", true
	case "Activated.PowerUp":
		return "activate a power-up ability", true
	case "Static.Unlock":
		return "unlock a door", true
	case "Static.Foretelling":
		return "foretell a card", true
	case "Static.isTurnFaceUp":
		return "turn a permanent face up", true
	case "Static.isTurnFaceUp+Creature":
		return "turn a creature face up", true
	case "Static.ManifestUp+Creature":
		return "turn a manifested creature face up", true
	case "Static.MorphUp":
		return "pay a morph cost", true
	}
	kind, spec, dotted := strings.Cut(term, ".")
	if !dotted {
		return "", false
	}
	switch kind {
	case "Spell":
		phrase, ok := humanizeSpec(g, source, spec, "spell")
		if !ok {
			return "", false
		}
		return "cast " + phrase, true
	case "Activated":
		phrase, ok := humanizeSpec(g, source, spec, "ability")
		if !ok {
			return "", false
		}
		return "activate " + phrase, true
	default:
		return "", false
	}
}

// humanizeSpec turns a dotted term's spec into a noun phrase ("a creature
// spell", "a Demon creature spell", "an artifact ability"), resolving
// ChosenType against the producing permanent's recorded choice. ok is false
// for a qualifier it does not model.
func humanizeSpec(g *state.Game, source state.ObjID, spec, noun string) (string, bool) {
	chosen := sourceChosenType(g, source)
	var subtypes, words, after []string
	hasChosen := false
	for q := range strings.SplitSeq(spec, "+") {
		q = strings.TrimSpace(q)
		switch {
		case q == "" || q == "inZoneBattlefield":
			// A zone predicate on the ability's own source; it is not part
			// of the noun phrase.
		case q == "ChosenType":
			hasChosen = true
		case knownSpecWord(q):
			words = append(words, strings.ToLower(q))
		case specWord(q) != "":
			words = append(words, specWord(q))
		case specAfter(q) != "":
			after = append(after, specAfter(q))
		case subtypeWord(q):
			subtypes = append(subtypes, q)
		default:
			return "", false
		}
	}
	lead := append(subtypes, words...)
	if hasChosen {
		if chosen == "" {
			// The source has left the battlefield or recorded no type: name
			// the shape honestly rather than inventing one.
			if len(lead) == 0 {
				lead = []string{"creature"}
			}
			after = append([]string{"of the chosen type"}, after...)
		} else {
			lead = append([]string{chosen}, lead...)
		}
	}
	phrase := strings.Join(append(lead, noun), " ")
	if len(after) > 0 {
		phrase += " " + strings.Join(after, " ")
	}
	return article(phrase) + " " + phrase, true
}

// specWord is the modifier a script qualifier reads as before the noun, or "".
func specWord(q string) string {
	switch q {
	case "nonCreature":
		return "noncreature"
	case "nonColorless":
		return "colored"
	case "MultiColor":
		return "multicolored"
	case "MonoColor":
		return "monocolored"
	case "Permanent":
		return "permanent"
	case "Kicked":
		return "kicked"
	case "isCastFaceDown":
		return "face-down"
	case "IsCommander":
		return "commander"
	}
	return ""
}

// specAfter is the phrase a script qualifier reads as after the noun, or "".
func specAfter(q string) string {
	switch q {
	case "ChosenColor":
		return "of the chosen color"
	case "numColorsEQ3":
		return "with exactly three colors"
	case "NoAbilities":
		return "with no abilities"
	case "hasXCost":
		return "with {X} in its cost"
	case "!hasXCost":
		return "without {X} in its cost"
	case "wasCastFromYourGraveyard":
		return "from your graveyard"
	case "wasCastFromGraveyard":
		return "from a graveyard"
	case "wasCastFromExile":
		return "from exile"
	case "!wasCastFromYourHand":
		return "from anywhere but your hand"
	case "YouOwn":
		return "you own"
	case "YouDontOwn":
		return "you don't own"
	case "IsRemembered":
		return "exiled with its source"
	case "withDevoid":
		return "with devoid"
	case "withFlashback":
		return "with flashback"
	case "withForetell":
		return "with foretell"
	case "withFreerunning":
		return "with freerunning"
	}
	if v, ok := strings.CutPrefix(q, "cmcGE"); ok && v != "" {
		return "with mana value " + v + " or greater"
	}
	return ""
}

// subtypeWord reports whether a qualifier reads as a printed subtype: every
// word capitalised once ("Dragon", "Time Lord"). A script predicate is camel
// case ("IsRemembered"), so it fails closed.
func subtypeWord(q string) bool {
	for word := range strings.SplitSeq(q, " ") {
		if len(word) < 2 || word[0] < 'A' || word[0] > 'Z' {
			return false
		}
		for i := 1; i < len(word); i++ {
			if word[i] < 'a' || word[i] > 'z' {
				return false
			}
		}
	}
	return true
}

// knownSpecWord reports whether a spec qualifier is a card type, supertype or
// colour that reads naturally as a noun modifier.
func knownSpecWord(q string) bool {
	switch strings.ToLower(q) {
	case "artifact", "battle", "creature", "enchantment", "instant", "land",
		"planeswalker", "sorcery", "tribal", "kindred",
		"basic", "legendary", "snow", "world",
		"white", "blue", "black", "red", "green", "colorless":
		return true
	}
	return false
}

// article picks "a" or "an" for a noun phrase by its first word's initial.
func article(phrase string) string {
	if phrase == "" {
		return "a"
	}
	switch phrase[0] {
	case 'a', 'A', 'e', 'E', 'i', 'I', 'o', 'O', 'u', 'U':
		return "an"
	}
	return "a"
}
