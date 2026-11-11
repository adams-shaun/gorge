package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gdSearchScen is a std-trk table with Genesis Device's activated search armed
// (res at its OpSearchEx) over a creature host.
func gdSearchScen(t *testing.T) (sc *scen, id map[string]CardID, host Obj) {
	t.Helper()
	set := cardsettest.Synth(t, "std-trk")
	creature := ""
	for i, n := range set.Names()[:set.NumCards()] {
		if d := set.Def(i); set.Supported()[i] && d.Types == kop.TCreature && len(d.Abilities) == 0 {
			creature = n
			break
		}
	}
	deck := []cardset.Entry{{Name: "Genesis Device", Count: 1}, {Name: "De-evolve", Count: 1}, {Name: "Vengeance Mission", Count: 1}, {Name: "Forest", Count: 4}}
	if creature != "" {
		deck = append(deck, cardset.Entry{Name: creature, Count: 2})
	}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-trk table", e.Name)
		}
	}
	if creature == "" {
		t.Skip("no vanilla creature in the std-trk table")
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	gd := id["Genesis Device"]
	pc := -1
	for i, in := range tbl.Cards[gd].Abilities[0].Prog {
		if in.Op == kop.OpSearchEx {
			pc = i
		}
	}
	if pc < 0 {
		t.Fatal("Genesis Device's activated ability has no OpSearchEx")
	}
	sc.perm(id[creature], 0) // two hosts: the Aura asks (CR 303.4f)
	host = sc.perm(id[creature], 1)
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: NoObj, card: gd, ab: 0, ctrl: 0, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj, pc: uint8(pc)}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve
	s.g.pickP, s.g.pickZ, s.g.pickF = 0, uint8(kop.ZoneBattlefield), kop.SrTapped
	return sc, id, host
}

// TestGenesisDeviceSearchAuraParkNoReWalk: Genesis Device's search puts a look
// of [enchantment, Aura, card] onto the battlefield; the Aura parks at its
// enchant ask, and the answer's re-run walks only the cards still to move, so
// the enchantment that entered is not left again ("kernel: place into
// unsupported zone", kplay pair 1230).
func TestGenesisDeviceSearchAuraParkNoReWalk(t *testing.T) {
	sc, id, host := gdSearchScen(t)
	s := sc.s
	a := sc.obj(id["Vengeance Mission"], 0)
	b := sc.obj(id["De-evolve"], 0)
	c := sc.obj(id["Forest"], 0)
	look := []Obj{a, b, c}
	for _, o := range look {
		s.place(o, kop.ZoneLibrary, false)
		if s.objs[o].zone != kop.ZoneLibrary {
			t.Fatalf("obj %d in zone %d, want the library", o, s.objs[o].zone)
		}
	}
	if a == b || s.objs[a].card == s.objs[b].card {
		t.Fatal("Vengeance Mission and De-evolve are not distinct")
	}
	copy(s.resArm().look[:], look)
	s.g.nlook = 3
	if !s.handAuraArmed(sc.k) {
		t.Fatal("want the OpSearchEx armed")
	}

	s.moveFoundAns(sc.k, NoSlot)
	if s.res.ask != raGenEnchant {
		t.Fatalf("ask %d, want the enchant ask", s.res.ask)
	}
	if !sc.onBF(a) || s.objs[b].zone != kop.ZoneLibrary || s.objs[c].zone != kop.ZoneLibrary {
		t.Fatalf("after the park: A on bf %v, B zone %d, C zone %d; want A entered, the rest waiting", sc.onBF(a), s.objs[b].zone, s.objs[c].zone)
	}
	if s.g.nlook != 2 || s.resArm().look[0] != b {
		t.Fatalf("nlook %d look[0] %d, want 2 starting at the parked Aura %d", s.g.nlook, s.resArm().look[0], b)
	}
	aSlot := sc.slot(a)

	s.res.ask = raNone
	s.moveFoundAns(sc.k, sc.slot(host))
	if !sc.onBF(a) || sc.slot(a) != aSlot {
		t.Fatalf("A on bf %v slot %d, want still in slot %d", sc.onBF(a), sc.slot(a), aSlot)
	}
	if !sc.onBF(b) || s.perms[sc.slot(b)].attached != sc.slot(host) {
		t.Fatalf("Aura on bf %v attached %d, want attached to %d", sc.onBF(b), s.perms[sc.slot(b)].attached, sc.slot(host))
	}
	if !sc.onBF(c) {
		t.Fatalf("the third card is in zone %d, want the battlefield", s.objs[c].zone)
	}
	n := 0
	for _, o := range look {
		if sc.onBF(o) {
			n++
		}
	}
	if n != 3 || s.g.nlook != 0 {
		t.Fatalf("%d of the look on the battlefield, nlook %d, want 3 and 0", n, s.g.nlook)
	}
}

// TestPickLeavesNeverTargetsBattlefield: a look card already on the battlefield
// under a battlefield destination has nothing to move; the walk skips it
// instead of leaving it "to" the battlefield (placeFromK has no such arm).
func TestPickLeavesNeverTargetsBattlefield(t *testing.T) {
	sc, id, _ := gdSearchScen(t)
	s := sc.s
	a := sc.perm(id["Vengeance Mission"], 0)
	c := sc.obj(id["Forest"], 0)
	s.place(c, kop.ZoneLibrary, false)
	if !sc.onBF(a) || s.objs[c].zone != kop.ZoneLibrary {
		t.Fatal("want A on the battlefield and C in the library")
	}
	aSlot := sc.slot(a)
	copy(s.resArm().look[:], []Obj{a, c})
	s.g.nlook = 2

	s.moveFoundAns(sc.k, NoSlot)
	if !sc.onBF(a) || sc.slot(a) != aSlot {
		t.Fatalf("A on bf %v slot %d, want untouched in slot %d", sc.onBF(a), sc.slot(a), aSlot)
	}
	if !sc.onBF(c) {
		t.Fatalf("C in zone %d, want the battlefield", s.objs[c].zone)
	}
}
