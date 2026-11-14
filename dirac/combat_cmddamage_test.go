package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// cmdVanillaScen is a Commander scenario over the hand-built vanilla table
// with a synthetic power-7 commander; no corpus is needed.
func cmdVanillaScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Commander", Types: kop.TCreature, Power: 7, Toughness: 7})
	return newScenTable(t, &tbl, Config{NoElide: true, Commander: true}), CardID(len(tbl.Cards) - 1)
}

// cmdPlstatScen is a Commander scenario over the plstat fixture pool, which
// carries Absolute Virtue (protection from each opponent), plus the same
// synthetic power-7 commander.
func cmdPlstatScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	set, err := cardset.ByName("plstat")
	if err != nil {
		t.Skip(err)
	}
	set = cardset.AllowUnverified(set)
	deck := []cardset.Entry{{Name: "Absolute Virtue", Count: 1}, {Name: "Plains", Count: 20}}
	if _, ok := set.Lookup("Plains"); !ok {
		t.Fatal("Plains not in the plstat pool")
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Commander", Types: kop.TCreature, Power: 7, Toughness: 7})
	id["Test Commander"] = CardID(len(tbl.Cards) - 1)
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, Commander: true}), id
}

// cmdParkedScen is a Commander scenario over the FDN table for the
// replacement-order ask (two Twinflame Tyrants).
func cmdParkedScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, Commander: true}), id
}

// nextMain1 starts another seat-0 main phase and untaps the battlefield, so
// a test can run a second real combat without the full turn machinery.
func (sc *scen) nextMain1() {
	sc.s.turn++
	sc.s.newTurnGen()
	sc.s.active = 0
	sc.s.step = kop.StepMain1
	sc.s.prio = 0
	sc.s.passes = 0
	sc.s.carry = carryNone
	sc.s.ctl = ctlPriority
	for m := sc.s.occ; m.any(); m = m.rest() {
		sc.s.perms[m.first()].flags &^= pfTapped
	}
}

// TestCommanderDamageLoses: a power-7 commander connects three times through
// real combat damage steps; the third takes the defender out with life above
// 0. The tally grows by the damage actually dealt each time.
func TestCommanderDamageLoses(t *testing.T) {
	sc, cmdrCard := cmdVanillaScen(t)
	cmdr := sc.perm(cmdrCard, 0)
	if sc.s.cfg&cfgCommander == 0 {
		t.Fatal("scenario is not a commander game")
	}
	sc.s.pl[0].cmd = cmdr
	for hit := 1; hit <= 3; hit++ {
		sc.combat(t, []Obj{cmdr}, nil)
		if got, want := sc.s.pl[1].cmdDmg[0], uint8(7*hit); got != want {
			t.Fatalf("after hit %d: tally %d, want %d", hit, got, want)
		}
		if hit < 3 {
			if sc.s.pl[1].lost {
				t.Fatalf("lost after %d hits (%d commander damage)", hit, sc.s.pl[1].cmdDmg[0])
			}
			if got, want := sc.s.pl[1].life, int16(40-7*hit); got != want {
				t.Fatalf("after hit %d: life %d, want %d", hit, got, want)
			}
			sc.nextMain1()
		}
	}
	if !sc.s.pl[1].lost {
		t.Fatal("the defender did not lose at 21 commander damage")
	}
	if got := sc.s.pl[1].life; got != 19 {
		t.Fatalf("after the loss: life %d, want 19 (above 0)", got)
	}
}

// TestCommanderDamagePrevented: real protection from each opponent prevents
// the commander's combat damage, and noncombat damage the commander deals
// does not count. A control run without the protection shows the same hit
// would otherwise tally.
func TestCommanderDamagePrevented(t *testing.T) {
	sc, cmdrCard := cmdVanillaScen(t)
	cmdr := sc.perm(cmdrCard, 0)
	sc.s.pl[0].cmd = cmdr
	sc.combat(t, []Obj{cmdr}, nil)
	if got := sc.s.pl[1].cmdDmg[0]; got != 7 {
		t.Fatalf("control: tally %d after a 7-power hit, want 7", got)
	}

	sc, id := cmdPlstatScen(t)
	cmdr = sc.perm(id["Test Commander"], 0)
	sc.s.pl[0].cmd = cmdr
	sc.perm(id["Absolute Virtue"], 1)
	sc.combat(t, []Obj{cmdr}, nil)
	if got := sc.s.pl[1].cmdDmg[0]; got != 0 {
		t.Fatalf("protected: tally %d, want 0", got)
	}
	if got := sc.s.pl[1].life; got != 40 {
		t.Fatalf("protected: life %d, want 40 (the damage was prevented)", got)
	}

	sc, cmdrCard = cmdVanillaScen(t)
	pyro := sc.perm(v.ProdigalPyromancer, 0)
	sc.s.pl[0].cmd = pyro
	sc.do(t, AkActivate, TCode(pyro), scAny)
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
	sc.passBoth(t)
	if got := sc.s.pl[1].cmdDmg[0]; got != 0 {
		t.Fatalf("noncombat: tally %d, want 0", got)
	}
	if got := sc.s.pl[1].life; got != 39 {
		t.Fatalf("noncombat: life %d, want 39 (the ping was dealt)", got)
	}
}

// TestCommanderDamageParkedOrder: two Twinflame Tyrants (one the attacking
// commander) pose a real CR 616.1 replacement-order ask and land 12 damage.
// The tally is taken where the ordered walk lands the event, not once per
// recomputed pass or before the walk: 12, not 24.
func TestCommanderDamageParkedOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 2}, {Name: "Plains", Count: 20}}
	sc, id := cmdParkedScen(t, deck)
	cmdr := sc.perm(id["Twinflame Tyrant"], 0)
	sc.perm(id["Twinflame Tyrant"], 0)
	sc.s.pl[0].cmd = cmdr
	sc.combat(t, []Obj{cmdr}, nil)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.N < 2 {
		t.Fatalf("want the replacement-order ask, got kind %d sub %d opts %v", d.Kind, d.Sub, d.Options[:d.N])
	}
	if got := sc.s.pl[1].cmdDmg[0]; got != 0 {
		t.Fatalf("tally %d before the ordered walk, want 0", got)
	}
	sc.s.Apply(d.Options[0])
	sc.passUntil(t, kop.StepMain2)
	if got := sc.s.pl[1].cmdDmg[0]; got != 12 {
		t.Fatalf("tally %d after the ordered walk, want 12", got)
	}
	if got := sc.s.pl[1].life; got != 28 {
		t.Fatalf("life %d after the ordered walk, want 28", got)
	}
	if sc.s.pl[1].lost {
		t.Fatal("lost to 12 commander damage")
	}
}
