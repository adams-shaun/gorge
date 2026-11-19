package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugRmCreatSubKeep is the mutation switch of TestRmcreatsubMutation: the
// RemoveCreatureTypes strip of an override (kop.Override.RemoveCreatureSub)
// is skipped, the add-only reading.
var DebugRmCreatSubKeep bool

// ovrCreatSub reports whether override o drops the creature subtypes and the
// all-creature-types mark: its RemoveCreatureSub flag (RemoveCreatureTypes$
// without RemoveCardTypes$), or CR 205.1a's follow-through, the subtypes
// correlated with the card types its RemoveTypes removes -- a static's
// RemoveType$ Creature takes the God word with the Creature card type (the
// devotion gods' isn't-a-creature; gorge chars/types.go's RemoveTypes
// block). Every reader of the two fields goes through this one helper, as
// ovrAllCreat's readers do.
func ovrCreatSub(o *kop.Override) bool {
	return o.RemoveCreatureSub || o.RemoveTypes&kop.TCreature != 0
}
