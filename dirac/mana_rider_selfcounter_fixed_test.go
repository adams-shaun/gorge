package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestManaRiderSelfCounterFixedKernel: the kernel's counter-rider arm runs a
// fixed-colour and a finite-Combo rider (Vulshok Factory's "{T}: Add {R}.
// Put a charge counter on CARDNAME", Land Cap's "{T}: Add {W} or {U}. Put a
// depletion counter on CARDNAME", kgen ctrRiderMana's widened shape) exactly
// as the any-colour one (Twitching Doll): a payment whose only source is the
// card taps it as the deferred gap and the tap puts its counter on it with
// the mana (CR 605.3b), the card staying on the battlefield.
func TestManaRiderSelfCounterFixedKernel(t *testing.T) {
	run := func(t *testing.T, card string, ctr kop.CounterKind, col int) (tapped bool, n int) {
		deck := []cardset.Entry{{Name: card, Count: 1}, {Name: "Island", Count: 19}}
		tbl, _, _, err := cardsettest.Set(t, "manarider").Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for c := range tbl.Cards {
			id[tbl.Cards[c].Name] = CardID(c)
		}
		src := id[card]
		if _, _, ok := cardManaRider(&tbl.Cards[src]); !ok {
			t.Fatalf("precondition: %s's mana ability is not the counter-rider shape", card)
		}
		sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
		if !sc.k.gen.anyManaRider {
			t.Fatal("precondition: the table has no counter-rider source")
		}
		o := sc.perm(src, 0)
		if !sc.s.planDeferred(sc.k, 0, sc.slot(o)) || !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(o)) {
			t.Fatalf("precondition: %s is not a deferred source and prefix gap", card)
		}
		if n := sc.s.perms[sc.slot(o)].ctrN(sc.s, int(ctr)); n != 0 {
			t.Fatalf("precondition: %s entered with %d counters", card, n)
		}
		c := cost(0, col)
		sc.s.tapFor(sc.k, 0, &c, 0, NoObj)
		return sc.tapped(o), sc.s.perms[sc.slot(o)].ctrN(sc.s, int(ctr))
	}
	t.Run("vulshok factory", func(t *testing.T) {
		if tapped, n := run(t, "Vulshok Factory", kop.CtrCharge, kop.R); !tapped || n != 1 {
			t.Fatalf("tapped %v, charge counters %d: want it tapped with one charge counter", tapped, n)
		}
	})
	t.Run("land cap", func(t *testing.T) {
		if tapped, n := run(t, "Land Cap", kop.CtrDepletion, kop.W); !tapped || n != 1 {
			t.Fatalf("tapped %v, depletion counters %d: want it tapped with one depletion counter", tapped, n)
		}
		if tapped, n := run(t, "Land Cap", kop.CtrDepletion, kop.U); !tapped || n != 1 {
			t.Fatalf("{U}: tapped %v, depletion counters %d: want it tapped with one depletion counter", tapped, n)
		}
	})
}
