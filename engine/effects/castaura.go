package effects

import "github.com/adams-shaun/gorge/engine/cards"

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
