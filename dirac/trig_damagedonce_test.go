package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func ddoScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Venus, Torn Between Worlds", Count: 1}, {Name: "Innocent Bystander", Count: 1},
		{Name: "Nested Ghoul", Count: 1}, {Name: "Farsight Mask", Count: 1}, {Name: "Sun Droplet", Count: 1}, {Name: "Gray Ogre", Count: 2}, {Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "ddo")
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

// TestDdoGate: a DamageAmount$ GE3 line (Innocent Bystander) queues on a
// batch total of 3, not 2; the compare operators are exact.
func TestDdoGate(t *testing.T) {
	for _, c := range []struct {
		cmp   kop.DmgCmp
		lit   int
		total int
		want  bool
	}{{kop.DmgGE, 3, 2, false}, {kop.DmgGE, 3, 3, true}, {kop.DmgLE, 2, 3, false}, {kop.DmgEQ, 2, 2, true},
		{kop.DmgNE, 2, 2, false}, {kop.DmgGT, 2, 2, false}, {kop.DmgLT, 2, 1, true}} {
		if got := kop.DmgGatePass(kop.DmgGate(c.cmp, c.lit), c.total); got != c.want {
			t.Errorf("cmp %d lit %d total %d: %v, want %v", c.cmp, c.lit, c.total, got, c.want)
		}
	}
	sc, id := ddoScen(t)
	by := sc.perm(id["Innocent Bystander"], 0)
	og := sc.perm(id["Gray Ogre"], 1)
	for _, c := range []struct {
		amt  int16
		want int
	}{{2, 0}, {3, 1}} {
		sc.s.npend = 0
		sc.s.fireDamage(sc.k, sc.slot(og), c.amt, false, false, uint8(by))
		if int(sc.s.npend) != c.want {
			t.Fatalf("%d damage: %d queued, want %d", c.amt, sc.s.npend, c.want)
		}
		if c.want == 1 && (sc.s.pend[0].trigAmt != c.amt || sc.s.pend[0].trigObj != by) {
			t.Fatalf("queued amount %d object %d, want %d and the Bystander %d", sc.s.pend[0].trigAmt, sc.s.pend[0].trigObj, c.amt, by)
		}
	}
}

// TestDdoFilter2Recipient: a per-event DamageDone line with an object
// ValidTarget$ (Nested Ghoul's Creature.Self) fires only for damage to that
// permanent, never for another creature or a player.
func TestDdoFilter2Recipient(t *testing.T) {
	sc, id := ddoScen(t)
	gh := sc.perm(id["Nested Ghoul"], 0)
	og := sc.perm(id["Gray Ogre"], 1)
	for _, c := range []struct {
		rcp  uint8
		want int
	}{{uint8(gh), 1}, {uint8(og), 0}, {uint8(PlayerTarget0), 0}, {uint8(PlayerTarget0 + 1), 0}} {
		sc.s.npend = 0
		sc.s.fireDamage(sc.k, sc.slot(og), 2, TCode(c.rcp) >= PlayerTarget0, false, c.rcp)
		if int(sc.s.npend) != c.want {
			t.Fatalf("recipient %#x: %d queued, want %d", c.rcp, sc.s.npend, c.want)
		}
	}
}

// TestDdoCoalesce: two sources dealing combat damage to one enrage creature
// in one damage step queue its DamageDoneOnce line once, carrying the total
// (Venus, 2 + 3 damage); a second referent queues its own instance.
func TestDdoCoalesce(t *testing.T) {
	sc, id := ddoScen(t)
	ve := sc.perm(id["Venus, Torn Between Worlds"], 0)
	a := sc.perm(id["Gray Ogre"], 1)
	b := sc.perm(id["Gray Ogre"], 1)
	by := sc.perm(id["Innocent Bystander"], 0)
	r := &dmgRec{}
	for _, e := range []dmgEv{{sc.slot(a), sc.slot(ve), 2}, {sc.slot(b), sc.slot(ve), 3}, {sc.slot(b), sc.slot(by), 1}} {
		r.ev[r.nev] = e
		r.nev++
		r.dealt[e.src] += e.n
		r.toPerm[e.dst] += e.n
	}
	sc.s.npend = 0
	sc.s.combatEventsGen(sc.k, r, true)
	if sc.s.npend != 1 || sc.s.pend[0].obj != ve || sc.s.pend[0].trigAmt != 5 {
		t.Fatalf("queued %d (first: object %d amount %d), want one for Venus with 5", sc.s.npend, sc.s.pend[0].obj, sc.s.pend[0].trigAmt)
	}
}

