package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

var extra, stepMode int

func scratchBoard(t *testing.T) (*scen, map[string]CardID, Obj) {
	t.Helper()

	set := cardsettest.Synth(t, "tail-02")
	deck := []cardset.Entry{
		{Name: "Island", Count: 2},
		{Name: "Plains", Count: 1},
		{Name: "Racers' Ring", Count: 1},
		{Name: "Energy Refractor", Count: 1},
		{Name: "Bubbling Cauldron", Count: 1},
		{Name: "Ominous Seas", Count: 1},
		{Name: "Blue Sun's Zenith", Count: 1},
		{Name: "Call to Mind", Count: 1},
		{Name: "Ondu Cleric", Count: 1},
		{Name: "Chamber of Manipulation", Count: 1},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		panic(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	ra := tbl.Cards[id["Racers' Ring"]].Abilities[0]
	fmt.Printf("RING aflags=%d choice=%d manaCol=%d n=%d costflags=%d\n", ra.AFlags, ra.AFlags&kop.AfManaChoice, ra.ManaColors, ra.ManaN, ra.Cost.Flags)
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, GorgeLowering: true, ArrangeRestOrder: true})
	if stepMode != 0 {
		sc.s.step = kop.Step(stepMode)
	}
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Plains"], 0)
	ring := sc.perm(id["Racers' Ring"], 0)
	sc.s.perms[sc.slot(ring)].flags &^= pfTapped // its ETB-tapped replacement fired; the game's copy was untapped
	sc.perm(id["Energy Refractor"], 0)
	sc.perm(id["Bubbling Cauldron"], 0)
	sc.perm(id["Ominous Seas"], 0)
	var zen Obj
	if extra&1 != 0 {
		sc.hand(id["Call to Mind"], 0)
	}
	if extra&2 != 0 {
		sc.hand(id["Ondu Cleric"], 0)
	}
	if extra&4 != 0 {
		sc.hand(id["Chamber of Manipulation"], 0)
	}
	zen = sc.hand(id["Blue Sun's Zenith"], 0)
	return sc, id, zen
}

func TestScratchMBCBlueSun(t *testing.T) {
	var all []byte
	for _, e := range []int{0, 7} {
		for _, st := range []int{0, 1, 2, 3, 4, 11} {
			stepMode = st
			all = append(all, runExtra(t, e)...)
		}
	}
	panic(string(all))
}

func runExtra(t *testing.T, e int) []byte {
	extra = e
	sc, _, zen := scratchBoard(t)
	var b []byte
	b = fmt.Appendf(b, "EXTRA=%d STEP=%d zen obj=%d zone=%d\n", extra, stepMode, zen, sc.s.objs[zen].zone)
	d := sc.s.Step()
	b = fmt.Appendf(b, "decision kind %d player %d n %d\n", d.Kind, d.Player, d.N)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		b = fmt.Appendf(b, "opt[%d] kind=%d src=%d tgt=%d mode=%d\n", i, a.Kind(), a.Source(), a.Target(), a.Mode())
	}
	s := sc.s
	zact := MakeAction(AkCast, TCode(zen), 0, 0)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == zen {
			zact = a
		}
	}
	b = fmt.Appendf(b, "zact=%d\n", zact)
	var anyD Decision
	s.plainOptions(s.k(), &anyD, availAny)
	b = fmt.Appendf(b, "anywalk n=%d\n", anyD.N)
	for i := 0; i < int(anyD.N); i++ {
		a := anyD.Options[i]
		b = fmt.Appendf(b, "  any[%d] kind=%d src=%d mode=%d\n", i, a.Kind(), a.Source(), a.Mode())
	}
	if zact != 0 {
		okf := s.filtScript(s.k(), zact, 0, false)
		b = fmt.Appendf(b, "filtScript ok=%v\n", okf)
	}
	// the hand-cast gates for the Zenith under the any walk
	kk := s.k()
	cz := s.objs[zen].card
	was := s.g.availX
	s.g.availX = availPlain | availAny
	var avz avail
	s.availFor(kk, 0, &avz)
	b = fmt.Appendf(b, "anyCensus total=%d n=%d\n", avz.total, avz.n)
	pc := &kk.cards[cz]
	cost := &pc.cost
	var fav2 avail
	cav := s.castGrantAvail(kk, 0, zen, &avz, &fav2)
	gate := s.castGateOrPoolPays(kk, 0, cz, zen, cost, cav)
	b = fmt.Appendf(b, "gate=%v\n", gate)
	if gate {
		b = fmt.Appendf(b, "tgt=%v costsFeas=%v gorgeOffers=%v xbound=%v\n",
			s.spellTargetsFeasibleFrom(kk, cz, 0, zen), s.spellCostsFeasible(kk, cz, 0, zen),
			s.gorgeOffers(0, zen, 0), s.spellXBoundPayable(kk, cz, 0, cost))
	}
	s.g.availX = was
	var av avail
	s.availFor(s.k(), 0, &av)
	for m2 := s.ctrlM[0]; m2.any(); m2 = m2.rest() {
		sl := m2.first()
		pc2 := s.manaPCGen(s.k(), sl)
		b = fmt.Appendf(b, "slot %d card=%d manaAb=%d manaCol=%#02x manaTap=%v chosen=%d manaChosen=%v tapped=%v\n",
			sl, s.cardAt(sl), pc2.manaAb, pc2.manaCol, pc2.manaTap, s.perms[sl].chosen, pc2.manaChosen, s.perms[sl].flags&pfTapped != 0)
	}
	b = fmt.Appendf(b, "census total=%d n=%d\n", av.total, av.n)
	for i := 0; i < av.n; i++ {
		b = fmt.Appendf(b, "  unit %d mask=%#02x cnt=%d\n", i, av.mask[i], av.cnt[i])
	}
	var av2 avail
	m := s.g.availX
	s.g.availX = m | availGaps
	s.availForMode(s.k(), 0, &av2, slotMask{}, true)
	s.g.availX = m
	b = fmt.Appendf(b, "census-capped total=%d n=%d\n", av2.total, av2.n)
	for i := 0; i < av2.n; i++ {
		b = fmt.Appendf(b, "  capped unit %d mask=%#02x cnt=%d\n", i, av2.mask[i], av2.cnt[i])
	}
	return b
}
