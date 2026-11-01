package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLateBargainForms: the three late Bargain carriers lower
// and build. Brave the Wilds' bargained target range (CR 702.166d) is the
// kicked-range slot, Realm-Scorcher Hellkite's triggered "Combo Any" mana is
// ExtManaChoice (CR 106.3, 605.1a), Torch the Tower's
// ReplaceDyingDefined$ Remembered beside RememberDamaged$ is the remembered,
// exile-if-dies damage (CR 614.1a). The checks read the built table.
func TestLateBargainForms(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Brave the Wilds", "Realm-Scorcher Hellkite", "Torch the Tower"})
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	defs := map[string]kop.CardDef{}
	for _, b := range bs {
		if len(b.refusals) != 0 {
			t.Errorf("%s refused: %v", b.def.Name, b.refusals)
		}
		defs[b.def.Name] = b.def
	}
	for _, c := range lw.cards {
		if len(c.Refusals) != 0 {
			t.Errorf("%s lowered with refusals: %v", c.Name, c.Refusals)
		}
	}

	brave := defs["Brave the Wilds"]
	if !brave.Bargain || len(brave.Abilities) == 0 {
		t.Fatalf("Brave the Wilds: Bargain=%v abilities=%d", brave.Bargain, len(brave.Abilities))
	}
	if ts := brave.Abilities[0].Targets[0]; ts.TFlags&kop.TgfKickedRange == 0 ||
		ts.Min != 0 || ts.Max != 0 || ts.MinKicked != 1 || ts.MaxKicked != 1 {
		t.Errorf("Brave the Wilds target = %+v, want TgfKickedRange 0/0 + 1/1", ts)
	}

	found := false
	for _, ab := range defs["Realm-Scorcher Hellkite"].Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpExt && in.A == kop.Const(int(kop.ExtManaChoice)) {
				found = true
				if in.B != kop.Const(4) || in.C&0x1ff != 0x1f {
					t.Errorf("Hellkite ExtManaChoice B=%+v C=%#x, want amount 4, mask 0x1f", in.B, in.C&0x1ff)
				}
			}
		}
	}
	if !found {
		t.Error("Realm-Scorcher Hellkite: no ExtManaChoice instruction")
	}

	found = false
	for _, ab := range defs["Torch the Tower"].Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpDealDamageEx {
				found = true
				want := int16(kop.DmgRemember | kop.DmgExileIfDiesThisTurn)
				if in.C&want != want {
					t.Errorf("Torch the Tower DealDamageEx C=%#x, want Remember|ExileIfDiesThisTurn", in.C)
				}
			}
		}
	}
	if !found {
		t.Error("Torch the Tower: no OpDealDamageEx")
	}
}
