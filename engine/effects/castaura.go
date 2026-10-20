package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// AttachAILogic is the face's AttachAILogic SVar -- the script author's hint
// at whom an Aura or Equipment is for (Pump, Curse, ...). Nil-safe; empty
// when the face declares none. It lives here rather than in engine/cards
// because that package is under the strings ratchet.
func AttachAILogic(f *cards.Face) string {
	if f == nil {
		return ""
	}
	return f.SVars["AttachAILogic"]
}

// CastAuraLogic is the AttachAILogic hint of a face whose spell is an Aura
// aimed at a creature: SP$ Attach with a ValidTgts starting "Creature".
// Empty for a nil face, a face without the SVar, and every other spell (an
// enchant-land or enchant-artifact Aura, a non-Attach spell), so a caller
// that reads it never holds a card for lack of a creature it could not have
// enchanted anyway.
func CastAuraLogic(f *cards.Face) string {
	logic := AttachAILogic(f)
	if logic == "" {
		return ""
	}
	sa := f.SpellAbility()
	if sa == nil || sa.API != "Attach" || !strings.HasPrefix(sa.Params["ValidTgts"], "Creature") {
		return ""
	}
	return logic
}
