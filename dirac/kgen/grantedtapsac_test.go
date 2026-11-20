package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// ktapsacLower lowers a synthetic Lotus-Ring-shaped card body and returns the
// lowered card and its refusals.
func ktapsacLower(t *testing.T, sVar string) (*lCard, []refusal) {
	t.Helper()
	src := "Name:Lotus Ring\nManaCost:3\nTypes:Artifact Equipment\nK:Indestructible\n" +
		"S:Mode$ Continuous | AffectedDefined$ Equipped | AddToughness$ 3 | AddPower$ 3 | AddKeyword$ Vigilance | AddAbility$ Damage | Description$ Equipped creature gets +3/+3 and has vigilance and \"{T}, Sacrifice this creature: Add three mana of any one color.\"\n" +
		sVar + "\nK:Equip:3\n"
	card, diags := cards.ParseBytes("lotus_ring.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Lotus Ring"}, byName: map[string]*cards.Card{"Lotus Ring": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	return lw.cards[0], lw.cards[0].Refusals
}

// TestGrantedTapSacLowering: the granted mana ability whose cost is {T} AND
// sacrificing the source (Lotus Ring's upstream script changed "Sacrifice
// this creature" to "{T}, Sacrifice this creature", matching the Oracle
// text; Goldspan Dragon's granted Treasure ability is the same shape) lowers
// as a plain tap grant whose step also sacrifices the source
// (dirac/mana_grantcost.go), where the sacrifice-only grant alone was
// refused. The sacrifice-only script (the corpus card until the fetch lands)
// still lowers as it did.
func TestGrantedTapSacLowering(t *testing.T) {
	for _, v := range []struct {
		name string
		cost string
		lir  string
	}{
		{"T Sac", "SVar:Damage:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | Amount$ 3 | SpellDescription$ Add three mana of any one color.", "Mana Granted cost=T SacSelf"},
		{"Sac only", "SVar:Damage:AB$ Mana | Cost$ Sac<1/CARDNAME> | Produced$ Any | Amount$ 3 | SpellDescription$ Add three mana of any one color.", "Mana Granted cost=SacSelf"},
	} {
		t.Run(v.name, func(t *testing.T) {
			c, r := ktapsacLower(t, v.cost)
			if len(r) != 0 {
				t.Fatalf("a granted mana ability of a supported cost shape is refused: %v", r)
			}
			var sb strings.Builder
			dumpLIR(&sb, c)
			if !strings.Contains(sb.String(), v.lir) {
				t.Errorf("lowered IR lacks %q:\n%s", v.lir, sb.String())
			}
		})
	}
}

// TestGrantedTapSacStillRefused: the gate keeps refusing the granted shapes
// the step cannot pay: a mana fee beside the tap and sacrifice, a dynamic
// amount, and a sacrifice of a count of other permanents.
func TestGrantedTapSacStillRefused(t *testing.T) {
	for _, v := range []struct {
		name, cost, want string
	}{
		{"mana fee", "T Sac<1/CARDNAME> 1", "granted mana ability with a mana fee"},
		{"sacrifice other", "T Sac<2/Creature> ", "granted mana ability whose cost is not {T}"},
	} {
		t.Run(v.name, func(t *testing.T) {
			_, r := ktapsacLower(t, "SVar:Damage:AB$ Mana | Cost$ "+v.cost+" | Produced$ Any | Amount$ 3 | SpellDescription$ Add three mana of any one color.")
			got := ""
			for _, q := range r {
				got += q.Node + ": " + q.Reason + "; "
			}
			if !strings.Contains(got, v.want) {
				t.Fatalf("want refusal %q, got %q", v.want, got)
			}
		})
	}
}
