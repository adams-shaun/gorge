package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// recruit lowers `DB$ Recruit` (gorge effRecruit, engine/effects/recruit.go:
// draw a card, then discard a card, then create a 1/1 white Human Soldier if
// a nonland card was discarded). The corpus carries it as one SA with no
// parameters of its own, so kgen emits one cold OpExt (kop.ExtRecruit): the
// kernel draws, poses the discard pick exactly as gorge's KChoose does (the
// hand order, no ask when the hand holds no more than one), and creates the
// soldier only when the discarded card was a nonland.
//
// Composition from OpDraw/OpDiscard cannot match gorge's decision: the
// discard ask there is a DkChoose/ChDiscard pick and gorge's Recruit poses a
// decision.KChoose (the shared "discard" options) rather than a KModes
// discard, so the canonical kinds differ. The sub-op is the fallback; the
// oracle maps ChRecruitDiscard to the same choose:discard
// options gorge poses.
//
// only Condition*$ (the node's generic condition, read by node), the
// descriptive SpellDescription$/StackDescription$ and the SubAbility$ chain
// link may ride a Recruit node. Remembered anywhere on the face refuses: the
// kernel's discard is not a remembered-set effect, so a reader elsewhere on
// the face could disagree with gorge (which never touches the set).
func (lc *lowerCtx) recruit(sa *cards.SA, ps *params) error {
	for _, k := range sortedParamKeys(ps.m) {
		switch {
		case strings.HasPrefix(k, "Condition"):
		case k == "StackDescription", k == "SpellDescription", k == "SubAbility":
		default:
			return refuse("Recruit param %q", k)
		}
	}
	if lc.saReadsRemembered(sa) {
		return refuse("Recruit beside a Remembered read")
	}
	const script = "w_1_1_human_soldier"
	if _, ok := lc.p.reg.Token(script); !ok {
		return refuse("token script %q not in the corpus", script)
	}
	lc.out.Tokens = append(lc.out.Tokens, script)
	lc.emit(lInstr{Op: "Recruit", CS: script})
	return nil
}
