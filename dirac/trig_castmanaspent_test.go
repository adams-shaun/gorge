package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kopusScen is a scenario over the opus pool with the named cards and
// basics.
func kopusScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Mountain", Count: 8}, {Name: "Island", Count: 8}}
	set := cardsettest.Set(t, "opus")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the opus table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Island"], Config{NoElide: true}), id
}

// castModeKopus casts o in the option whose mode bits include want.
func (sc *scen) castModeKopus(t *testing.T, o Obj, want uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) && a.Mode()&want == want {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("cast of %d (mode %d) not offered: %v", o, want, d.Options[:d.N])
}

// spellEntry is the spell o's stack entry's recorded spend.
func (sc *scen) spentOf(t *testing.T, o Obj) int {
	t.Helper()
	for i := int(sc.s.sp) - 1; i >= 0; i-- {
		if e := &sc.s.stack[i]; e.kind == seSpell && e.obj == o {
			return int(e.trigAmt)
		}
	}
	t.Fatal("the spell is not on the stack")
	return 0
}

// TestOpusSpent: the spend a reader snapshots is the pool mana the cast
// spent (gorge manaSpentTotal): X included, a cost reduction taken off, a
// kicker added. (No opus card has convoke: histCast's convoke subtraction
// is the Mockingbird path, dirac/oracle convoke lockstep.)
func TestOpusSpent(t *testing.T) {
	sc, id := kopusScen(t, "Shantotto, Tactician Magician", "Crater's Claws", "Burst Lightning", "Wizard's Lightning", "Lightning Bolt")
	sc.lands(0, id["Mountain"], 8)
	sc.perm(id["Shantotto, Tactician Magician"], 0)
	for _, c := range []struct {
		name string
		mode uint8
		x    int
		want int
	}{
		{"Wizard's Lightning", 0, -1, 1}, // {2}{R}, {2} less with a Wizard in play
		{"Lightning Bolt", 0, -1, 1},
		{"Burst Lightning", 0, -1, 1},
		{"Burst Lightning", 1, -1, 5}, // kicked: {R} + {4}
		{"Crater's Claws", 0, 3, 4},   // {X}{R}, X = 3
	} {
		o := sc.hand(id[c.name], 0)
		sc.castModeKopus(t, o, c.mode)
		if c.x >= 0 {
			sc.chooseX(t, uint8(c.x))
		}
		for i := 0; i < 20; i++ { // targets, kicker, payment: until the cast is done
			d := sc.s.Step()
			if d.Kind == DkPriority {
				break
			}
			sc.s.Apply(d.Options[0])
		}
		if got := sc.spentOf(t, o); got != c.want {
			t.Errorf("%s mode %d x %d: recorded spend %d, want %d", c.name, c.mode, c.x, got, c.want)
		}
		sc.resolveAll(t)
		for m := sc.s.occ; m.any(); m = m.rest() {
			sc.s.perms[m.first()].flags &^= pfTapped
		}
	}
}

// TestOpusPlannerDecline: while a mana-spent reader is on either
// battlefield gorge's payment planner prices no cast ("shape:mana_spent_
// reader"): FeatPlannedCast is false for both players, true again once it
// leaves; a face-down reader still declines (gorge's Object.Face() ignores
// face-down status).
func TestOpusPlannerDecline(t *testing.T) {
	sc, id := kopusScen(t, "Shantotto, Tactician Magician", "Lightning Bolt", "Counterspell")
	sc.lands(0, id["Mountain"], 3)
	sc.lands(1, id["Island"], 3)
	bolt := sc.hand(id["Lightning Bolt"], 0)
	cs := sc.hand(id["Counterspell"], 1)
	both := func(want bool, why string) {
		t.Helper()
		if got := sc.s.FeatPlannedCast(0, bolt, 0); got != want {
			t.Errorf("%s: player 0 planned = %v, want %v", why, got, want)
		}
		if got := sc.s.FeatPlannedCast(1, cs, 0); got != want {
			t.Errorf("%s: player 1 planned = %v, want %v", why, got, want)
		}
	}
	both(true, "no reader")
	sh := sc.perm(id["Shantotto, Tactician Magician"], 0)
	both(false, "reader on player 0's battlefield")
	sc.s.leave(sc.k, sc.slot(sh), kop.ZoneGraveyard, false)
	both(true, "reader gone")
	sh2 := sc.perm(id["Shantotto, Tactician Magician"], 1)
	both(false, "reader on player 1's battlefield")
	sc.s.leave(sc.k, sc.slot(sh2), kop.ZoneGraveyard, false)
	both(true, "reader gone again")
	fd := sc.hand(id["Shantotto, Tactician Magician"], 0)
	sc.s.enterFaceDown(sc.k, fd, 0, kop.MorphMorph)
	both(false, "face-down reader")
}