// TestDdoToYou: a "you're dealt damage" line (Farsight Mask, per event; Sun
// Droplet, DamageDoneOnce) fires for damage to its own controller only, in
// either seat: not for the other player, and not for a permanent.
func TestDdoToYou(t *testing.T) {
	sc, id := ddoScen(t)
	for seat := uint8(0); seat < 2; seat++ {
		og := sc.perm(id["Gray Ogre"], 1-seat) // Farsight Mask reads "a source an opponent controls"
		nm := sc.perm(id["Farsight Mask"], seat)
		sg := sc.perm(id["Sun Droplet"], seat)
		for rp := uint8(0); rp < 2; rp++ {
			sc.s.npend = 0
			sc.s.fireDamage(sc.k, sc.slot(og), 2, true, true, uint8(PlayerTarget0+TCode(rp)))
			var gotNM, gotSG bool
			for i := 0; i < int(sc.s.npend); i++ {
				gotNM = gotNM || sc.s.pend[i].obj == nm
				gotSG = gotSG || sc.s.pend[i].obj == sg
			}
			if want := rp == seat; gotNM != want || gotSG != want {
				t.Errorf("seat %d, combat damage to player %d: Farsight Mask %v, Sun Droplet %v, want %v", seat, rp, gotNM, gotSG, want)
			}
		}
		sc.s.npend = 0
		sc.s.fireDamage(sc.k, sc.slot(og), 2, false, false, uint8(og))
		for i := 0; i < int(sc.s.npend); i++ {
			if o := sc.s.pend[i].obj; o == nm || o == sg {
				t.Errorf("seat %d: damage to a permanent queued a to-you line", seat)
			}
		}
	}
}

// TestDdoDealtOnceFilter2: a DamageDealtOnce line with a recipient filter
// fires once per batch with the summed damage of the matching recipients
// (a player recipient never matches an object filter).
func TestDdoDealtOnceFilter2(t *testing.T) {
	deck := []cardset.Entry{{Name: "Nested Ghoul", Count: 1}, {Name: "Gray Ogre", Count: 2}, {Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "ddo")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cd := &tbl.Cards[id["Nested Ghoul"]]
	abs := append([]kop.Ability(nil), cd.Abilities...)
	abs[0].Trig = kop.Trigger{Event: kop.TrigDamageDealtOnce, Filter: kop.FilterID(4), Filter2: kop.FilterID(8)}
	cd.Abilities = abs
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	gh := sc.perm(id["Nested Ghoul"], 0)
	a := sc.perm(id["Gray Ogre"], 1)
	b := sc.perm(id["Gray Ogre"], 1)
	sc.s.res.e.obj = gh
	sc.s.npend = 0
	sc.s.fireDamageRefs(sc.k, []uint8{uint8(a), uint8(PlayerTarget0 + 1), uint8(b)}, []int16{2, 4, 3})
	if sc.s.npend != 1 || sc.s.pend[0].trigAmt != 5 {
		t.Fatalf("queued %d (amount %d), want one carrying 5", sc.s.npend, sc.s.pend[0].trigAmt)
	}
}

// TestFireDamageSrcInPlay: damage from a source that is not a permanent (an
// ability whose source was sacrificed, matched by its card) fires a plain
// "Creature" ValidSource line but never a Creature.inZoneBattlefield one
// (No Mercy; gorge wordInZone is a live zone test).
func TestFireDamageSrcInPlay(t *testing.T) {
	deck := []cardset.Entry{{Name: "No Mercy", Count: 1}, {Name: "Gray Ogre", Count: 2}, {Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "ddo")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	tr := &tbl.Cards[id["No Mercy"]].Abilities[0].Trig
	if tr.Flags2&kop.Tf2SrcInPlay == 0 {
		t.Fatal("No Mercy lost Tf2SrcInPlay")
	}
	nm := sc.perm(id["No Mercy"], 0)
	og := sc.obj(id["Gray Ogre"], 1) // the left-play source: no permanent
	for _, c := range []struct {
		inPlay bool
		want   int
	}{{true, 0}, {false, 1}} {
		if c.inPlay {
			tr.Flags2 |= kop.Tf2SrcInPlay
		} else {
			tr.Flags2 &^= kop.Tf2SrcInPlay
		}
		sc.s.npend = 0
		sc.s.fireDamageSrc(sc.k, og, 1, 1, 1, uint8(PlayerTarget0), nil)
		got := 0
		for i := 0; i < int(sc.s.npend); i++ {
			if sc.s.pend[i].obj == nm {
				got++
			}
		}
		if got != c.want {
			t.Errorf("inZoneBattlefield %v: %d queued, want %d", c.inPlay, got, c.want)
		}
	}
	tr.Flags2 |= kop.Tf2SrcInPlay
}
