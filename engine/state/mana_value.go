package state

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// ManaValue is the CR 202.3 mana value of o, the ONE object-level mana-value
// read every engine tier shares (effects' filters, Count$ heads, cascade,
// discover, the budgeted pickers, rules' ward/evidence/protection/setprops
// reads and the sacrifice LKI). A face's own printed value (cards.Face.Cmc)
// is a different, face-level fact; this function answers "what is this
// OBJECT's mana value right now", which differs from its current face's
// printed cost in exactly these cases, checked in this order:
//
//   - CR 708.2a/708.4: a face-down permanent or face-down spell has no mana
//     cost, so its mana value is 0 (FaceDownHidden).
//   - CR 707.2/202.3b/202.3c: a copy reads its copied face (CopyFace) as
//     printed -- the copy of a transformed back face or of a melded
//     permanent prints "no cost" and is 0, exactly as the rules say.
//   - CR 709.5: a battlefield Room is the sum of its unlocked doors.
//   - CR 712.8g/202.3c: a melded permanent is the sum of its two cards'
//     FRONT faces (the partner is g.Obj(o.MeldedWith); a nil g or a
//     missing partner contributes 0).
//   - CR 712.8c/712.8e/202.3b: a transforming DFC with its back face up
//     (FaceIdx != 0, on the battlefield or cast transformed) uses its front
//     face's mana cost.
//   - otherwise the current face's printed value.
//
// CR 202.3e: on the stack each {X} in the counted cost adds the announced
// X (Object.X); everywhere else X is 0. CR 202.3f is the face-level parse
// (cards.Face.ManaValue counts {2/W} as 2).
//
// g may be nil (a census or direct call): only the meld partner needs it.
func ManaValue(g *Game, o *Object) int32 {
	if o == nil {
		return 0
	}
	return manaValueAs(g, o, o.Zone, o.X)
}

// manaValueAs is ManaValue for o read as if it were in zone with announced
// X.
func manaValueAs(g *Game, o *Object, zone Zone, x int32) int32 {
	f := o.Face()
	if f == nil || o.faceDownIn(zone) {
		return 0
	}
	if o.CopyFace != nil {
		return faceManaValue(f) + stackX(zone, x, f)
	}
	if mv, ok := roomManaValue(o); ok {
		return mv
	}
	c := o.Card
	if o.MeldedWith != 0 && c != nil && len(c.Faces) != 0 && c.Faces[0] != nil {
		n := faceManaValue(c.Faces[0])
		if g != nil {
			if p := g.Obj(o.MeldedWith); p != nil && p.Card != nil && len(p.Card.Faces) != 0 && p.Card.Faces[0] != nil {
				n += faceManaValue(p.Card.Faces[0])
			}
		}
		return n
	}
	if c != nil && c.IsTransformingDFC() && o.FaceIdx != 0 && c.Faces[0] != nil {
		front := c.Faces[0]
		return faceManaValue(front) + stackX(zone, x, front)
	}
	return faceManaValue(f) + stackX(zone, x, f)
}

// faceManaValue is a face's printed mana value: the load-time value while
// the ManaCost is the text it was derived from, else the current text priced
// by the same parser (a face copy with a rewritten ManaCost).
func faceManaValue(f *cards.Face) int32 {
	if mv, ok := f.PrintedManaValue(); ok {
		return mv
	}
	return f.ManaValue()
}

// stackX is CR 202.3e's on-stack term: the announced x once per {X} symbol
// in f's mana cost while the object is on the stack, 0 elsewhere. A spell
// without an {X} adds nothing, whatever Object.X holds.
func stackX(zone Zone, x int32, f *cards.Face) int32 {
	if zone != ZStack || x == 0 {
		return 0
	}
	var n int32
	for sym := range strings.FieldsFuncSeq(f.ManaCost, isCostSeparator) {
		if sym == "X" {
			n++
		}
	}
	return n * x
}

func isCostSeparator(r rune) bool { return r == ' ' || r == '{' || r == '}' }

// roomManaValue is the CR 709.5 mana value of a battlefield Room permanent
// (BattlefieldRoomFaces): the sum of its unlocked doors' printed values. ok
// is false for every other object.
func roomManaValue(o *Object) (int32, bool) {
	faces, ok := o.BattlefieldRoomFaces()
	if !ok {
		return 0, false
	}
	var n int32
	for fi, f := range faces {
		if o.DoorUnlocked(fi) {
			n += faceManaValue(f)
		}
	}
	return n, true
}

// BattlefieldRoomFaces returns o's two printed faces when o is a
// battlefield, non-face-down permanent whose two faces are both Rooms
// (cards.Face.IsRoom). The CR 709.5 rule is a pair of static abilities of
// the permanent, so it functions only on the battlefield; a Room card in any
// other zone is an ordinary split card (CR 709.4).
func (o *Object) BattlefieldRoomFaces() ([]*cards.Face, bool) {
	if o == nil || o.Card == nil || len(o.Card.Faces) != 2 || o.Zone != ZBattlefield || o.FaceDown {
		return nil, false
	}
	faces := o.Card.Faces
	if faces[0] == nil || faces[1] == nil || !faces[0].IsRoom() || !faces[1].IsRoom() {
		return nil, false
	}
	return faces, true
}

// FaceDownHidden reports whether o is face down where its face-down
// characteristics apply (CR 708.2a/708.4): a face-down permanent on the
// battlefield or a face-down spell on the stack. Such an object has no name,
// no mana cost and no colour; a FaceDown marker stranded on a card in any
// other zone hides nothing.
func (o *Object) FaceDownHidden() bool {
	return o != nil && o.faceDownIn(o.Zone)
}

func (o *Object) faceDownIn(zone Zone) bool {
	return o.FaceDown && (zone == ZBattlefield || zone == ZStack)
}
