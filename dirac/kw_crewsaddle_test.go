package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kw_crewsaddle.go on the generated crew pool: Crew's and Saddle's
// tap-any-number cost (CostTapPower, gorge's KChoose "tapcost" with
// MinSum), the saddled designation, Exhaust's once-only activation.

func crewScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "crew")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the crew table", n)
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
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// tapPicks lists a ChCostTap decision's picked objects and whether Done
// is offered.
func tapPicks(t *testing.T, d *Decision) (objs []Obj, done bool) {
	t.Helper()
	if d.Kind != DkChoose || d.Sub != ChCostTap {
		t.Fatalf("decision kind %d sub %d, want the tap-cost pick (opts %v)", d.Kind, d.Sub, d.Options[:d.N])
	}
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkDone {
			done = true
		} else {
			objs = append(objs, Obj(a.Source()))
		}
	}
	return objs, done
}

func hasObj(os []Obj, o Obj) bool {
	for _, x := range os {
		if x == o {
			return true
		}
	}
	return false
}

// TestCrewTapsPower (CR 702.122a; gorge tapPermanentCostAsk, castparts.go):
// Crew 1 is not offered while the other untapped creatures' total power is
// below 1 (a 0/3 wall); with a Bear it is. A pick is offered while the
// answer can still reach the floor, Done only once it does, and picking
// the last candidate ends the pick. The picks tap with the cost and the
// Vehicle becomes an artifact creature with its printed P/T.
func TestCrewTapsPower(t *testing.T) {
	sc, id := crewScen(t, "Sleek Schooner", "Wall of Wood", "Grizzly Bears")
	if !sc.k.gen.anyTapPower {
		t.Fatal("anyTapPower not set")
	}
	ship := sc.perm(id["Sleek Schooner"], 0)
	wall := sc.perm(id["Wall of Wood"], 0)
	if sc.activateAb(ship, 0) {
		t.Fatal("Crew 1 offered with total power 0")
	}
	bears := sc.perm(id["Grizzly Bears"], 0)
	if !sc.activateAb(ship, 0) {
		t.Fatal("Crew 1 not offered with a 2-power creature")
	}
	d := sc.s.Step()
	objs, done := tapPicks(t, &d)
	if done || len(objs) != 2 || !hasObj(objs, wall) || !hasObj(objs, bears) {
		t.Fatalf("first pick %v done %v, want wall and bears, no Done", objs, done)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(wall), 0, 0))
	d = sc.s.Step()
	objs, done = tapPicks(t, &d)
	if done || len(objs) != 1 || objs[0] != bears {
		t.Fatalf("after the wall %v done %v, want bears only (total 0 < 1)", objs, done)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(bears), 0, 0))
	// every candidate picked: the pick ends without Done
	sc.resolveAll(t)
	if !sc.tapped(wall) || !sc.tapped(bears) || sc.tapped(ship) {
		t.Fatalf("tapped wall %v bears %v ship %v, want true true false", sc.tapped(wall), sc.tapped(bears), sc.tapped(ship))
	}
	sl := sc.slot(ship)
	if !sc.s.isCreature(sc.k, sl) {
		t.Fatal("the crewed Vehicle is not a creature")
	}
	if p, tg := sc.s.pt(sc.k, sl); p != 4 || tg != 3 {
		t.Fatalf("crewed Sleek Schooner %d/%d, want 4/3", p, tg)
	}
}

// TestCrewDoneAtFloor: with the floor met by one pick, Done joins the
// remaining candidates; Done ends the pick with the rest untapped.
func TestCrewDoneAtFloor(t *testing.T) {
	sc, id := crewScen(t, "Sleek Schooner", "Grizzly Bears")
	ship := sc.perm(id["Sleek Schooner"], 0)
	b1 := sc.perm(id["Grizzly Bears"], 0)
	b2 := sc.perm(id["Grizzly Bears"], 0)
	if !sc.activateAb(ship, 0) {
		t.Fatal("Crew 1 not offered")
	}
	sc.s.Step()
	sc.s.Apply(MakeAction(AkChoose, TCode(b1), 0, 0))
	d := sc.s.Step()
	objs, done := tapPicks(t, &d)
	if !done || len(objs) != 1 || objs[0] != b2 {
		t.Fatalf("after one Bear %v done %v, want the other Bear and Done", objs, done)
	}
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
	sc.resolveAll(t)
	if !sc.tapped(b1) || sc.tapped(b2) {
		t.Fatalf("tapped %v %v, want true false", sc.tapped(b1), sc.tapped(b2))
	}
}

// TestSaddleAttack (CR 702.171; gorge effAlterAttribute Saddled, filter
// IsSaddled): Saddle is sorcery speed; a saddled Bridled Bighorn's attack
// trigger makes a Sheep, an unsaddled one's does not; the designation
// ends as the next turn begins.
func TestSaddleAttack(t *testing.T) {
	sc, id := crewScen(t, "Bridled Bighorn", "Grizzly Bears")
	if !sc.k.gen.anySaddle {
		t.Fatal("anySaddle not set")
	}
	horn := sc.perm(id["Bridled Bighorn"], 0)
	bears := sc.perm(id["Grizzly Bears"], 0)
	sheep := func() int {
		n := 0
		for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
			if sc.s.objs[sc.s.perms[m.first()].obj].of&ofToken != 0 {
				n++
			}
		}
		return n
	}
	if !sc.activateAb(horn, 0) {
		t.Fatal("Saddle 2 not offered")
	}
	d := sc.s.Step()
	if objs, done := tapPicks(t, &d); done || len(objs) != 1 {
		t.Fatalf("saddle pick %v done %v", objs, done)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(bears), 0, 0))
	sc.resolveAll(t)
	if sc.s.perms[sc.slot(horn)].cseen&cseenSaddled == 0 {
		t.Fatal("not saddled")
	}
	sc.combat(t, []Obj{horn}, nil)
	sc.resolveAll(t)
	if sheep() != 1 {
		t.Fatalf("%d Sheep after a saddled attack, want 1", sheep())
	}
	sc.s.beginTurn(1)
	if sc.s.perms[sc.slot(horn)].cseen&cseenSaddled != 0 {
		t.Fatal("still saddled on the next turn")
	}
}

// TestExhaustOnce (CR 702.177a; gorge activationLimitBlocked): an exhaust
// ability is offered once per object.
func TestExhaustOnce(t *testing.T) {
	sc, id := crewScen(t, "Prowcatcher Specialist", "Mountain")
	pc := sc.perm(id["Prowcatcher Specialist"], 0)
	sc.lands(0, id["Mountain"], 8)
	if !sc.activateAb(pc, 0) {
		t.Fatal("exhaust not offered")
	}
	sc.resolveAll(t)
	if sc.activateAb(pc, 0) {
		t.Fatal("exhaust offered twice")
	}
}
