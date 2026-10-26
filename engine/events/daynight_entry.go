package events

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

var kwrDaybound = cards.NewKeywordRef("Daybound")

// entersTransformedAtNight is CR 702.145b's first ability: a daybound
// permanent represented by a transforming double-faced card that enters the
// battlefield while it is night enters with its back face up. It reads the
// PRINTED front face (the object is entering, and no corpus line grants
// daybound). A face already on its back (ChangeZone Transformed$ True) stays
// single, and a face-down or non-DFC (token copy) object is left alone.
func entersTransformedAtNight(o *state.Object) bool {
	if o.Card == nil || o.FaceDown || o.FaceIdx != 0 || !o.Card.IsTransformingDFC() {
		return false
	}
	return o.Card.Faces[0] != nil && o.Card.Faces[1] != nil && o.Card.Faces[0].HasKeywordRef(kwrDaybound)
}
