package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// api_learn.go lowers api Learn (CR 701.48a; gorge engine/effects/learn.go
// effLearn): the resolving controller may reveal a Lesson card they own from
// outside the game (their sideboard) and put it into their hand, or discard a
// card and then draw a card, or do neither. gorge poses one KChoose ("nothing"
// first, then one "lesson" per Lesson card of the sideboard in zone order,
// then "rummage" when the hand holds a card) and, for "rummage", a second
// KChoose over the hand ("discard", hand order, asked even with one card). The
// kernel's half is dirac/act_learn.go (ExtLearn, the ChLearn election and the
// discard pick); the Lesson filter rides the instruction's F.
//
// Only Condition*$ (the node's generic condition), the descriptive
// SpellDescription$/StackDescription$, an activated ability's Cost$ and the
// SubAbility$ chain link may ride a Learn node. Remembered anywhere on the
// ability refuses: neither engine's learn touches the remembered set, but a
// reader elsewhere could be fed by a move this lowering does not model.
func init() {
	apiOp("Learn", (*lowerCtx).learn)
	instrOp("Learn", (*instrCtx).opLearn)
}

// learnLessonSpec is gorge's learnLessonSpec: the Lesson cards the learning
// player owns.
const learnLessonSpec = "Card.Lesson+YouOwn"

func (lc *lowerCtx) learn(sa *cards.SA, ps *params) error {
	for _, k := range sortedParamKeys(ps.m) {
		switch {
		case strings.HasPrefix(k, "Condition"):
		case k == "StackDescription", k == "SpellDescription", k == "SubAbility":
		case k == "Cost": // AB$ Learn (Poet's Quill): the ability builder pays it
		default:
			if !ignoredParams[k] {
				return refuse("Learn param %q", k)
			}
		}
	}
	if lc.saReadsRemembered(sa) {
		return refuse("Learn beside a Remembered read")
	}
	f, err := lc.filter(learnLessonSpec)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Learn", F: f, Note: "learn: a Lesson from the sideboard, a rummage, or nothing (CR 701.48a; dirac/act_learn.go)"})
	return nil
}

// opLearn is OpExt ExtLearn, F the Lesson filter read over the sideboard.
func (ic *instrCtx) opLearn() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtLearn))
	return ic.objFilter(kop.ZoneNone)
}
