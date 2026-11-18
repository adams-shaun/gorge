package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// addTapBeside appends a plain {T}: Add {R} mana ability beside c's mana
// abilities, so a sole life-only source (Treasonous Ogre, prepSoleLifeMana's
// pain census unit) becomes the mixed card the census gap stays for
// (kop.GapLifeManaAb without kop.SoleGapLifeManaAb).
func addTapBeside(tbl *kop.Table, c CardID) {
	cd := &tbl.Cards[c]
	cd.Abilities = append(cd.Abilities, kop.Ability{Kind: kop.AbMana, ManaColors: kop.ColR, ManaN: 1, Cost: kop.Cost{Flags: kop.CostTap}})
}

// Two mixed Treasonous Ogres (each with a {T}: Add {R} beside its life
// ability, so the life ability stays a census gap) at 4 life cannot pay two
// 3-life activations. The gaps walk lists one unit per source against the
// shared life total, so the {2}{R} Mind Stone reaches the prefix search; the
// search must fail the second activation's branch and backtrack, not tap the
// source as an ordinary gap. The cost is {2}{R}, not {2}: the two {T}
// abilities are census units whose two {R} alone cannot pay three mana, so
// the play still reaches the search. tapGap would call eachFee, which
// indexes the Ogre's missing each-colour ability (-1) and panics.
func TestLifeGapSharedBudgetPrunesBranch(t *testing.T) {
	deck := []cardset.Entry{{Name: "Treasonous Ogre", Count: 2}, {Name: "Mind Stone", Count: 1}, {Name: "Mountain", Count: 17}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	addTapBeside(tbl, id["Treasonous Ogre"])
	tbl.Cards[id["Mind Stone"]].Cost = kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{3: 1}} // {2}{R}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true, GorgeLowering: true})
	o1 := sc.perm(id["Treasonous Ogre"], 0)
	o2 := sc.perm(id["Treasonous Ogre"], 0)
	stone := sc.hand(id["Mind Stone"], 0)
	s, k := sc.s, sc.k
	s.pl[0].life = 4
	sl1, sl2 := sc.slot(o1), sc.slot(o2)
	if !s.lifeGapMask(k, 0).has(sl1) || !s.lifeGapMask(k, 0).has(sl2) {
		t.Fatalf("precondition: the two Ogres are not both payable life gaps at 4 life (mask %#x)", s.lifeGapMask(k, 0))
	}
	if mv := k.t.Cards[id["Mind Stone"]].Cost.MV(); mv != 3 {
		t.Fatalf("precondition: Mind Stone costs {%d}, want a three-mana play the two {T} taps cannot pay", mv)
	}
	a := MakeAction(AkCast, TCode(stone), 0, 0)
	if !s.scriptDeadEndAny(k, a, 0, slotMask{}, slotMask{}, slotMask{}, slotMask{}, slotMask{}) {
		t.Fatal("the {2} cast with two Ogres at 4 life is not a dead script; the search paid an unpayable second activation")
	}
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if o := d.Options[i]; o.Kind() == AkCast && o.Source() == TCode(stone) {
			t.Fatalf("the {2} cast is offered with one payable Ogre activation: opts %v", d.Options[:d.N])
		}
	}
}

// A life-only ability that makes one mana of any colour takes its answer from
// gorge's scriptOrderAnswers order, which puts the play's announced colours
// first. The Ogre rewritten to any colour, with a {T}: Add {R} beside it so
// the life ability stays a census gap: answer 0 of a {G} cast must float {G},
// not engine order's {W}.
func TestLifeGapAnyColourAnswerFollowsPlayNeed(t *testing.T) {
	deck := []cardset.Entry{{Name: "Treasonous Ogre", Count: 1}, {Name: "Rancor", Count: 1}, {Name: "Mountain", Count: 18}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	rewriteMana(tbl, id["Treasonous Ogre"], func(a *kop.Ability) { a.ManaColors = kop.ColAny })
	addTapBeside(tbl, id["Treasonous Ogre"])
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true, GorgeLowering: true})
	ogre := sc.perm(id["Treasonous Ogre"], 0)
	bears := sc.hand(id["Rancor"], 0)
	s, k := sc.s, sc.k
	s.pl[0].life = 3
	sl := sc.slot(ogre)
	if a, ok := s.lifeGapSource(k, sl); !ok || a.ManaColors != kop.ColAny {
		t.Fatalf("precondition: the rewritten Ogre is not an any-colour life gap (ok %v)", ok)
	}
	if n := s.lifeGapAnswers(k, sl); n != 5 {
		t.Fatalf("precondition: %d colour answers, want 5 (W U B R G)", n)
	}
	a := MakeAction(AkCast, TCode(bears), 0, 0)
	f := s.fork()
	var spill [kop.NumManaKinds]uint8
	ok := f.scriptGapTap(k, 0, a, sl, slotMask{}, slotMask{}, slotMask{}, slotMask{}, slotMask{}, 0, &spill)
	if !ok {
		t.Fatal("the payable life gap failed to tap")
	}
	if f.pl[0].pool[kop.G] != 1 {
		t.Fatalf("answer 0 floated pool %v, want one {G} (the play's needed colour first)", f.pl[0].pool)
	}
	s.unfork()
}
