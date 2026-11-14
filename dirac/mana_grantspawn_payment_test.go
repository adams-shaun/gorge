package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// spawnPairScen seats seat 0's Path of Annihilation beside two Eldrazi Spawn
// tokens, the lower ObjID entering first as lockstep pair 10's #103/#104 do.
// Nothing else is on the battlefield, so the Spawns' printed sacrifices are
// the only printed sources and no {W} pip has one.
func spawnPairScen(t *testing.T, cfg Config) (*scen, Obj, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Path of Annihilation", Count: 4}, {Name: "Destroy Evil", Count: 4},
		{Name: "Goblin Taskmaster", Count: 4}, {Name: "Larger Than Life", Count: 4},
		{Name: "Exquisite Huntmaster", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Island", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 6}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tok, ok := CardID(0), false
	for c := range tbl.Cards { // the token's script sits beside its carrier
		if tbl.Cards[c].Name == "c_0_1_eldrazi_spawn_sac" {
			tok, ok = CardID(c), true
			break
		}
	}
	if !ok {
		t.Fatal("the Spawn token is not in the mbc matchup table")
	}
	sc := newScenTable(t, tbl, cfg)
	sc.perm(tableID(t, tbl, "Path of Annihilation"), 0)
	var objs [2]Obj
	for i := range objs {
		objs[i] = sc.obj(tok, 0)
		sc.s.objs[objs[i]].of |= ofToken
		sl := sc.s.enter(sc.k, objs[i], 0, false, kop.ZoneNone)
		sc.s.perms[sl].flags &^= pfSick
	}
	sc.s.npend = 0
	return sc, objs[0], objs[1]
}

func tableID(t *testing.T, tbl *kop.Table, name string) CardID {
	t.Helper()
	for c := range tbl.Cards {
		if tbl.Cards[c].Name == name {
			return CardID(c)
		}
	}
	t.Fatalf("%s not in the mbc matchup table", name)
	return 0
}

// TestSpawnGrantPaymentChoice: Destroy Evil's {1}{W} beside two Spawns under
// Path of Annihilation has no printed {W} source, so gorge pays it with a
// Spawn's granted tap for the {W} and the other Spawn's printed sacrifice for
// the {1}. Its offer's prefix search (potential_script.go) walks the census
// gaps in battlefield order: the first Spawn's step sacrifices it for {C}
// (CR 605.1a, a printed sacrifice-only ability beside a tap grant) and the
// second Spawn's grant makes the {W}. Lockstep pair 10 paid #103 (the first)
// by sacrifice and #104 by granted tap, and the kernel must pick the same
// permanents, not the other way round.
func TestSpawnGrantPaymentChoice(t *testing.T) {
	sc, first, second := spawnPairScen(t, Config{NoElide: true, GorgeLowering: true})
	// the premise: both tokens are on the battlefield, untapped and able to
	// tap, and the cast is offered
	for _, o := range []Obj{first, second} {
		if !sc.onBF(o) || sc.s.perms[sc.slot(o)].flags&(pfTapped|pfSick) != 0 {
			t.Fatalf("a Spawn is not an untapped battlefield permanent before the cast")
		}
	}
	if first >= second {
		t.Fatalf("the first-entered token has ObjID %d, not below the second's %d", first, second)
	}
	de := sc.hand(tableID(t, sc.k.t, "Destroy Evil"), 0)
	sc.do(t, AkCast, TCode(uint8(de)), scAny)
	sc.settle(t)
	if sc.onBF(first) {
		t.Errorf("the first Spawn stayed on the battlefield, want it sacrificed for the {1}")
	}
	if !sc.onBF(second) {
		t.Fatalf("the second Spawn left the battlefield, want it alive with its granted tap used for the {W}")
	}
	if sc.s.perms[sc.slot(second)].flags&pfTapped == 0 {
		t.Errorf("the second Spawn is untapped, want its granted tap used for the {W}")
	}
}

// settle answers the cast's remaining asks (a target, then the payment) with
// their first option until priority returns: the prefix search floats the
// granted mana at the announcement, and the payment spends it at the asks that
// follow. The bound keeps a loop from hanging the test.
func (sc *scen) settle(t *testing.T) {
	t.Helper()
	for i := 0; i < 8; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority || d.N == 0 {
			return
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("the cast's asks never returned to priority")
}
