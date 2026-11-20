package cards

import (
	"regexp"
	"strings"
)

// craftMaterialCost matches the material-cost body of a `K:Craft:` line this
// build implements: a run of mana tokens (generic, coloured, hybrid, or the
// XMin<N> announced-X floor) followed by one or more material parts. The two
// material verbs are ExileCtrlOrGrave (exile from among other permanents you
// control and/or cards in your graveyard) and ExileFromGrave (graveyard-only:
// Ore-Rich Stalactite) -- both are existing exile cost-part verbs the payment
// machinery already prices (engine/rules/pay/unless.go's announced
// ExileFromGrave<X/Spec>, exAsk's per-zone candidate walk), so admitting the
// head only widens the grammar to a part the cast path knows.
//
// The spec group is deliberately broad: the per-candidate filter grammar is
// owned by effects' matcher, which fails closed on a term it does not know, so
// this classifier only has to reject the shapes whose SEMANTICS the chooser
// cannot express (the group predicates below).
var craftMaterialCost = regexp.MustCompile(`^\s*(?:(?:\d+|[WUBRGCX]|[WUBRGC]/[WUBRGC]|XMin\d+)\s+)*(?:(?:ExileCtrlOrGrave|ExileFromGrave)<(?:X|\d+)/[^><]+>\s*)+$`)

// craftGroupPredicates are Forge spec terms that constrain the paid SET rather
// than each candidate on its own. effects' per-object matcher cannot evaluate
// them (it never sees the whole set during a material pick), so every term
// fails closed and the Craft activation would be permanently unoffered rather
// than mis-modelled. They stay behind api:Craft.OtherShape and keep the card
// named in the shape census.
//
//   - withSharedCardType: Eye of Ojer Taq, "two permanents that share a card
//     type" -- the N picked objects must share one type, a set relation.
//   - withTotalPowerGE<N>: the set-level power floor the ordinary tapXType
//     costs carry (Crew, Mossbridge Troll); no Craft carrier today, listed so
//     the grammar cannot silently accept a future one.
var craftGroupPredicates = [...]string{"withSharedCardType", "withTotalPowerGE"}

// craftMaterialBody returns the material-cost body of a Craft keyword
// parameter, dropping Forge's trailing ":<description>[:<label>]" display
// fields. Throne of the Grim Captain spells its parameter
// "4 ExileCtrlOrGrave<...>...:a Dinosaur, a Merfolk, a Pirate, and a Vampire:the four";
// the text after the last material group is display prose, never cost. The
// material grammar itself contains no ':', so the first one ends the cost.
func craftMaterialBody(param string) string {
	if i := strings.IndexByte(param, ':'); i >= 0 {
		return strings.TrimSpace(param[:i])
	}
	return strings.TrimSpace(param)
}

// CraftShapeSupported reports whether a Craft material parameter is one this
// build's expander and material chooser implement: a single fixed-count
// carrier (ExileCtrlOrGrave<1/Artifact.Other>), the announced-X form with an
// XMin floor (ExileCtrlOrGrave<X/...> beside XMin<N>, The Enigma Jewel and
// the X-minimum family), the graveyard-only head ExileFromGrave in either
// count form (Ore-Rich Stalactite), or several ExileCtrlOrGrave slots each
// filled by a distinct card (Throne of the Grim Captain). Keeping this
// classifier beside the expander makes the primitive census fail closed for
// every other Craft grammar -- the expander installs no ability for a shape
// it returns false for, so an exotic carrier can never be marked supported by
// accident.
func CraftShapeSupported(param string) bool {
	body := craftMaterialBody(param)
	if !craftMaterialCost.MatchString(body) {
		return false
	}
	for _, pred := range craftGroupPredicates {
		if strings.Contains(body, pred) {
			return false
		}
	}
	return true
}

func keywordParam(line string) string {
	if i := strings.IndexByte(line, ':'); i >= 0 {
		return strings.TrimSpace(line[i+1:])
	}
	return ""
}

// kwCraft installs the CR 702.167a ability. The card returns to the
// battlefield TRANSFORMED (ChangeZone Transformed$ True, CR 711.10a), so it
// never enters front face up: a front-face ETB replacement or trigger (Braided
// Net's net counters, Tithing Blade's sacrifice) must not see the entry.
func kwCraft(f *Face, _ int, k, _, param string, has func(kind, line string) bool) {
	if has("A", k) || !CraftShapeSupported(param) {
		return
	}
	// A material part's ";" multi-spec (every corpus spelling of it is a cost
	// part: "ExileFromGrave<1/Artifact;Creature/artifact or creature card>",
	// Ore-Rich Stalactite's red instant;sorcery pair) is Forge's or-list; the
	// parser normalises it to the matcher's ',' at load time, so the part the
	// cost parser binds and every reader of part.Spec evaluate the printed
	// or-list with the matcher's own separator.
	body := normalizeCraftMaterialBody(craftMaterialBody(param))
	cost := body + " Exile<1/CARDNAME>"
	sa, _ := parseSA("", "AB$ ChangeZone | Cost$ "+cost+
		" | ActivationZone$ Battlefield | SorcerySpeed$ True | Defined$ Self | Origin$ Exile | Destination$ Battlefield | Transformed$ True | Keyword$ Craft | SpellDescription$ Craft -- pay "+body+": return this transformed")
	if sa == nil {
		return
	}
	sa.Params["KeywordLine"] = k
	f.Abilities = append(f.Abilities, sa)
}

func init() { registerKeyword(kwCraft, "Craft") }
