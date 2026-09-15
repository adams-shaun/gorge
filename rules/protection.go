package rules

// Protection (CR 702.16) — the third and last of the M2r ratchet's keyword
// families. Task 15 implements the classic protection cycle end to end:
// a protected permanent cannot be targeted, blocked, or dealt damage by a
// source it is protected from, and cannot be enchanted/equipped/fortified
// by one (CR 702.16b-g). The two cards that pinned the ratchet at the very
// end of the table were both protection-bearers — Goblin Piledriver
// (Protection from blue) and Knight of Infamy (Protection from white) — so
// this task retires them and empties knownUnsupported outright.
//
// The file carries the five registrations the coverage gate needs to call a
// "Protection from <colour>" bearer fully supported, the protectedFrom
// predicate the rest of the engine consults, and the emit-side hooks. The
// engine keeps the actual enforcement inside rules' emit (per config plan),
// NOT on effects.Host: Host stays tiny, the interface grows only toward
// effects' own needs (CastThisTurn and the layer host hooks), and the
// "which event does protection swallow" logic is engine rules, not an
// effect's business.

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// protectedFrom reports whether target is protected from source (CR 702.16):
// true exactly when some DERIVED keyword of target names a quality that
// source carries — "Protection from X itself has no effect on X's source
// being a spell or a permanent; it affects damage, enchantment/equipment
// and targeting". Quality is matched case-insensitively against:
//
//   - a colour word (white/blue/black/red/green) — source has it when its
//     colours (effects.ColorsOf, which applies Devoid) contain that colour;
//   - a permanent-type word (artifacts/creatures/enchantments) or a card-
//     type word (instants/sorceries) — source has it when its face carries
//     that type;
//   - everything — true against any source at all.
//
// Derived keywords include granted ones, so a permanent that gained
// "Protection from red" through a Resolution (e.g. a DB$ Protection
// resolution, or a continuous effect) protects as surely as a printed
// bearer does. target or source being a zero/nonexistent object is never
// protected (players have no ObjID, so a player target is never withheld on
// protection grounds).
func (e *Engine) protectedFrom(target, source state.ObjID) bool {
	if target == 0 || source == 0 {
		return false
	}
	for _, kw := range e.Keywords(target) {
		if q, ok := protectionQuality(kw); ok && e.sourceHasQuality(source, q) {
			return true
		}
	}
	return false
}

// protectionQuality turns one derived keyword into the single quality it
// protects against. "Protection from red and from blue" is emitted by Forge
// as TWO keywords joined with "&" — each resolves here to its own keyword
// ("Protection from red", "Protection from blue"), so parsing one quality
// per keyword after the "Protection from " prefix is exactly right and never
// faces the " and from " separator (that never survives inside a single
// keyword string; the parser separates keywords at "&" first, face.go).
// A keyword with no quality after the prefix ("Protection" alone, or a
// stray trailing token) yields nothing.
func protectionQuality(kw string) (string, bool) {
	const prefix = "Protection from "
	if len(kw) >= len(prefix) && strings.EqualFold(kw[:len(prefix)], prefix) {
		q := strings.TrimSpace(kw[len(prefix):])
		return q, q != ""
	}
	// Forge's general spelling is K:Protection:<Spec>:<display text>.
	// Keep only the spec; the following field is reminder text, not syntax.
	if len(kw) >= len("Protection:") && strings.EqualFold(kw[:len("Protection:")], "Protection:") {
		q, _, _ := strings.Cut(strings.TrimSpace(kw[len("Protection:"):]), ":")
		return q, q != ""
	}
	return "", false
}

