package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// tpTreasure is a mana rock that sacrifices itself for one mana of any colour.
	tpTreasure = "Name:Target Paid Gem\nTypes:Artifact Treasure\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
)

// Targets are chosen at 601.2c, before mana abilities are activated (601.2g)
// and costs paid (601.2h), so a Spree spell whose only target is the mana
// source its payment sacrifices is castable: it finds its target gone on
// resolution (608.2b). The potential walk's lowering floats mana before the
// cast and cannot reach it, so the planned cast carries it.
func TestSpreeMayPayWithItsOwnOnlyTarget(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9411, ppFiller)
	spell := handCard(e, corpusCard(t, "Explosive Derailment"), 0)
	gem := onBoard(t, e, 0, tpTreasure)
	onBoard(t, e, 0, lrForest)
	onBoard(t, e, 0, lrForest)
	e.paidManaCensus = true
	d := ppPriority(t, e)
	e.EnsurePaymentActions()
	d = e.Pending()
	if !ppHasAction(d, spell) {
		t.Fatalf("no planned cast for the Spree spell whose only target is the gem it pays with")
	}
	submitPaymentPlan(t, e, d, ppAction(t, d, spell))
	for guard := 0; guard < 20; guard++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		if d.Kind == decision.KChoose {
			for _, o := range d.Options {
				in.Choices = append(in.Choices, o.Index)
				if len(in.Choices) >= max(d.Min, 1) {
					break
				}
			}
		} else {
			in.Choices = []int{d.Options[0].Index}
		}
		if err := e.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.G.Stack) != 1 || e.G.Obj(gem).Zone == state.ZBattlefield {
		t.Fatalf("stack %v, gem in zone %v: want the spell cast and the gem sacrificed to pay", e.G.Stack, e.G.Obj(gem).Zone)
	}
	// Both players pass: the spell finds its only target gone and does nothing (CR 608.2b).
	for guard := 0; guard < 4 && len(e.G.Stack) > 0; guard++ {
		d := e.Pending()
		ppChoose(t, e, func(o decision.Option) bool { return o.Kind == "pass" })
		_ = d
	}
	if len(e.G.Stack) != 0 || e.G.Obj(spell).Zone != state.ZGraveyard {
		t.Fatalf("stack %v, spell in %v: want it resolved to the graveyard", e.G.Stack, e.G.Obj(spell).Zone)
	}
}
