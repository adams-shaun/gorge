package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A waterbend unless cost the floating pool covers taps nothing: gorge's
// WaterbendTapsOwed is 0, so PayWaterbend(n, nil) only pays the pool (and the
// ward's need == 0 arm, rules/ward.go, likewise). The lockstep's resolutions
// never float mana, so this is the only test of that state.

func uwbPoolScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Waterbending Lesson", Count: 1}, {Name: "The Unagi of Kyoshi Island", Count: 1},
		{Name: "Ornithopter", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Island", Count: 1}, {Name: "Mountain", Count: 1},
	}
	tbl, _, _, err := cardsettest.Set(t, "unlesswaterbend").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for i := range tbl.Cards {
		id[tbl.Cards[i].Name] = CardID(i)
	}
	return newScenTableLib(t, tbl, id["Island"], Config{NoElide: true}), id
}

// uwbPayFromPool floats pool mana for p once the spell is on the stack (so
// the payable check sees it), answers pay at the unless / ward pay election
// (sub) and resolves the stack; it fails on a waterbend tap pick.
func (sc *scen) uwbPayFromPool(t *testing.T, sub ChoiceKind, p uint8, pool int) {
	t.Helper()
	asked, floated := false, false
	for i := 0; i < 80; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && sc.s.sp == 0 && sc.s.npend == 0 {
			break
		}
		switch {
		case d.Kind == DkPriority:
			if !floated {
				sc.s.pl[p].pool[kop.U], floated = uint8(pool), true
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind == DkChoose && (d.Sub == sub || sub == ChGenPayMode && d.Sub == ChGenPay):
			if d.N != 2 {
				t.Fatalf("pay election offers %d answers, want pay and decline", d.N)
			}
			if got := poolTotal(&sc.s.pl[p]); got != pool {
				t.Fatalf("{%d} floating at the pay election, want {%d}", got, pool)
			}
			asked = true
			sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
		case d.Kind == DkChoose && (d.Sub == ChWaterbend || d.Sub == ChWardWaterbend):
			t.Fatalf("a waterbend tap pick was asked with {%d} floating", pool)
		default:
			sc.s.Apply(d.Options[0])
		}
	}
	if !asked {
		t.Fatal("the pay election was never asked")
	}
}

func TestUnlessWaterbendPoolCoversTapsNothing(t *testing.T) {
	t.Run("Waterbending Lesson", func(t *testing.T) {
		sc, id := uwbPoolScen(t)
		sc.lands(0, id["Island"], 4)
		lesson := sc.hand(id["Waterbending Lesson"], 0)
		orn := sc.perm(id["Ornithopter"], 0)
		sc.castCard(t, lesson)
		sc.uwbPayFromPool(t, ChGenPayMode, 0, 2)
		if !sc.onBF(orn) || sc.s.perms[sc.slot(orn)].flags&pfTapped != 0 {
			t.Fatal("Ornithopter was tapped for a waterbend the pool covered")
		}
		if _, floating := sc.tappedAndFloating(0); floating != 0 {
			t.Fatalf("%d mana still floating, want the {2} paid from the pool", floating)
		}
		if n := sc.s.pl[0].handN; n != 3 {
			t.Fatalf("hand %d, want 3 (drew three, paid: no discard)", n)
		}
	})
	t.Run("ward", func(t *testing.T) {
		sc, id := uwbPoolScen(t)
		sc.lands(0, id["Mountain"], 1)
		shock := sc.hand(id["Shock"], 0)
		orn := sc.perm(id["Ornithopter"], 0)
		unagi := sc.perm(id["The Unagi of Kyoshi Island"], 1)
		sc.castCard(t, shock)
		if d := sc.s.Step(); d.Kind != DkPriority {
			sc.s.Apply(MakeAction(AkTarget, 0, TCode(unagi), 0))
		}
		sc.uwbPayFromPool(t, ChWardPay, 0, 4)
		if !sc.onBF(orn) || sc.s.perms[sc.slot(orn)].flags&pfTapped != 0 {
			t.Fatal("Ornithopter was tapped for a ward waterbend the pool covered")
		}
		if _, floating := sc.tappedAndFloating(0); floating != 0 {
			t.Fatalf("%d mana still floating, want the {4} paid from the pool", floating)
		}
		if sc.s.objs[shock].zone != kop.ZoneGraveyard || sc.s.perms[sc.slot(unagi)].damage != 2 {
			t.Fatalf("Shock zone %v, Unagi damage %d: want Shock resolved for 2 (the ward was paid)", sc.s.objs[shock].zone, sc.s.perms[sc.slot(unagi)].damage)
		}
	})
}
