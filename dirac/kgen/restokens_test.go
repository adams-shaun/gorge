package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// resTokBuild lowers and builds one synthetic card script, returning its
// CardDef and refusal reasons.
func resTokBuild(t *testing.T, name, src string) (kop.CardDef, []string) {
	t.Helper()
	card, diags := cards.ParseBytes("restok_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	var rs []string
	for _, r := range lw.cards[0].Refusals {
		rs = append(rs, r.Reason)
	}
	if len(rs) != 0 {
		return kop.CardDef{}, rs
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	return bs[0].def, nil
}

// resTokPay finds the card's one OpUnlessPaid / OpUnlessSwitched instruction.
func resTokPay(t *testing.T, d kop.CardDef) kop.Instr {
	t.Helper()
	for i := range d.Abilities {
		for _, in := range d.Abilities[i].Prog {
			if in.Op == kop.OpUnlessPaid || in.Op == kop.OpUnlessSwitched {
				return in
			}
		}
	}
	t.Fatal("no unless instruction built")
	return kop.Instr{}
}

// TestResolutionCostTokens: a ward of N poison counters, an UnlessCost$ of
// the resolving source's damage to the payer (DamageYou<X>) and one of the
// payer drawing X (Draw<X/You>) lower to their table forms; every near
// spelling stays refused by name.
func TestResolutionCostTokens(t *testing.T) {
	ward := "Name:Poison Ward Probe\nManaCost:1 B\nTypes:Creature Snake\nPT:2/2\nK:Ward:AddCounterYou<%s>\n"
	d, rs := resTokBuild(t, "Poison Ward Probe", strings.Replace(ward, "%s", "3/POISON", 1))
	if len(rs) != 0 {
		t.Fatalf("poison ward refused: %v", rs)
	}
	if d.WardPoison != 3 || d.Keywords&kop.KwWard == 0 || d.WardN != 0 || d.WardLife != 0 || d.WardEvidence != 0 {
		t.Errorf("poison ward: WardPoison %d KwWard %v WardN %d WardLife %d", d.WardPoison, d.Keywords&kop.KwWard != 0, d.WardN, d.WardLife)
	}
	for _, bad := range []string{"3/ENERGY", "0/POISON", "300/POISON"} {
		if _, rs := resTokBuild(t, "Poison Ward Probe", strings.Replace(ward, "%s", bad, 1)); len(rs) == 0 {
			t.Errorf("ward AddCounterYou<%s> lowered, want a refusal", bad)
		}
	}

	dmg := "Name:Damage Probe\nManaCost:3 R\nTypes:Creature Human\nPT:4/6\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ Smash | TriggerDescription$ probe\n" +
		"SVar:Smash:DB$ Destroy | UnlessCost$ %s | ValidTgts$ Creature,Land\n" +
		"SVar:X:Count$CardPower\n"
	d, rs = resTokBuild(t, "Damage Probe", strings.Replace(dmg, "%s", "DamageYou<X>", 1))
	if len(rs) != 0 {
		t.Fatalf("DamageYou<X> refused: %v", rs)
	}
	in := resTokPay(t, d)
	if in.Op != kop.OpUnlessPaid || in.C&0xF != kop.PayDraw || in.B.N != kop.PayBSubDamage {
		t.Errorf("DamageYou<X>: op %v C %#x B %d, want OpUnlessPaid over PayDraw with the damage sentinel", in.Op, in.C, in.B.N)
	}
	// a literal amount and a kind other than the face's X stay refused by name
	for _, bad := range []string{"DamageYou<2>", "DamageYou<X/You>"} {
		_, rs := resTokBuild(t, "Damage Probe", strings.Replace(dmg, "%s", bad, 1))
		if all := strings.Join(rs, " | "); !strings.Contains(all, "cost token") {
			t.Errorf("UnlessCost$ %s: refusals %q, want a cost token refusal", bad, all)
		}
	}
	noX := strings.Replace(strings.Replace(dmg, "%s", "DamageYou<X>", 1), "SVar:X:Count$CardPower\n", "", 1)
	if _, rs := resTokBuild(t, "Damage Probe", noX); len(rs) == 0 {
		t.Error("DamageYou<X> without SVar:X lowered, want a refusal")
	}

	draw := "Name:Draw Probe\nManaCost:2 U\nTypes:Enchantment Saga\nK:Chapter:1:DBDraw\n" +
		"SVar:DBDraw:DB$ Draw | Defined$ Opponent | NumCards$ 1 | UnlessCost$ %s | UnlessPayer$ You | UnlessSwitched$ True\n" +
		"SVar:X:Count$Valid " + "Artifact.YouCtrl\n"
	d, rs = resTokBuild(t, "Draw Probe", strings.Replace(draw, "%s", "Draw<X/You>", 1))
	if len(rs) != 0 {
		t.Fatalf("Draw<X/You> refused: %v", rs)
	}
	in = resTokPay(t, d)
	if in.Op != kop.OpUnlessSwitched || in.C&0xF != kop.PayDraw || in.B.N != 0 {
		t.Errorf("Draw<X/You>: op %v C %#x B %d, want OpUnlessSwitched over a plain PayDraw", in.Op, in.C, in.B.N)
	}
	for _, bad := range []string{"Draw<X/Opponent>", "Draw<2/You>"} {
		_, rs := resTokBuild(t, "Draw Probe", strings.Replace(draw, "%s", bad, 1))
		if all := strings.Join(rs, " | "); !strings.Contains(all, "cost token") && !strings.Contains(all, "draw cost") {
			t.Errorf("UnlessCost$ %s: refusals %q, want a draw or cost token refusal", bad, all)
		}
	}
}
