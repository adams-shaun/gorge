package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newCmd5ShapesScen is a scenario over the cmd5shapes pool with Forests in
// the libraries.
func newCmd5ShapesScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Chandra's Phoenix", Count: 1}, {Name: "Chandra, Flameshaper", Count: 1},
		{Name: "Lightning Bolt", Count: 1}, {Name: "Counterspell", Count: 1}, {Name: "Gray Ogre", Count: 1},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Vastwood Hydra", Count: 1}, {Name: "Forest", Count: 10},
	}
	set := cardsettest.Set(t, "cmd5shapes")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in this table", e.Name)
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
	return newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true}), id
}

// queuedFor counts the pending triggers whose source is o.
func queuedFor(s *State, o Obj) int {
	n := 0
	for i := 0; i < int(s.npend); i++ {
		if s.pend[i].obj == o {
			n++
		}
	}
	return n
}

// TestChandrasPhoenixFromGraveyard: damage to the opponent from a red instant
// or sorcery spell its controller controls, or from a red planeswalker that
// controller controls, queues the Phoenix in that controller's graveyard; a
// creature's damage, an off-colour spell, the opponent's own spell, damage to
// oneself, and a Phoenix on the battlefield queue nothing (CR 603.6, 113.6k).
func TestChandrasPhoenixFromGraveyard(t *testing.T) {
	sc, id := newCmd5ShapesScen(t)
	s := sc.s
	ph := sc.grave(id["Chandra's Phoenix"], 0, 1)[0]
	if s.objs[ph].zone != kop.ZoneGraveyard {
		t.Fatal("the Phoenix is not in the graveyard")
	}
	bolt := sc.obj(id["Lightning Bolt"], 0)
	oppBolt := sc.obj(id["Lightning Bolt"], 1)
	spell := sc.obj(id["Counterspell"], 0)
	pw := sc.perm(id["Chandra, Flameshaper"], 0)
	ogre := sc.perm(id["Gray Ogre"], 0)
	opp := PlayerTarget0 + 1
	for _, c := range []struct {
		name string
		fire func()
		want int
	}{
		{"red instant spell to the opponent", func() { s.fireDamageSrc(sc.k, bolt, 0, 3, 1<<1, uint8(opp), nil) }, 1},
		{"red planeswalker to the opponent", func() { s.fireDamage(sc.k, sc.slot(pw), 2, true, false, uint8(opp)) }, 1},
		{"a creature's noncombat damage", func() { s.fireDamage(sc.k, sc.slot(ogre), 2, true, false, uint8(opp)) }, 0},
		{"a creature's combat damage", func() { s.fireDamage(sc.k, sc.slot(ogre), 2, true, true, uint8(opp)) }, 0},
		{"a blue spell", func() { s.fireDamageSrc(sc.k, spell, 0, 3, 1<<1, uint8(opp), nil) }, 0},
		{"the opponent's red spell to the Phoenix's owner", func() { s.fireDamageSrc(sc.k, oppBolt, 1, 3, 1<<0, uint8(PlayerTarget0), nil) }, 0},
		{"the controller's own spell to the controller", func() { s.fireDamageSrc(sc.k, bolt, 0, 3, 1<<0, uint8(PlayerTarget0), nil) }, 0},
		{"the red planeswalker's damage to a creature", func() { s.fireDamage(sc.k, sc.slot(pw), 2, false, false, uint8(ogre)) }, 0},
	} {
		s.npend = 0
		c.fire()
		if got := queuedFor(s, ph); got != c.want {
			t.Errorf("%s: %d queued for the Phoenix, want %d", c.name, got, c.want)
		}
	}
	// the Phoenix on the battlefield has no graveyard line to fire
	s.detach(ph)
	s.objs[ph].zone = kop.ZoneBattlefield
	s.npend = 0
	s.fireDamageSrc(sc.k, bolt, 0, 3, 1<<1, uint8(opp), nil)
	if got := queuedFor(s, ph); got != 0 {
		t.Errorf("a Phoenix off the graveyard queued %d", got)
	}
}

// hydraDies kills a Vastwood Hydra carrying n +1/+1 counters beside nBears
// Grizzly Bears of its controller, accepts the may-trigger and picks the
// bears in the order given (indexes into bears), then resolves the stack.
// It returns the bears' counters and the picks asked.
func hydraDies(t *testing.T, n, nBears int, picks []int) (counters []int, asked int) {
	t.Helper()
	sc, id := newCmd5ShapesScen(t)
	s := sc.s
	hy := sc.perm(id["Vastwood Hydra"], 0)
	hs := sc.slot(hy)
	if n > 0 {
		p := &s.perms[hs]
		p.seeCtr(s)
		s.placeCounters(sc.k, hs, int(kop.CtrP1P1), n, 0, ctrPutRaw)
	}
	bears := make([]Obj, nBears)
	for i := range bears {
		bears[i] = sc.perm(id["Grizzly Bears"], 0)
	}
	s.leave(sc.k, hs, kop.ZoneGraveyard, false)
	next := 0
	for i := 0; i < 60; i++ {
		d := s.Step()
		if d.Kind == DkPriority && s.sp == 0 && s.npend == 0 {
			break
		}
		if d.Kind == DkPriority {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		a := d.Options[0]
		switch {
		case d.Sub == ChGenDistPick:
			asked++
			if next < len(picks) {
				for j := 0; j < int(d.N); j++ {
					if o := d.Options[j]; o.Kind() == AkChoose && Obj(o.Source()) == bears[picks[next]] {
						a = o
					}
				}
				next++
			} else {
				for j := 0; j < int(d.N); j++ {
					if d.Options[j].Kind() == AkDone {
						a = d.Options[j]
					}
				}
			}
		case d.Kind == DkChoose:
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Mode() == 1 {
					a = d.Options[j]
					break
				}
			}
		}
		s.Apply(a)
	}
	for _, b := range bears {
		counters = append(counters, s.perms[sc.slot(b)].ctrN(s, int(kop.CtrP1P1)))
	}
	return counters, asked
}

// TestVastwoodHydraDistributes: the dying Hydra's last-known counters go
// round-robin among the creatures its controller picks (no targets; the
// earlier picks take the remainder), with a count of 0 and of 5, one and two
// eligible creatures, and a smaller-than-all set.
func TestVastwoodHydraDistributes(t *testing.T) {
	for _, c := range []struct {
		name      string
		n, bears  int
		picks     []int
		want      []int
		wantAsked int
	}{
		{"no counters", 0, 2, nil, []int{0, 0}, 0},
		{"one creature takes all unasked", 5, 1, nil, []int{5}, 0},
		{"two picked, the first takes the extra", 5, 2, []int{1, 0}, []int{2, 3}, 2},
		{"one picked of two", 5, 2, []int{0}, []int{5, 0}, 2},
		{"three counters, two picked", 3, 2, []int{0, 1}, []int{2, 1}, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, asked := hydraDies(t, c.n, c.bears, c.picks)
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("bears carry %v, want %v", got, c.want)
				}
			}
			if asked != c.wantAsked {
				t.Errorf("%d pick decisions, want %d", asked, c.wantAsked)
			}
		})
	}
}
