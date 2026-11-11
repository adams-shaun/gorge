package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// partnerTestCommanders are each seat's partner pair, deck indices in Config
// order. Seat 1 names the later card first, so the kernel must find both
// cards before either leaves the library.
var partnerTestCommanders = [2][2]int16{{37, 80}, {62, 10}}

// partnerGorgeGenesis deals 100-card decks whose partner-pair indices hold
// two legendary Partner creatures in gorge, and returns per seat the library,
// hand and command zone as deck indices (command zone in zone order), the
// per-commander bookkeeping sizes, and the RNG draws.
func partnerGorgeGenesis(t *testing.T) (lib, hand, cmd [2][]int, casts, dmg [2]int, draws uint64) {
	t.Helper()
	land := cmdTestCard(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
	pa := cmdTestCard(t, "Name:Partner Alpha\nManaCost:1 G\nTypes:Legendary Creature Elf\nPT:2/2\nK:Partner\nOracle:x\n")
	pb := cmdTestCard(t, "Name:Partner Beta\nManaCost:1 G\nTypes:Legendary Creature Elf\nPT:2/2\nK:Partner\nOracle:x\n")
	decks := make([][]*cards.Card, 2)
	cfg := rules.Config{Seed: cmdTestSeed, Names: []string{"a", "b"}, Format: rules.FormatCommander, StartingLife: 40}
	for p := range decks {
		decks[p] = make([]*cards.Card, 100)
		for i := range decks[p] {
			decks[p][i] = land
		}
		ix := partnerTestCommanders[p]
		decks[p][ix[0]], decks[p][ix[1]] = pa, pb
		cfg.Commanders = append(cfg.Commanders, []int{int(ix[0]), int(ix[1])})
	}
	cfg.Decks = decks
	e := rules.New(cfg)
	for p := 0; p < 2; p++ {
		pid := state.PlayerID(p)
		base := state.ObjID(0)
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZCommand} {
			for _, id := range e.G.Zone(z, pid) {
				if base == 0 || id < base {
					base = id
				}
			}
		}
		idx := func(z state.Zone) []int {
			var out []int
			for _, id := range e.G.Zone(z, pid) {
				out = append(out, int(id-base))
			}
			return out
		}
		lib[p], hand[p], cmd[p] = idx(state.ZLibrary), idx(state.ZHand), idx(state.ZCommand)
		casts[p], dmg[p] = len(e.G.Players[p].CmdCasts), len(e.G.Players[p].CmdDamage)
	}
	return lib, hand, cmd, casts, dmg, e.RNGDraws()
}

// partnerScen is newCommanderScen with seat 0 seating deck cards 0 and 1 as
// a partner pair of the same {2}{G} fixture commander, seat 1 none.
func partnerScen(t *testing.T) *scen {
	t.Helper()
	tbl, card := commanderTable(0)
	var deck [20]CardID
	for i := range deck {
		deck[i] = v.Forest
	}
	deck[0], deck[1] = card, card
	cs := noCommanders()
	cs[0] = [2]int16{0, 1}
	s := new(State)
	s.Init(tbl, deck[:], deck[:], 1, Config{NoElide: true, Commander: true, Commanders: cs})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}
}

// partnerCastOffered reports whether p's priority walk offers a cast of o.
func partnerCastOffered(sc *scen, p uint8, o Obj) bool {
	sc.s.prio = p
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == uint8(o) {
			return true
		}
	}
	return false
}

