package cards

import "strings"

// The AI-hint vocabulary. These are deck-builder hints in Forge card
// scripts, not rules text: the engine never enforces them, and only the
// opt-in botpolicy AIHints cast rules read them.
const (
	// PlayMain1Always is Forge's `SVar:PlayMain1:ALWAYS`: cast in the first
	// main phase unconditionally.
	PlayMain1Always = "ALWAYS"
	// PlayMain1OwnCreatures is Forge's `TRUE` (and any value other than the
	// two named exceptions): cast in the first main phase only while the
	// caster controls a creature.
	PlayMain1OwnCreatures = "TRUE"
	// PlayMain1OpponentCreatures is Forge's `OPPONENTCREATURES`: cast in the
	// first main phase only while an opponent controls a creature.
	PlayMain1OpponentCreatures = "OPPONENTCREATURES"
)

// PlayMain1Hint returns the canonical PlayMain1 mode for this face, or "".
// It reads the existing SVar map rather than adding a parallel parse path:
// PlayMain1 is an SVar, and the spellbench IR reader already reads it from
// Face.SVars. Forge's castPermanentInMain1 treats every value except ALWAYS
// and OPPONENTCREATURES (and FALSE/empty) as the own-creature condition, so
// Random/True and any future spelling canonicalise to TRUE.
func (f *Face) PlayMain1Hint() string {
	if f == nil {
		return ""
	}
	v := strings.TrimSpace(f.SVars["PlayMain1"])
	switch {
	case strings.EqualFold(v, PlayMain1Always):
		return PlayMain1Always
	case strings.EqualFold(v, PlayMain1OpponentCreatures):
		return PlayMain1OpponentCreatures
	case v == "" || strings.EqualFold(v, "FALSE"):
		return ""
	default:
		return PlayMain1OwnCreatures
	}
}

// canonicalRemoveDeck keeps the corpus spelling of the closed RemoveDeck
// set, case-normalising only the three values Forge names.
func canonicalRemoveDeck(v string) string {
	v = strings.TrimSpace(v)
	switch {
	case strings.EqualFold(v, "All"):
		return "All"
	case strings.EqualFold(v, "Random"):
		return "Random"
	case strings.EqualFold(v, "NonCommander"):
		return "NonCommander"
	default:
		return v
	}
}
