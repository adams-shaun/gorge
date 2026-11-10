package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

const firion = "Firion, Wild Rose Warrior"

// TestLowerCopypermSacupkeep: Firion's CopyPermanent (AtEOT$ SacrificeUpkeep,
// AddStaticAbilities$ an equip cost reduction of the copy itself) lowers to
// one copy op carrying kop.CpSacAtUpkeep and a copy exception that grants the
// equip reduction by an OvGate bit; every Equipment of the pool carries that
// gated static, and no other card does. Each shape beside the rider that the
// kernel does not carry refuses by name.
func TestLowerCopypermSacupkeep(t *testing.T) {
	t.Run("Firion", func(t *testing.T) {
		all, bs, pt := keclBuildPT(t, []string{firion, "Black Mage's Rod", "Buster Sword", "Grizzly Bears"})
		if r := lowerRefusals(all[keclIndex(t, all, firion)]); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		i := keclIndex(t, all, firion)
		copies := 0
		var gate uint8
		for _, ab := range bs[i].def.Abilities {
			for n := range ab.Prog {
				in := &ab.Prog[n]
				if in.Op != kop.OpCopyAsTokenEx {
					continue
				}
				copies++
				if got := int(uint16(in.B.N)); got != kop.CpSacAtUpkeep {
					t.Errorf("B.N %#x, want %#x (sacrifice at the next upkeep)", got, kop.CpSacAtUpkeep)
				}
				if in.C == 0 {
					t.Fatal("no copy exception")
				}
				gate = pt.overrides[in.C-1].GrantStatics
			}
		}
		if copies != 1 || gate != 0x40 {
			t.Fatalf("%d copies, grant bits %#x: want one copy granting the amount-2 bit 0x40", copies, gate)
		}
		for _, name := range []string{"Black Mage's Rod", "Buster Sword", "Grizzly Bears", firion} {
			j := keclIndex(t, all, name)
			var gated []*kop.Static
			for _, ab := range bs[j].def.Abilities {
				if ab.Kind == kop.AbStatic && ab.Static.OvGate != 0 {
					gated = append(gated, &ab.Static)
				}
			}
			if name == "Black Mage's Rod" || name == "Buster Sword" {
				if len(gated) != 1 || gated[0].Kind != kop.StatReduceAbility || gated[0].OvGate != gate || gated[0].B&kop.ReduceAbEquip == 0 || gated[0].A != 2 {
					t.Errorf("%s: gated statics %+v, want one equip reduction of 2 on bit %#x", name, gated, gate)
				}
			} else if len(gated) != 0 {
				t.Errorf("%s carries %d gated statics, want none", name, len(gated))
			}
		}
	})
	t.Run("without Firion no equipment carries a gated static", func(t *testing.T) {
		all, bs := keclBuild(t, []string{"Black Mage's Rod"})
		j := keclIndex(t, all, "Black Mage's Rod")
		for _, ab := range bs[j].def.Abilities {
			if ab.Kind == kop.AbStatic && ab.Static.OvGate != 0 {
				t.Errorf("a gated static on the lone Rod: %+v", ab.Static)
			}
		}
	})
	const (
		trig = "T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Equipment.YouCtrl+!token+Other | TriggerZones$ Battlefield | Execute$ TrigCopy | TriggerDescription$ x\n"
		stat = "SVar:EquipReduce:Mode$ ReduceCost | ValidCard$ Card.Self | ValidSpell$ Activated.Equip | Amount$ 2 | Description$ x\n"
		line = "DB$ CopyPermanent | Defined$ TriggeredCardLKICopy | AddStaticAbilities$ EquipReduce | AtEOT$ SacrificeUpkeep"
	)
	body := func(copyLine, extra string) string {
		return trig + "SVar:TrigCopy:" + copyLine + "\n" + extra
	}
	t.Run("inline shape lowers", func(t *testing.T) {
		if r := sacLower(t, body(line, stat)); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
	})
	for _, v := range []struct{ name, src, want string }{
		{"your upkeep", body(strings.Replace(line, "SacrificeUpkeep", "YourSacrificeUpkeep", 1), stat), `AtEOT "YourSacrificeUpkeep"`},
		{"exile at upkeep", body(strings.Replace(line, "SacrificeUpkeep", "YourExileUpkeep", 1), stat), `AtEOT "YourExileUpkeep"`},
		{"beside AtEOTTrig", body(line+" | AtEOTTrig$ Sacrifice", stat), "beside AtEOTTrig$"},
		{"another static mode", body(line, strings.Replace(stat, "ReduceCost", "RaiseCost", 1)), "AddStaticAbilities$"},
		{"another ability class", body(line, strings.Replace(stat, "Activated.Equip", "Activated.Exhaust", 1)), "AddStaticAbilities$"},
		{"a reduction of 5", body(line, strings.Replace(stat, "Amount$ 2", "Amount$ 5", 1)), "a literal 1 to 4 less"},
		{"a computed reduction", body(line, strings.Replace(stat, "Amount$ 2", "Amount$ X", 1)), "a literal 1 to 4 less"},
		{"a named static that does not exist", body(line, ""), "AddStaticAbilities$"},
		{"two statics", body(strings.Replace(line, "EquipReduce", "EquipReduce,EquipReduce", 1), stat), "AddStaticAbilities$"},
	} {
		t.Run(v.name, func(t *testing.T) {
			kdefWant(t, sacLower(t, v.src), v.want)
		})
	}
}

// keclBuildPT is keclBuild with the pool's tables, which hold the overrides.
func keclBuildPT(t *testing.T, names []string) ([]*lCard, []built, *poolTables) {
	t.Helper()
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, bs, pt, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	return all, bs, pt
}

// sacLower lowers an inline Firion-like card with body as its ability lines
// and returns its refusals.
func sacLower(t *testing.T, body string) []string {
	t.Helper()
	src := "Name:Rose\nManaCost:2 R\nTypes:Legendary Creature Human Rebel Warrior\nPT:3/3\n" + body
	card, diags := cards.ParseBytes("sacupkeep_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Rose"}, byName: map[string]*cards.Card{"Rose": card}, nDecks: map[string]int{}, reg: cards.NewRegistry(),
		typeWords: map[string]bool{"Equipment": true}}
	var got []string
	for _, r := range lowerRefusals(lowerCard(p, "Rose", card, false)) {
		got = append(got, r.Reason)
	}
	return got
}