// TestCommanderPartnerGenesis: a two-commander deck seats both commanders in
// the command zone at genesis, in gorge's deal (library, hand, command zone
// order, RNG draws); each commander pays its own tax (CR 903.8) and carries
// its own commander-damage tally (CR 903.10a).
func TestCommanderPartnerGenesis(t *testing.T) {
	t.Run("genesis", func(t *testing.T) {
		d := cmdTestDecks()
		cs := noCommanders()
		for p := range partnerTestCommanders {
			cs[p] = partnerTestCommanders[p]
			for _, ix := range cs[p] {
				d[p][ix] = v.GrizzlyBears
			}
		}
		s := NewConfig(v.Table, d[0], d[1], cmdTestSeed, Config{NoElide: true, Commander: true, Commanders: cs})
		gLib, gHand, gCmd, gCasts, gDmg, gDraws := partnerGorgeGenesis(t)
		lib, hand := diracGenesis(s)
		for p := uint8(0); p < 2; p++ {
			// the gorge reference seated the pair: two commanders, a cast
			// count each, a damage slot for each of the match's four
			ix := partnerTestCommanders[p]
			if !sameInts(gCmd[p], []int{int(ix[0]), int(ix[1])}) || gCasts[p] != 2 || gDmg[p] != 4 {
				t.Fatalf("seat %d: gorge command zone %v casts %d damage slots %d, want %v 2 4", p, gCmd[p], gCasts[p], gDmg[p], ix)
			}
			for j := uint8(0); j < 2; j++ {
				want := Obj(int(p)*100 + int(ix[j]))
				if got := s.FeatCommanderN(p, j); got != want || s.objs[want].zone != kop.ZoneCommand {
					t.Fatalf("seat %d slot %d: commander %d zone %v, want object %d in the command zone", p, j, got, s.objs[want].zone, want)
				}
				if s.FeatCommanderCastN(p, j) != 0 {
					t.Fatalf("seat %d slot %d: casts not zero at genesis", p, j)
				}
			}
			if pl := &s.pl[p]; pl.libLen() != 98-7 || pl.handN != 7 || pl.life != 40 {
				t.Fatalf("seat %d: library %d hand %d life %d, want 91 7 40", p, pl.libLen(), pl.handN, pl.life)
			}
			if !sameInts(lib[p], gLib[p]) || !sameInts(hand[p], gHand[p]) {
				t.Fatalf("seat %d: library/hand differ from gorge\nkernel lib %v hand %v\ngorge  lib %v hand %v", p, lib[p], hand[p], gLib[p], gHand[p])
			}
		}
		if s.RNGDraws() != gDraws {
			t.Fatalf("RNG draws %d, gorge %d", s.RNGDraws(), gDraws)
		}
	})

	t.Run("tax", func(t *testing.T) {
		sc := partnerScen(t)
		s := sc.s
		a, b := s.FeatCommanderN(0, 0), s.FeatCommanderN(0, 1)
		if a == NoObj || b == NoObj || a == b {
			t.Fatalf("partner pair not seated: %d %d", a, b)
		}
		for i := 0; i < 3; i++ {
			sc.perm(v.Forest, 0)
		}
		if !partnerCastOffered(sc, 0, a) || !partnerCastOffered(sc, 0, b) {
			t.Fatal("both commanders not offered at {2}{G} with three lands")
		}
		castCommanderFromCommandZone(t, sc, a)
		if s.FeatCommanderCastN(0, 0) != 1 || s.FeatCommanderCastN(0, 1) != 0 {
			t.Fatalf("casts %d %d after casting the first, want 1 0", s.FeatCommanderCastN(0, 0), s.FeatCommanderCastN(0, 1))
		}
		returnCommanderToCommandZone(t, sc, a)
		if s.commanderTax(0, a) != 2 || s.commanderTax(0, b) != 0 {
			t.Fatalf("tax %d %d, want 2 0: each commander its own tax", s.commanderTax(0, a), s.commanderTax(0, b))
		}
		for i := 0; i < 3; i++ {
			sc.perm(v.Forest, 0) // three fresh lands: three mana again
		}
		if partnerCastOffered(sc, 0, a) {
			t.Fatal("the cast commander offered at three mana; its taxed cost is five")
		}
		if !partnerCastOffered(sc, 0, b) {
			t.Fatal("the partner, never cast, not offered at three mana: it took the other's tax")
		}
	})

	t.Run("damage", func(t *testing.T) {
		sc, card := cmdVanillaScen(t)
		a, b := sc.perm(card, 0), sc.perm(card, 0)
		sc.s.pl[0].cmd = a
		sc.s.partnerCold().cmd[0] = b
		for hit := 1; hit <= 2; hit++ {
			sc.combat(t, []Obj{a, b}, nil)
			if da, db := sc.s.FeatCommanderDamageN(1, 0, 0), sc.s.FeatCommanderDamageN(1, 0, 1); da != uint8(7*hit) || db != uint8(7*hit) {
				t.Fatalf("after hit %d: tallies %d %d, want %d each", hit, da, db, 7*hit)
			}
			sc.nextMain1()
		}
		// 28 commander damage in all, 14 from each: CR 903.10a counts one
		// commander, so no loss
		if sc.s.pl[1].lost || sc.s.pl[1].life != 12 {
			t.Fatalf("after 14+14: lost %v life %d, want not lost at 12", sc.s.pl[1].lost, sc.s.pl[1].life)
		}
		var pr Projection
		if sc.s.Project(&pr); pr.CmdDmg[1][0] != 28 {
			t.Fatalf("projected seat damage %d, want the summed 28", pr.CmdDmg[1][0])
		}
		sc.combat(t, []Obj{a}, nil)
		if !sc.s.pl[1].lost || sc.s.pl[1].life != 5 {
			t.Fatalf("after the first commander's 21st: lost %v life %d, want lost at 5", sc.s.pl[1].lost, sc.s.pl[1].life)
		}
	})
}
