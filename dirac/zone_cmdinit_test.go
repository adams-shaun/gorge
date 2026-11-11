package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

const cmdTestSeed = 9031

// cmdTestCommanders are each seat's commander index into its 100-card deck.
var cmdTestCommanders = [MaxSeats]int16{37, 62}

// cmdTestConfig is cmdTestCommanders as Config.Commanders: one a seat.
func cmdTestConfig() [MaxSeats][2]int16 {
	c := noCommanders()
	for p := range c {
		c[p][0] = cmdTestCommanders[p]
	}
	return c
}

// cmdTestDecks are two 100-card kernel decks; the card at each seat's
// commander index is a creature.
func cmdTestDecks() [2][]CardID {
	var d [2][]CardID
	for p := range d {
		d[p] = make([]CardID, 100)
		for i := range d[p] {
			d[p][i] = v.Forest
			if i%3 == 0 {
				d[p][i] = v.GrizzlyBears
			}
		}
		d[p][cmdTestCommanders[p]] = v.GrizzlyBears
	}
	return d
}

func cmdTestCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("t.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// gorgeGenesis deals the same 100-card decks (by position; the commander a
// legendary creature so gorge seats it) in gorge and returns, per seat, the
// library, hand and command zone as deck indices, and the life totals.
func gorgeGenesis(t *testing.T, commander bool) (lib, hand, cmd [2][]int, life [2]int, draws uint64) {
	t.Helper()
	land := cmdTestCard(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
	leg := cmdTestCard(t, "Name:Test Commander\nManaCost:1 G\nTypes:Legendary Creature Elf\nPT:2/2\nOracle:x\n")
	decks := make([][]*cards.Card, 2)
	for p := range decks {
		decks[p] = make([]*cards.Card, 100)
		for i := range decks[p] {
			decks[p][i] = land
		}
		decks[p][cmdTestCommanders[p]] = leg
	}
	cfg := rules.Config{Seed: cmdTestSeed, Names: []string{"a", "b"}, Decks: decks}
	if commander {
		cfg.Format, cfg.StartingLife = rules.FormatCommander, 40
		cfg.Commanders = [][]int{{int(cmdTestCommanders[0])}, {int(cmdTestCommanders[1])}}
	}
	e := rules.New(cfg)
	for p := 0; p < 2; p++ {
		pid := state.PlayerID(p)
		zones := []state.Zone{state.ZLibrary, state.ZHand, state.ZCommand}
		base := state.ObjID(0)
		for _, z := range zones {
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
		life[p] = int(e.G.Players[p].Life)
	}
	return lib, hand, cmd, life, e.RNGDraws()
}

// diracGenesis is the kernel's deal of the same decks, as deck indices.
func diracGenesis(s *State) (lib, hand [2][]int) {
	for p := 0; p < 2; p++ {
		pl := &s.pl[p]
		base := p * 100
		for _, o := range pl.lib[pl.libLo:pl.libHi] {
			lib[p] = append(lib[p], int(o)-base)
		}
		for _, o := range pl.hand[:pl.handN] {
			hand[p] = append(hand[p], int(o)-base)
		}
	}
	return lib, hand
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCommanderGenesisCommandZone(t *testing.T) {
	d := cmdTestDecks()
	s := NewConfig(v.Table, d[0], d[1], cmdTestSeed, Config{NoElide: true, Commander: true, Commanders: cmdTestConfig()})
	gLib, gHand, gCmd, gLife, gDraws := gorgeGenesis(t, true)
	lib, hand := diracGenesis(s)
	for p := 0; p < 2; p++ {
		pl := &s.pl[p]
		want := Obj(p*100 + int(cmdTestCommanders[p]))
		if pl.cmd != want || s.objs[want].zone != kop.ZoneCommand {
			t.Fatalf("seat %d: cmd %d zone %v, want object %d in the command zone", p, pl.cmd, s.objs[want].zone, want)
		}
		if pl.cmdCast != 0 || pl.cmdAsk != 0 || pl.cmdDmg != [MaxSeats]uint8{} {
			t.Fatalf("seat %d: commander bookkeeping not zero", p)
		}
		if pl.libLen() != 99-7 || pl.handN != 7 || pl.life != 40 {
			t.Fatalf("seat %d: library %d hand %d life %d, want 92 7 40", p, pl.libLen(), pl.handN, pl.life)
		}
		// the gorge reference seated the commander too: the comparison below
		// compares two command-zone deals, not two plain ones
		if len(gCmd[p]) != 1 || gCmd[p][0] != int(cmdTestCommanders[p]) || gLife[p] != 40 {
			t.Fatalf("seat %d: gorge command zone %v life %d", p, gCmd[p], gLife[p])
		}
		if !sameInts(lib[p], gLib[p]) || !sameInts(hand[p], gHand[p]) {
			t.Fatalf("seat %d: library/hand differ from gorge\nkernel lib %v hand %v\ngorge  lib %v hand %v", p, lib[p], hand[p], gLib[p], gHand[p])
		}
	}
	if s.RNGDraws() != gDraws {
		t.Fatalf("RNG draws %d, gorge %d", s.RNGDraws(), gDraws)
	}
	if got := s.startingLife(); got != 40 {
		t.Fatalf("starting life %d, want 40", got)
	}
}

func TestCommanderGenesisOffUnchanged(t *testing.T) {
	d := cmdTestDecks()
	s := NewConfig(v.Table, d[0], d[1], cmdTestSeed, Config{NoElide: true, Commanders: cmdTestConfig()})
	gLib, gHand, gCmd, _, gDraws := gorgeGenesis(t, false)
	lib, hand := diracGenesis(s)
	for p := 0; p < 2; p++ {
		pl := &s.pl[p]
		if pl.cmd != NoObj || pl.libLen() != 100-7 || pl.life != 20 || len(gCmd[p]) != 0 {
			t.Fatalf("seat %d: cmd %d library %d life %d gorge cmd %v", p, pl.cmd, pl.libLen(), pl.life, gCmd[p])
		}
		if !sameInts(lib[p], gLib[p]) || !sameInts(hand[p], gHand[p]) {
			t.Fatalf("seat %d: library/hand differ from gorge", p)
		}
	}
	// pinned: the first 10 library cards of each seat and the draw count
	// (one toss, two 100-card shuffles: 1 + 99 + 99)
	pin := [2][]int{
		{18, 85, 82, 35, 8, 58, 16, 40, 19, 30},
		{5, 55, 34, 10, 47, 64, 76, 8, 32, 59},
	}
	for p := 0; p < 2; p++ {
		if !sameInts(lib[p][:10], pin[p]) {
			t.Errorf("seat %d: first 10 library cards %v, pinned %v", p, lib[p][:10], pin[p])
		}
	}
	if s.RNGDraws() != 199 || gDraws != 199 {
		t.Errorf("RNG draws %d (gorge %d), pinned 199", s.RNGDraws(), gDraws)
	}
	if got := s.startingLife(); got != 20 {
		t.Fatalf("starting life %d, want 20", got)
	}
}