// sourceHasQuality reports whether the object source carries the given
// protection quality. "everything" is short for "a source that has any
// characteristics"; everything else is matched against the source's colours
// or its face's types (never a player: source==0 already returned at the
// protectedFrom gate).
func (e *Engine) sourceHasQuality(source state.ObjID, q string) bool {
	if strings.EqualFold(q, "everything") {
		return true
	}
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	// A card object on the stack is a spell. An ability source is represented
	// by its source permanent for protection checks, so it is not a spell.
	if strings.EqualFold(q, "Spell") {
		return o.Zone == state.ZStack && o.Face() != nil
	}
	if strings.EqualFold(q, "Permanent.ThisTurnCast") {
		return o.Zone == state.ZBattlefield && o.EnteredThisTurn
	}
	// Forge's remaining printed parameterised K:Protection qualities include
	// mana-value bounds, counters and "coloured spells". They cannot be sent
	// through MatchesSpec unchanged: cmc/counters are value predicates and
	// nonColorless is a colour-identity predicate, none of which the generic
	// object filter can safely guess.
	if m := protectionCMC.FindStringSubmatch(q); m != nil {
		n, _ := strconv.Atoi(m[2])
		cmc := o.Face().Cmc()
		return (m[1] == "GE" && cmc >= int32(n)) || (m[1] == "LE" && cmc <= int32(n))
	}
	if m := protectionCounter.FindStringSubmatch(q); m != nil {
		n, _ := strconv.Atoi(m[1])
		return o.Zone == state.ZBattlefield && o.Counter(m[2]) >= int32(n)
	}
	if strings.EqualFold(q, "Spell.nonColorless") {
		return o.Zone == state.ZStack && o.Face() != nil && effects.ColorsOf(o) != ""
	}
	// MonoColor and EnemyColor are Forge's colour-class predicates, rather
	// than type predicates. They occur on Guardian/Frenemy of the Guildpact;
	// keep them here with the other source-quality tests so generic
	// kw:Protection registration covers every live K:Protection form.
	switch strings.ToLower(q) {
	case "card.monocolor":
		return isMonoColor(effects.ColorsOf(o))
	case "card.enemycolor":
		return hasEnemyColorPair(effects.ColorsOf(o))
	}
	// Parameterised protection qualities are Forge object specs (Artifact,
	// Creature.God, Card.MultiColor, and so on). Reuse the filter grammar so
	// every supported type/colour predicate has identical meaning here.
	if effects.MatchesSpec(e.G, q, source, e.controllerOf(source)) {
		return true
	}
	if c := protecColourLetter(q); c != 0 {
		col := effects.ColorsOf(e.G.Obj(source))
		return col != "" && strings.ContainsRune(col, c)
	}
	if o.Face() == nil {
		return false
	}
	f := o.Face()
	switch strings.ToLower(q) {
	case "artifacts":
		return f.IsArtifact()
	case "creatures":
		return f.IsCreature()
	case "enchantments":
		return f.IsEnchantment()
	case "instants":
		return f.IsInstant()
	case "sorceries":
		return f.IsSorcery()
	}
	return false
}

// isMonoColor reports the CR colour-class meaning: exactly one colour, not
// colourless. ColorsOf has already applied Devoid before this point.
var (
	protectionCMC     = regexp.MustCompile(`^Card\.cmc(GE|LE)([0-9]+)$`)
	protectionCounter = regexp.MustCompile(`^Permanent\.counters_GE([0-9]+)_([^_]+)$`)
)

func isMonoColor(colors string) bool { return len(colors) == 1 }

// hasEnemyColorPair reports whether a multicoloured object includes an enemy
// pair. The five enemy pairs are the non-adjacent pairs on the WUBRG colour
// wheel; a three- or five-colour object matches when it contains any one of
// them, as "enemy-colored multicolored" requires.
func hasEnemyColorPair(colors string) bool {
	for _, pair := range [...]string{"WB", "WR", "UR", "UG", "BG"} {
		if strings.ContainsRune(colors, rune(pair[0])) && strings.ContainsRune(colors, rune(pair[1])) {
			return true
		}
	}
	return false
}

// protecColourLetter maps a colour quality word to its WUBRG letter, 0
// when q is not a colour word.
func protecColourLetter(q string) rune {
	switch strings.ToLower(q) {
	case "white":
		return 'W'
	case "blue":
		return 'U'
	case "black":
		return 'B'
	case "red":
		return 'R'
	case "green":
		return 'G'
	}
	return 0
}

// kw:Protection covers Forge's parameterised K:Protection:<Spec> spelling,
// including every parameterised quality printed on a K:Protection line in
// the corpus. The older colour-specific registrations remain for Forge's
// separate natural-language "Protection from <colour>" keyword spelling.
func init() {
	effects.RegisterNonAPI("kw:Protection", "kw:Protection from white", "kw:Protection from blue",
		"kw:Protection from black", "kw:Protection from red", "kw:Protection from green")
}
