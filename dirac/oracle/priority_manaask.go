package oracle

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// Config.PriorityManaAsk: a colour-asking mana source whose Mana node carries
// UnlessCost$ 1 | UnlessPayer$ Player (Rhystic Cave) is a canonical priority
// option, and its asks are canonical decisions (dirac mana_priask.go). gorge
// lists the source as an "activate" option at priority; answering it poses the
// colour ask (KChoose "choosecolor") and then, for each living player in seat
// order, the unless ask (KModes "unless_pay": Pay 1, Don't pay). The payer's own
// mana window after a "Pay 1" is answered beneath the surface (internal), like
// every payment window.

// unlessAskSA reports whether sa is the head of a Rhystic-Cave-shaped chain: an
// activated ChooseColor whose only cost is {T} and whose sub-ability is the Mana
// node asking every player to pay {1}.
func unlessAskSA(sa *cards.SA) bool {
	if sa == nil || sa.API != "ChooseColor" || strings.TrimSpace(sa.Params["Cost"]) != "T" {
		return false
	}
	m := sa.Sub
	return m != nil && m.API == "Mana" && strings.TrimSpace(m.Params["UnlessCost"]) == "1" &&
		strings.TrimSpace(m.Params["UnlessPayer"]) == "Player" && strings.TrimSpace(m.Params["Produced"]) == "Chosen"
}

// manaAskActivation is the canonical activation of the "activate" option o: its
// source's unless-ask ability, by its index in the face (the kernel's ability
// index). false when the knob is off or the source has no such ability.
func (g *GorgeSide) manaAskActivation(o *decision.Option) (Canon, bool) {
	if !g.Cfg.PriorityManaAsk {
		return 0, false
	}
	ob := g.E.G.Obj(o.Obj)
	if ob == nil || ob.Face() == nil {
		return 0, false
	}
	for i, sa := range ob.Face().Abilities {
		if unlessAskSA(sa) && i <= 0x7F {
			return Pack(ActActivate, g.id.ref(g.E.G, o.Obj), RefNone, uint8(i)), true
		}
	}
	return 0, false
}

// manaAskColourAction is the canonical form of one option of the unless source's
// colour ask: the kernel's ChGenManaChoice pick, choose:mana valued by the
// colour (W U B R G = 0..4).
func (g *GorgeSide) manaAskColourAction(d *decision.Decision, o *decision.Option) (Canon, bool) {
	if !g.Cfg.PriorityManaAsk || o.Kind != "color" || d.ResumeKind != "choosecolor" || !unlessAskSA(d.ResumeSA) {
		return 0, false
	}
	code, ok := chooseCode("mana")
	if !ok {
		return 0, false
	}
	mode := o.Index
	if c := strings.Index(manaSyms, o.ManaSymbol); len(o.ManaSymbol) == 1 && c >= 0 {
		mode = c
	}
	return Pack(ActChoose+CanonKind(code), RefNone, RefNone, uint8(mode)), mode < 5
}
