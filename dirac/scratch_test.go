package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// scratchLive names the arms of scr (scratch.go) whose live
// window holds in s.
func scratchLive(s *State) (n int, names string) {
	if s.scrCombatLive() {
		n++
		names += " combat"
	}
	if s.scrBuildLive() {
		n++
		names += " build"
	}
	if s.scrResLive() {
		n++
		names += " resolution"
	}
	return n, names
}

// TestScratchArmsExclusive: after every action of the vanilla soak of
// TestMarksCoverEveryMutation, and of a sample of FDN pairings (the
// generated tables: replacement-order asks, digs, piles, free casts), at
// most one arm of the scratch overlay is live. Two live arms would read
// each other's bytes.
func TestScratchArmsExclusive(t *testing.T) {
	var d Decision
	seen := [8]int{} // decisions by the arms live: combat, build, resolution as bits
	check := func(s *State, what string, g, i int) {
		t.Helper()
		if n, names := scratchLive(s); n > 1 {
			t.Fatalf("%s game %d action %d: arms live together:%s (ctl %d, ro.on %v, ctlRet %d)", what, g, i, names, s.ctl, s.g.ro.on, s.g.ctlRet)
		}
		b := 0
		if s.scrCombatLive() {
			b |= 1
		}
		if s.scrBuildLive() {
			b |= 2
		}
		if s.scrResLive() {
			b |= 4
		}
		seen[b]++
	}
	n := 150
	if testing.Short() {
		n = 40
	}
	var s State
	for g := 0; g < n; g++ {
		nd := len(v.Decks)
		s.Init(v.Table, v.Decks[g%nd], v.Decks[(g/nd)%nd], uint64(g)*0x51ed27, Config{Mulligans: uint8(g % 3), NoElide: g%4 == 0})
		pol := NewRandomPolicy(uint64(g) ^ 0x77)
		for i := 0; i < 4000; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			check(&s, "vanilla", g, i)
			s.Apply(pol.Pick(&d))
		}
	}
	decks := fdnDecks(t)
	set := cardsettest.Set(t, "fdn")
	for g := 0; g < 24; g++ {
		a := decks[(g*7919)%len(decks)]
		b := decks[(g*104729+31)%len(decks)]
		tbl, da, db, err := set.Matchup(a, b)
		if err != nil {
			t.Fatalf("pairing %d: %v", g, err)
		}
		fs := NewConfig(tbl, da, db, uint64(g)*0x9e37+1, Config{Mulligans: uint8(g % 3), NoElide: g%2 == 1})
		pol := NewRandomPolicy(uint64(g)*7919 + 3)
		for i := 0; i < 30000; i++ {
			fs.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			check(fs, "fdn", g, i)
			fs.Apply(pol.Pick(&d))
		}
		ReleaseTable(tbl)
	}
	t.Logf("decisions by live arms (none, combat, build, combat+build, resolution, ...): %v", seen)
	for _, c := range []struct {
		i    int
		name string
	}{{1, "combat A"}, {2, "build B"}, {4, "resolution C"}} {
		if seen[c.i] == 0 {
			t.Fatalf("no decision was taken with only arm %s live: the soak does not reach it", c.name)
		}
	}
}
