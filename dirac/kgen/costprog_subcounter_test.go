package kgen

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// costProgBuild lowers and builds one probe card and returns its def, its
// refusals and the pool's cost programs.
func costProgBuild(t *testing.T, name, src string) (kop.CardDef, []refusal, [][]kop.CostAtom) {
	t.Helper()
	card, diags := cards.ParseBytes("costprog_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	return bs[0].def, lw.cards[0].Refusals, pt.costProgs
}

// TestCostProgLowerSubCounterAll: a SubCounter<All/KIND> activation cost
// lowers as a cost program, [Mana] [Tap] SubCounter{N: the source's KIND
// counters}, with the ability's Cost keeping its mana and {T} for the offer;
// SVar$CostCountersRemoved reads the entry amount the atom records. A
// filtered target, a wildcard kind and a second counter part refuse in their
// census class (cost:SubCounter, the cost-token class), and a part the
// program does not carry refuses.
func TestCostProgLowerSubCounterAll(t *testing.T) {
	head := "ManaCost:2\nTypes:Artifact\n"
	ctr := kop.CostAtom{K: kop.CostAtomSubCounter, N: kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrCharge)}, Ctr: kop.CtrCharge}
	for _, c := range []struct {
		name, cost string
		want       []kop.CostAtom
		tap        bool
	}{
		{"Pile Mana Tap", "2 T SubCounter<All/CHARGE>", []kop.CostAtom{{K: kop.CostAtomMana}, {K: kop.CostAtomTap}, ctr}, true},
		{"Pile Tap", "T SubCounter<All/CHARGE>", []kop.CostAtom{{K: kop.CostAtomTap}, ctr}, true},
		{"Pile Alone", "SubCounter<All/CHARGE>", []kop.CostAtom{ctr}, false},
		{"Pile Nickname", "1 SubCounter<All/CHARGE/NICKNAME>", []kop.CostAtom{{K: kop.CostAtomMana}, ctr}, false},
	} {
		src := "Name:" + c.name + "\n" + head + "A:AB$ GainLife | Cost$ " + c.cost + " | Defined$ You | LifeAmount$ N\nSVar:N:SVar$CostCountersRemoved/Twice\n"
		d, rs, progs := costProgBuild(t, c.name, src)
		if len(rs) != 0 {
			t.Errorf("%s: refused %q", c.name, kpaylifexReasons(rs))
			continue
		}
		a := d.Abilities[0]
		if a.CostProg == 0 || int(a.CostProg) >= len(progs) {
			t.Errorf("%s: CostProg %d of %d programs", c.name, a.CostProg, len(progs))
			continue
		}
		if got := progs[a.CostProg]; !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: program %+v, want %+v", c.name, got, c.want)
		}
		if got := a.Cost.Flags&kop.CostTap != 0; got != c.tap {
			t.Errorf("%s: Cost {T} flag %v, want %v", c.name, got, c.tap)
		}
		if a.Cost.Flags&(kop.CostRemoveCtr|kop.CostLoyaltyX) != 0 {
			t.Errorf("%s: the pile rides a counter flag %v", c.name, a.Cost.Flags)
		}
	}
	for _, c := range []struct{ name, cost, why string }{
		{"Pile Host", "T SubCounter<All/CHARGE/OriginalHost>", `cost token "SubCounter<All/CHARGE/OriginalHost>"`},
		{"Pile Any", "T SubCounter<All/Any>", `cost token "SubCounter<All/Any>"`},
		{"Pile Twice", "T SubCounter<1/CHARGE> SubCounter<All/CHARGE>", `cost token "SubCounter<All/CHARGE>"`},
		{"Pile Life", "T PayLife<2> SubCounter<All/CHARGE>", "a part the cost program does not carry"},
	} {
		src := "Name:" + c.name + "\n" + head + "A:AB$ GainLife | Cost$ " + c.cost + " | Defined$ You | LifeAmount$ 1\n"
		_, rs, _ := costProgBuild(t, c.name, src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, c.why) {
			t.Errorf("%s: refusals %q, want %q", c.name, all, c.why)
		}
	}
	// the count read outside a pile cost refuses
	src := "Name:Pile Reader\n" + head + "A:AB$ GainLife | Cost$ T | Defined$ You | LifeAmount$ N\nSVar:N:SVar$CostCountersRemoved\n"
	if _, rs, _ := costProgBuild(t, "Pile Reader", src); !strings.Contains(kpaylifexReasons(rs), "CostCountersRemoved") {
		t.Errorf("Pile Reader: refusals %q, want the CostCountersRemoved refusal", kpaylifexReasons(rs))
	}
}
