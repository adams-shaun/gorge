package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

func TestEffectSpecClasses(t *testing.T) {
	t.Parallel()
	c, pool := testCorpus(t)
	seen := map[string]bool{}
	counts := map[uint8]int{}
	names := [...]string{"Kill", "Damage", "Counter", "Bounce", "Pump", "Debuff", "Other"}
	for _, p := range Pairings(pool, 40, 11) {
		_, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		fp, err := FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i := range tbl.Cards {
			name := tbl.Cards[i].Name
			card, ok := c.Reg.Lookup(name)
			if !ok || seen[name] {
				continue
			}
			sa := card.Faces[0].SpellAbility()
			if sa == nil {
				continue
			}
			seen[name] = true
			e := fp.Effect(kop.CardID(i))
			counts[e.Class]++
			if e.Class > feat.EffOther {
				t.Errorf("%s: class %d out of range", name, e.Class)
			}
			switch sa.API {
			case "Counter":
				if e.Class != feat.EffCounter {
					t.Errorf("%s: Counter card class %d", name, e.Class)
				}
			case "Destroy":
				if e.Class == feat.EffOther {
					t.Errorf("%s: Destroy card is EffOther", name)
				}
			case "DealDamage":
				if n, ok := atoiOK(sa.Params["NumDmg"]); ok && int(e.Amount) != n {
					t.Errorf("%s: NumDmg %d, Amount %d", name, n, e.Amount)
				}
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no FDN card with a spell ability reached the pool")
	}
	var ks []int
	for k := range counts {
		ks = append(ks, int(k))
	}
	sort.Ints(ks)
	for _, k := range ks {
		t.Logf("class %-8s %d", names[k], counts[uint8(k)])
	}
}

func atoiOK(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func TestEffectSpecAmountX(t *testing.T) {
	e := effectSpec(&cards.SA{API: "DealDamage", Params: map[string]string{"NumDmg": "X", "ValidTgts": "Any"}}, "{X}{R}")
	if e.Class != feat.EffDamage || e.Amount != -1 || !e.TgtPlayer || !e.TgtCreat || !e.CostX {
		t.Fatalf("X damage: %+v", e)
	}
	e = effectSpec(&cards.SA{API: "DealDamage", Params: map[string]string{"NumDmg": "Count$xPaid", "ValidTgts": "Any"}}, "{R}")
	if e.Amount != -2 || e.CostX {
		t.Fatalf("xPaid damage: %+v", e)
	}
	e = effectSpec(&cards.SA{API: "Pump", Params: map[string]string{"NumAtt": "-2", "NumDef": "-2", "ValidTgts": "Creature"}}, "{B}")
	if e.Class != feat.EffDebuff || e.Amount != 2 || e.NumAtt != -2 {
		t.Fatalf("debuff: %+v", e)
	}
	e = effectSpec(&cards.SA{API: "Pump", Params: map[string]string{"NumAtt": "-X", "NumDef": "-X", "ValidTgts": "Creature"}}, "{X}{B}")
	if e.Class != feat.EffDebuff || e.Amount != -1 || !e.CostX {
		t.Fatalf("-X debuff: %+v", e)
	}
	e = effectSpec(&cards.SA{API: "Pump", Params: map[string]string{"NumAtt": "-Count$Valid Creature", "NumDef": "-Count$Valid Creature", "ValidTgts": "Creature"}}, "{B}")
	if e.Class != feat.EffDebuff || e.Amount != -2 {
		t.Fatalf("-expr debuff: %+v", e)
	}
}
