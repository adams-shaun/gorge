package rules

import (
	"iter"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// blockWalkFaces yields the faces a combat-declaration trigger walk
// (trigger_blocks.go) reads printed triggers from: the top face f, then each
// card beneath a mutated pile's top (CR 702.140e: the permanent has all
// abilities of each card). A merged face carries its pile index and the
// faceIdx triggerFacesWithMerged gives it, so its fire-count key never shares
// an entry with the top face's same-index line and its instance is pushed as
// the under card's own trigger (MergedTriggerPush). An unmutated object
// yields only f, keyed exactly as before.
func blockWalkFaces(o *state.Object, f *cards.Face) iter.Seq[triggerFace] {
	return func(yield func(triggerFace) bool) {
		if !yield(triggerFace{face: f, active: true}) {
			return
		}
		for i := range o.MergedCards {
			if 2+i > 255 {
				return
			}
			if uf := o.MergedFaceAt(i); uf != nil {
				if !yield(triggerFace{face: uf, faceIdx: uint8(2 + i), merged: i + 1}) {
					return
				}
			}
		}
	}
}

// pileHasFlankingMarker reports whether any face of o's pile prints a
// kw:Flanking marker: that printed line then owns every derived flanking
// instance (queueAttackerBlockedTrigger), so the granted path defers.
func pileHasFlankingMarker(o *state.Object, f *cards.Face) bool {
	for fc := range blockWalkFaces(o, f) {
		for _, t := range fc.face.Triggers {
			if isFlankingMarker(t) {
				return true
			}
		}
	}
	return false
}

// blockTriggerKey is the fire-count key of face fc's line idx on source.
func blockTriggerKey(source state.ObjID, idx int, fc triggerFace) triggerKey {
	return triggerKey{Source: source, Idx: idx, Face: fc.faceIdx}
}

// blockMergedExecute is the Execute$ name a merged face's instance carries
// (the main walk's MergedTriggerPush shape); a top-face line keeps none.
func blockMergedExecute(t cards.Trigger, fc triggerFace) string {
	if fc.merged == 0 {
		return ""
	}
	return t.ParamStr(cards.PKExecute)
}
