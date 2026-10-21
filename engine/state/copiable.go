package state

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
)

// CopiableFace is the face a copy of o copies (CR 707.2): o's copiable
// values. While o is a face-down battlefield permanent it is the face-down
// characteristic set (CR 708.2), never the hidden card; otherwise it is
// o.Face(), which for a permanent that is itself copying something is
// already its layer-1 copy basis (CopyFace, CR 707.3) and else its printed
// face. Every events.Apply fold that snapshots another object's
// characteristics for a copy (ClonePermanent, CopyToken, MyriadCopy) reads
// it, so the copy routes cannot disagree about what a copy is.
func (o *Object) CopiableFace() *cards.Face {
	if o.faceDownEffective() {
		return o.faceDownCopiableFace()
	}
	// CR 707.2: text-changing effects are not copiable values.
	return o.PreTextFace()
}

// CopiesFaceDown reports whether CopiableFace is o's face-down
// characteristic set (o is face down on the battlefield): a copy of o must
// not carry o's hidden Card.
func (o *Object) CopiesFaceDown() bool { return o.faceDownEffective() }

// faceDownCopiableFace builds CR 708.2a's copiable values of a face-down
// battlefield permanent from its folded face-down fields -- the facts
// engine/rules/chars reads for its layer walk: no name, no mana cost, no
// colour, no rules text, the FaceDownSetType$ type set (default {Creature}),
// the FaceDownPower$/FaceDownToughness$ pair when one was folded, else 2/2
// for a creature and no P/T otherwise, and ward {2} for a cloaked permanent
// (CR 701.58a). A fresh face per call, shared with nothing. Only SetPT's
// parse is bound; every other derived reader takes its unguarded slow path,
// so a replay builds a FaceEqual twin.
func (o *Object) faceDownCopiableFace() *cards.Face {
	f := &cards.Face{Types: o.FaceDownTypeWords(), SVars: map[string]string{}}
	switch {
	case o.FaceDownHasPT:
		f.SetPT(strconv.Itoa(int(o.FaceDownPower)) + "/" + strconv.Itoa(int(o.FaceDownToughness)))
	case o.EffectiveIsCreature():
		f.SetPT("2/2")
	}
	if o.Cloaked {
		f.Keywords = []string{"Ward:2"}
	}
	return f
}
