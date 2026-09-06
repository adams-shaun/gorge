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
	if len(kw) < len(prefix) || !strings.EqualFold(kw[:len(prefix)], prefix) {
		return "", false
	}
	q := strings.TrimSpace(kw[len(prefix):])
	if q == "" {
		return "", false
	}
	return q, true
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
	if c := protecColourLetter(q); c != 0 {
		col := effects.ColorsOf(e.G.Obj(source))
		return col != "" && strings.ContainsRune(col, c)
	}
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
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

// Registered here: the five single-colour "Protection from" keywords, the
// exact shape Goblin Piledriver and Knight of Infamy (the last two ratchet
// entries) carry and the only protection keywords the M2r ratchet schedule
// files a registration for — and, honestly, the ONLY protection syntax
// protectionQuality parses. The corpus's dominant form is actually the
// K:Protection:<Spec> syntax (K:Protection:Creature, K:Protection:Instant:
// instants, K:Protection:Card.MultiColor, ... — 40+ distinct shapes across
// .cards/cardsfolder), none of which protectionQuality parses: it matches
// only "Protection from <colour>" plus the general words in sourceHasQuality's
// switch, which five do not cover what the corpus actually spells. The plural
// type words (artifacts/creatures/enchantments/instants/sorceries) the switch
// handles never appear in Forge keyword syntax at all — the corpus writes
// K:Protection:Creature, not "Protection from creatures" — and a
// K:Protection from each color parses to a quality matching nothing.
// Registering the type/everything protections would grow the "supported" set
// without a card test to prove it (Ruling W2), and protectedFrom does NOT in
// fact handle the corpus's other forms correctly: parsing and registering
// them is real future work, and until a card test retires a ratchet entry
// for one they will not be reported as supported.
func init() {
	effects.RegisterNonAPI("kw:Protection from white", "kw:Protection from blue",
		"kw:Protection from black", "kw:Protection from red", "kw:Protection from green")
}
