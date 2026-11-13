package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

const (
	cwSelfSacFeeSpell = "Name:Face Self Sac Fee BR\nManaCost:B/R B/R\nTypes:Instant\n" +
		"A:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw one.\nOracle:x\n"
	cwSelfSacFeePizza = "Name:Face Self Sac Fee Pizza\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ 1 T Sac<1/CARDNAME/this artifact> | Produced$ Any | Amount$ 1\nOracle:x\n"
)

// A hybrid pip announced as R must stay payable when a self-sacrificing source
// can make the R for a {1} fee (CR 601.2b, 601.2g). Two black mana in the pool
// pay the fee, the source makes one R, and the Mountain makes the other, so the
// second pip's R face is legal beside its B face.
func TestCastWindowHybridFaceFromSelfSacFee(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 887, cwSelfSacFeeSpell, cwSelfSacFeePizza, cwMountain)
	pizza := putCreature(t, e, 0, cwSelfSacFeePizza)
	mountain := putCreature(t, e, 0, cwMountain)
	if pizza == 0 || mountain == 0 {
		t.Fatalf("precondition: sources missing: pizza=%d mountain=%d", pizza, mountain)
	}
	addMana(t, e, 0, "BB")
	if got := e.G.Players[0].Pool; got.Total() != 2 {
		t.Fatalf("precondition: pool total = %d, want 2", got.Total())
	}

	first := cwFaceAsk(t, e, spell)
	red, ok := first["pay_R"]
	if !ok {
		t.Fatalf("first hybrid face R withheld: %v", first)
	}
	if _, ok := first["pay_B"]; !ok {
		t.Fatalf("first hybrid face B withheld: %v", first)
	}
	submitChoices(t, e, red)

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("after announcing R = %+v, want the second pip face ask", d)
	}
	faces := map[string]int{}
	for _, o := range d.Options {
		faces[o.Kind] = o.Index
	}
	if _, ok := faces["pay_B"]; !ok {
		t.Fatalf("second hybrid face B withheld: %v", faces)
	}
	if _, ok := faces["pay_R"]; !ok {
		t.Fatalf("second hybrid face R withheld: the {1} self-sacrifice source can make the second R: %v", faces)
	}
}
