package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoRepeateachCztShapes: Ezuri's Predation and Mirror-Style Master lower
// with no refusal. Ezuri is a ChangeZoneTable$ RepeatEach whose body is a
// Beast token then the Imprinted/Remembered Fight, ordered by the controller
// (kop.RepeatEachOrder). Mirror is a batched CopyPermanent whose remembered
// copies a DpMany end-of-combat delayed trigger exiles (kop.DpMany; its
// latent body is the battlefield-origin RefDelayedRemembered exile).
func TestDemoRepeateachCztShapes(t *testing.T) {
	names := []string{"Ezuri's Predation", "Mirror-Style Master"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	var ezuri, mirror, many, exileMany bool
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		for _, ab := range b.def.Abilities {
			for i, in := range ab.Prog {
				if in.Op == kop.OpRepeatEach && in.C&kop.RepeatEachOrder != 0 {
					// Ezuri: CreateToken, the union Fight, ClearRemembered
					if int(in.B.N) != 3 || i+3 >= len(ab.Prog) {
						t.Errorf("%s: ordered RepeatEach body %d", b.def.Name, in.B.N)
						continue
					}
					tok, fight, clear := ab.Prog[i+1], ab.Prog[i+2], ab.Prog[i+3]
					if tok.Op != kop.OpCreateTokenEx || tok.B.N&kop.TokRememberObj == 0 {
						t.Errorf("%s: body token %v, want CreateTokenEx with RememberTokens", b.def.Name, tok)
					}
					if fight.Op != kop.OpFight || fight.Obj != kop.RefEach || fight.C != int16(kop.RefRemembered) {
						t.Errorf("%s: body fight %v, want RefEach vs RefRemembered", b.def.Name, fight)
					}
					if clear.Op != kop.OpClearRemembered {
						t.Errorf("%s: body tail %v, want ClearRemembered", b.def.Name, clear)
					}
					ezuri = true
				}
				if in.Op == kop.OpCopyAsTokenEx && in.Obj == kop.RefEach &&
					in.B.N&(kop.CpRemember|kop.CpTapped|kop.CopyAttacking) == kop.CpRemember|kop.CpTapped|kop.CopyAttacking {
					mirror = true
				}
				if in.Op == kop.OpDelayedProgram && in.B.N == kop.DpMany {
					many = true
				}
				if in.Op == kop.OpExileEx && in.C == kop.MvFromBattlefield && in.Obj == kop.RefDelayedRemembered {
					exileMany = true
				}
			}
		}
	}
	if !ezuri {
		t.Error("Ezuri's Predation: no ordered RepeatEach token-fight body")
	}
	if !mirror {
		t.Error("Mirror-Style Master: no batched copy-of-Each body")
	}
	if !many || !exileMany {
		t.Errorf("Mirror-Style Master: DpMany delayed program %v, exile-many body %v; want both", many, exileMany)
	}
}
