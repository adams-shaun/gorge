package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// battlesAccepted gates the Battle (CR 310) lift. Part B landed the defeat
// ask and the free transformed cast, so every shape below is accepted.
// (A var only so TestBattleRefusals can close the gate; nothing else
// writes it.)
var battlesAccepted = true

func faceIsBattle(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for _, t := range f.Types {
		if t == "Battle" {
			return true
		}
	}
	return false
}

// refuseBattle refuses a card with a Battle face the kernel does not
// read exactly and reports whether it did. Accepted: a 2-face
// DoubleFaced card whose front is a Battle (Siege) with a plain numeric
// Defense 1..127 and a non-Battle back, with no enters replacement (the
// protector shares perm.chosen), not a token.
func (lc *lowerCtx) refuseBattle(c *cards.Card) bool {
	any := false
	for _, f := range c.Faces {
		any = any || faceIsBattle(f)
	}
	if !any {
		return false
	}
	if !battlesAccepted {
		lc.refuseNode("card", "battle (CR 310)")
		return true
	}
	if lc.out.Token {
		lc.refuseNode("card", "battle token or copy")
		return true
	}
	if c.AlternateMode != cards.AlternateDoubleFaced || len(c.Faces) != 2 || c.Faces[0] == nil || c.Faces[1] == nil || !faceIsBattle(c.Faces[0]) {
		lc.refuseNode("card", "battle layout %q", c.AlternateMode)
		return true
	}
	if faceIsBattle(c.Faces[1]) {
		lc.refuseNode("card", "battle back face is a battle")
		return true
	}
	f := c.Faces[0]
	if n, err := strconv.Atoi(strings.TrimSpace(f.Defense)); err != nil || n < 1 || n > 127 {
		lc.refuseNode("card", "battle defense %q", f.Defense)
		return true
	}
	if len(f.Repls) > 0 {
		lc.refuseNode("card", "battle with an enters replacement or as-enters choice")
		return true
	}
	return false
}
