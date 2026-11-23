package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBattlefieldPickUntilLeavesLowers: a Hidden$ True battlefield-to-exile
// pick with Duration$ UntilHostLeavesPlay lowers as an OpChooseMove carrying
// kop.SrBattleLink (the picked set returns to the battlefield when the source
// leaves). Glorious Protector (the any-number pick, no rider) and Abdel Adrian
// (the pick plus its token rider) lower whole; Yannik's pick lowers with the
// mandatory exactly-one count and its card lowers whole (the reflexive
// divided-counters body lowers too; see TestYannikReflexiveDividedCountersLowers). Every other Duration$ on the
// path, and the zone pairs the link does not model, still refuse.
func TestBattlefieldPickUntilLeavesLowers(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Glorious Protector", form: "any-number battlefield pick until the host leaves", lir: "ChooseMove obj=You A=Count({types=0x2 notsub=Angel ctrl=1}) C=5 CS=from=Battlefield|UntilLeaves "},
		{card: "Abdel Adrian, Gorion's Ward", form: "any-number pick plus the token rider", lir: "CS=from=Battlefield|UntilLeaves|Remember"},
		{card: "Abdel Adrian, Gorion's Ward", form: "token per exiled permanent", lir: "CreateToken obj=You A=RememberedCount(ctx) CS=w_1_1_soldier"},
		{card: "Yannik, Scavenging Sentinel", form: "mandatory exactly-one pick", lir: "ChooseMove obj=You A=1 C=5 CS=from=Battlefield|UntilLeaves|Remember|Mandatory"},
		{card: "Lumbering Battlement", form: "ExiledWith reader keeps its own blocker", refused: "ExiledWith reader with a ChooseMove exile"},
		{card: "Idris, Soul of the TARDIS", form: "Vanishing keeps its own blocker", refused: "keyword \"Vanishing\""},
	})

	lw := lowerNamedPool(t, []string{"Glorious Protector", "Abdel Adrian, Gorion's Ward", "Yannik, Scavenging Sentinel"})
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "Duration") {
				t.Errorf("%s refused on Duration$: %s", c.Name, r.Reason)
			}
		}
	}

	// the kernel instruction: the link bit rides B, bit 0 (the hand link) stays clear
	all, bs, pt, err := buildPool(lowerNamedPool(t, []string{"Glorious Protector"}))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for i, c := range all {
		if c.Name != "Glorious Protector" {
			continue
		}
		if len(bs[i].refusals) != 0 {
			t.Fatalf("Glorious Protector refused at build: %v", bs[i].refusals)
		}
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op != kop.OpChooseMove {
					continue
				}
				found++
				if in.B.N&kop.SrBattleLink == 0 || in.B.N&1 != 0 {
					t.Errorf("battlefield pick B %#x: want SrBattleLink only", in.B.N)
				}
				if z := pt.filters[in.F].Zone; z != kop.ZoneNone {
					t.Errorf("battlefield pick filter zone %d: battleLinkPick keys on the zoneless battlefield filter, so a zoned filter silently drops the return link", z)
				}
			}
		}
	}
	if found != 1 {
		t.Fatalf("%d picks, want the card's one", found)
	}

	for _, c := range []struct{ name, root, want string }{
		{"another Duration", "SP$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Hidden$ True | ChangeType$ Creature.YouCtrl | ChangeNum$ 1 | Mandatory$ True | Duration$ UntilYourNextTurn", "ChangeZone Duration"},
		{"battlefield to hand", "SP$ ChangeZone | Origin$ Battlefield | Destination$ Hand | Hidden$ True | ChangeType$ Creature.YouCtrl | ChangeNum$ 1 | Mandatory$ True | Duration$ UntilHostLeavesPlay", "ChangeZone Duration"},
		{"graveyard to exile", "SP$ ChangeZone | Origin$ Graveyard | Destination$ Exile | Hidden$ True | ChangeType$ Creature.YouCtrl | ChangeNum$ 1 | Mandatory$ True | Duration$ UntilHostLeavesPlay", "ChangeZone Duration"},
	} {
		got, _ := czLowered(t, c.root)
		if len(got) == 0 || !strings.Contains(strings.Join(got, ";"), c.want) {
			t.Errorf("%s: refusals %v, want one naming %q", c.name, got, c.want)
		}
	}
	got, ops := czLowered(t, "SP$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Hidden$ True | ChangeType$ Creature.YouCtrl | ChangeNum$ 1 | Mandatory$ True | Duration$ UntilHostLeavesPlay")
	if len(got) != 0 || len(ops) != 1 || !strings.Contains(ops[0], "UntilLeaves") {
		t.Errorf("ChangeNum$ 1 pick: refusals %v ops %v, want one ChooseMove with UntilLeaves", got, ops)
	}
}
