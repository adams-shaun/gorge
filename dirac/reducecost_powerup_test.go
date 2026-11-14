package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// powerUpReducerScen builds a scenario over the power-up pool with a
// hand-built reducer shaped like Hulk, Gamma Goliath ("power-up abilities of
// other creatures you control cost {3} less") and a hand-built creature with
// both a Power-up ability and an ordinary activated ability.
func powerUpReducerScen(t *testing.T) (*scen, Obj, Obj, Obj, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}, {Name: "Brave Brawler", Count: 1}}
	set := cardsettest.Set(t, "powerup")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	pl, ok := id["Plains"]
	if !ok {
		t.Fatal("scenario needs Plains")
	}
	// The reducer's ValidCard$ Creature.Other+YouCtrl, as Hulk's static reads.
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags: kop.FOther})
	reducer := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Powerup Reducer", Types: kop.TCreature, Power: 6, Toughness: 5,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatReduceAbility, Filter: kop.FilterID(len(tbl.Filters) - 1), A: 3, B: kop.ReduceAbPowerUp}}},
	})
	// The Power-up marker is a table flag, not State: an ability with
	// CostPowerUp is the source class the reduction keys on.
	two := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Two Abilities", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{
			{Kind: kop.AbActivated, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 4, Pips: [kop.NumManaKinds]uint8{kop.R: 1}}, Flags: kop.CostPowerUp}},
			{Kind: kop.AbActivated, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 2}}},
		},
	})
	sc := newScenTableLib(t, tbl, pl, Config{NoElide: true})
	return sc, sc.perm(reducer, 0), sc.perm(two, 0), sc.perm(id["Brave Brawler"], 0), two
}

// TestReduceAbilityPowerUpOnly: a Power-up-only StatReduceAbility reaches a
// Power-up activation and no other activated ability of the same source. The
// power-up of another creature costs {1}{W} rather than {4}{W}, while the
// ordinary activated ability of the hand-built source keeps its printed {2}.
func TestReduceAbilityPowerUpOnly(t *testing.T) {
	sc, reducer, src, brawler, two := powerUpReducerScen(t)
	if !sc.onBF(reducer) || !sc.onBF(src) || !sc.onBF(brawler) {
		t.Fatal("the scenario did not put every permanent onto the battlefield")
	}
	// The reducer is on the battlefield and matches every other creature.
	var static bool
	for i := range sc.k.t.Cards[sc.s.objs[reducer].card].Abilities {
		a := &sc.k.t.Cards[sc.s.objs[reducer].card].Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceAbility {
			static = true
			if a.Static.B&kop.ReduceAbPowerUp == 0 {
				t.Fatalf("the reducer's B %#x lacks ReduceAbPowerUp", a.Static.B)
			}
		}
	}
	if !static {
		t.Fatal("the reducer has no StatReduceAbility static")
	}
	// Neither creature entered this turn, so only the reducer can lower the
	// power-up: {4}{W} becomes {1}{W}.
	sc.nextTurn()
	sc.s.pl[0].pool[kop.W] = 2
	d := sc.s.Step()
	if !powerupActivation(&d, brawler) {
		t.Fatalf("Brave Brawler's power-up not offered from two white (printed {4}{W}): %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkActivate, TCode(brawler), 0, 0))
	d = sc.s.Step()
	if d.Kind != DkPriority {
		t.Fatalf("after the power-up activation: decision kind %d", d.Kind)
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	if n := sc.s.perms[sc.slot(brawler)].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("Brave Brawler has %d +1/+1 counters, want 2", n)
	}
	if got := sc.s.pl[0].pool[kop.W]; got != 0 {
		t.Fatalf("the reduced {1}{W} charge left %d white in the pool", got)
	}

	// The same source's Power-up ability is lowered; its ordinary ability is
	// not.
	cd := &sc.k.t.Cards[two]
	powerUp, other := &cd.Abilities[0], &cd.Abilities[1]
	if powerUp.Cost.Flags&kop.CostPowerUp == 0 || other.Cost.Flags&kop.CostPowerUp != 0 {
		t.Fatalf("hand-built abilities cost flags %#x and %#x, want Power-up then ordinary", powerUp.Cost.Flags, other.Cost.Flags)
	}
	sl := sc.slot(src)
	got := sc.s.abReduced(sc.k, powerUp, sl, 0, powerUp.Cost.Mana)
	want := powerUp.Cost.Mana
	want.Generic -= 3
	if got != want {
		t.Fatalf("the source's power-up priced %v, want %v (printed %v less {3})", got, want, powerUp.Cost.Mana)
	}
	if kept := sc.s.abReduced(sc.k, other, sl, 0, other.Cost.Mana); kept != other.Cost.Mana {
		t.Fatalf("the source's ordinary ability priced %v, want its printed %v", kept, other.Cost.Mana)
	}
}
