package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// handWalkFixture seats Mm'menon (granting {T}: Add {U} to artifacts, barred
// from a hand cast) and an artifact, with Coral Merfolk in hand.
// mutate sets the alternative cost fields on the matched table's CardDef;
// it returns the scenario, the hand object, the card and the alt cost.
func handWalkFixture(t *testing.T, mutate func(cc *kop.CardDef, cost kop.ManaCost)) (*scen, Obj, CardID, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Mm'menon, the Right Hand", Count: 1},
		{Name: "Ornithopter", Count: 1},
		{Name: "Coral Merfolk", Count: 1},
		{Name: "Island", Count: 10},
	}
	set := cardsettest.Set(t, "wascastspell")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in wascastspell", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.perm(id["Mm'menon, the Right Hand"], 0)
	sc.perm(id["Ornithopter"], 0)
	cardC := id["Coral Merfolk"]
	cand := sc.hand(cardC, 0)
	var u kop.ManaCost
	u.Pips[kop.U] = 1
	mutate(&tbl.Cards[cardC], u)
	sc.k.cards[cardC].altCost = true
	return sc, cand, cardC, id
}

// handWalkCheck asserts the unfiltered walk pays cost, the filtered one does
// not, offer lists nothing with the grant only, and lists it with an Island.
func handWalkCheck(t *testing.T, sc *scen, cand Obj, cardC CardID, id map[string]CardID, cost kop.ManaCost, og payDesc, kind ActionKind, offer func(d *Decision, av *avail)) {
	t.Helper()
	s := sc.s
	s.g.availX = availPlain | availWalk
	var av avail
	s.availFor(sc.k, 0, &av)
	if !av.affords(&cost, pipMaskOf(&cost), 0) {
		t.Fatalf("precondition: the unfiltered walk does not pay the cost")
	}
	var f avail
	s.walkHandAvail(sc.k, 0, og, &f)
	if f.affords(&cost, pipMaskOf(&cost), 0) {
		t.Fatalf("precondition: the filtered walk still pays the cost")
	}
	s.g.availX = 0
	listed := func(d *Decision) bool {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == kind && d.Options[i].Source() == TCode(uint8(cand)) {
				return true
			}
		}
		return false
	}
	var d Decision
	s.g.availX = availPlain | availWalk
	offer(&d, &av)
	s.g.availX = 0
	if listed(&d) {
		t.Fatalf("offered though only the hand-barred grant pays it: %v", d.Options[:d.N])
	}
	sc.perm(id["Island"], 0)
	var d2 Decision
	s.g.availX = availPlain | availWalk
	var av2 avail
	s.availFor(sc.k, 0, &av2)
	offer(&d2, &av2)
	s.g.availX = 0
	if !listed(&d2) {
		t.Fatalf("control: not offered with a printed Island: %v", d2.Options[:d2.N])
	}
}

func TestHandWalkExcludesHandBarredEvokeGrant(t *testing.T) {
	sc, cand, cardC, id := handWalkFixture(t, func(cc *kop.CardDef, u kop.ManaCost) { cc.HasEvoke, cc.EvokeCost = true, u })
	spell := payDesc{class: pdSpell, card: cardC, src: NoSlot, og: sc.s.restrOriginOfObj(cand)}
	handWalkCheck(t, sc, cand, cardC, id, sc.k.t.Cards[cardC].EvokeCost, spell, AkCast, func(d *Decision, av *avail) {
		sc.s.offerEvokeHand(sc.k, d, 0, cand, cardC, av)
	})
}

func TestHandWalkExcludesHandBarredImpendingGrant(t *testing.T) {
	sc, cand, cardC, id := handWalkFixture(t, func(cc *kop.CardDef, u kop.ManaCost) { cc.ImpendingCost = u })
	spell := payDesc{class: pdSpell, card: cardC, src: NoSlot, og: sc.s.restrOriginOfObj(cand)}
	handWalkCheck(t, sc, cand, cardC, id, sc.k.t.Cards[cardC].ImpendingCost, spell, AkCast, func(d *Decision, av *avail) {
		sc.s.offerImpendingHand(sc.k, d, 0, cand, cardC, av)
	})
}

func TestHandWalkExcludesHandBarredPlotGrant(t *testing.T) {
	sc, cand, cardC, id := handWalkFixture(t, func(cc *kop.CardDef, u kop.ManaCost) { cc.HasPlot, cc.PlotCost = true, u })
	other := payDesc{class: pdOther, src: NoSlot}
	handWalkCheck(t, sc, cand, cardC, id, sc.k.t.Cards[cardC].PlotCost, other, AkPlot, func(d *Decision, av *avail) {
		sc.s.offerPlotHand(sc.k, d, 0, cand, cardC, av)
	})
}
