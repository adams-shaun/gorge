package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gdSearchScen is an FDN table plus a source card shaped like Genesis Device's
// activated search (an AbActivated ability whose OpSearchEx moves tapped, SrTapped,
// onto the battlefield), armed in the resolver, with two Trufflesnout hosts so an
// Aura's entry asks (CR 303.4f). Genesis Device itself is only in the synthetic
// std-trk pool, which archtest bars from tests that are not on its list.
func gdSearchScen(t *testing.T) (sc *scen, id map[string]CardID, hosts [2]Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Plains", Count: 16}}
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
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Genesis Search", Types: kop.TCreature | kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpSearchEx}}}}})
	sc = newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	hosts = [2]Obj{sc.perm(id["Eager Trufflesnout"], 0), sc.perm(id["Eager Trufflesnout"], 1)}
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: NoObj, card: src, ab: 0, ctrl: 0, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve
	s.g.pickP, s.g.pickZ, s.g.pickF = 0, uint8(kop.ZoneBattlefield), kop.SrTapped
	if !s.handAuraArmed(sc.k) {
		t.Fatal("want the OpSearchEx armed")
	}
	return sc, id, hosts
}

// TestGenesisDeviceSearchAuraParkNoReWalk: a Genesis Device-shaped search puts a
// look of [land, Aura, land] onto the battlefield; the Aura parks at its
// enchant ask, and the answer's re-run walks only the cards still to move, so the
// land that entered is not left again ("kernel: place into unsupported
// zone", kplay pair 1230; fixed by 7e7458e2be's trim of the look at the park).
func TestGenesisDeviceSearchAuraParkNoReWalk(t *testing.T) {
	sc, id, hosts := gdSearchScen(t)
	s := sc.s
	a := sc.obj(id["Plains"], 0)
	b := sc.obj(id["Witness Protection"], 0)
	c := sc.obj(id["Plains"], 0)
	look := []Obj{a, b, c}
	for _, o := range look {
		s.place(o, kop.ZoneLibrary, false)
		if s.objs[o].zone != kop.ZoneLibrary {
			t.Fatalf("obj %d in zone %d, want the library", o, s.objs[o].zone)
		}
	}
	if a == b || s.objs[a].card == s.objs[b].card {
		t.Fatal("the land and the Aura are not distinct")
	}
	copy(s.resArm().look[:], look)
	s.g.nlook = 3

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
	aSlot, host := sc.slot(a), sc.slot(hosts[1])

	s.res.ask = raNone
	s.moveFoundAns(sc.k, host)
	if !sc.onBF(a) || sc.slot(a) != aSlot {
		t.Fatalf("A on bf %v slot %d, want still in slot %d", sc.onBF(a), sc.slot(a), aSlot)
	}
	if !sc.onBF(b) || s.perms[sc.slot(b)].attached != host {
		t.Fatalf("Aura on bf %v attached %d, want attached to %d", sc.onBF(b), s.perms[sc.slot(b)].attached, host)
	}
	if !sc.onBF(c) {
		t.Fatalf("the third card is in zone %d, want the battlefield", s.objs[c].zone)
	}
	if s.g.nlook != 0 {
		t.Fatalf("nlook %d after the answer, want 0", s.g.nlook)
	}
}

// TestPickLeavesNeverTargetsBattlefield: a look card already on the battlefield
// under a battlefield destination has nothing to move; the walk skips it
// instead of leaving it "to" the battlefield (placeFromK has no such arm).
func TestPickLeavesNeverTargetsBattlefield(t *testing.T) {
	sc, id, _ := gdSearchScen(t)
	s := sc.s
	a := sc.perm(id["Plains"], 0)
	c := sc.obj(id["Plains"], 0)
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
