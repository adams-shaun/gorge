package oracle

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// hbotReflexTgts gives the latent abilities an ability's ReflexiveTrigger
// instructions put on the stack (an ImmediateTrigger's "when you do" body) the
// targeted SAs of the Execute$ chain they lower: the kernel poses the body's
// targets as the latent ability's own ask, which has no gorge ability index
// (SBTCards.gorgeAbility), while gorge's bot reads the body SA's TargetEffect.
// The n-th ReflexiveTrigger of prog is the n-th ImmediateTrigger of sa's chain.
func hbotReflexTgts(abs []hbot.AbFacts, face *cards.Face, prog []kop.Instr, sa *cards.SA) {
	var chain []*cards.SA
	for x := sa; x != nil; x = x.Sub {
		if x.API == "ImmediateTrigger" {
			chain = append(chain, x)
		}
	}
	n := 0
	for i := range prog {
		if prog[i].Op != kop.OpReflexiveTrigger {
			continue
		}
		if n >= len(chain) {
			return
		}
		body := cards.ResolveSVar(face.SVars, chain[n].Params["Execute"])
		n++
		if l := int(prog[i].C); body != nil && l >= 0 && l < len(abs) && len(abs[l].Tgt) == 0 {
			abs[l].Tgt = hbotTgtChain(face, body)
		}
	}
}
