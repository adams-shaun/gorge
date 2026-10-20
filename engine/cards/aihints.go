package cards

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
// Face.SVars. The parser canonicalises the value once at the script
// boundary (canonicalPlayMain1), so this reader is a lookup, not another
// spelling table: Forge's castPermanentInMain1 treats every value except
// ALWAYS and OPPONENTCREATURES (and FALSE/empty) as the own-creature
// condition, so Random/True and any future spelling canonicalise to TRUE.
func (f *Face) PlayMain1Hint() string {
	if f == nil {
		return ""
	}
	switch f.SVars["PlayMain1"] {
	case PlayMain1Always:
		return PlayMain1Always
	case PlayMain1OpponentCreatures:
		return PlayMain1OpponentCreatures
	case "", "FALSE":
		return ""
	default:
		return PlayMain1OwnCreatures
	}
}
