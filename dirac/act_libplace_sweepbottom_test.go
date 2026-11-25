package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// hbScen is Hallowed Burial's bottom-of-library sweep (ChangeZoneAll to
// LibraryPosition$ -1, bracketed by kop.ExtLibPlace PlaceArmSweepBottom) on
// p0's one creature and p1's three. run executes the arm, the sweep and the
// ask; it returns false at the pose.
func hbScen(t *testing.T) (sc *scen, run func() bool, p0, p1 []Obj) {
	t.Helper()
	set := cardsettest.Synth(t, "tail-07")
	deck := []cardset.Entry{
		{Name: "Hallowed Burial", Count: 1},
		{Name: "Eager Cadet", Count: 1},
		{Name: "Centaur Courser", Count: 3},
		{Name: "Plains", Count: 35},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cd := tbl.Cards[id["Hallowed Burial"]]
	ab := cd.Abilities[0]
	var prog []kop.Instr
	for _, in := range ab.Prog {
		if in.Op == kop.OpChangeZoneAll || in.Op == kop.OpExt && kop.ExtOp(in.A.N) == kop.ExtLibPlace {
			prog = append(prog, in)
		}
	}
	if len(prog) != 3 {
		t.Fatalf("Hallowed Burial's sweep is bracketed by %d instructions, want arm, sweep, ask", len(prog))
	}
	ab.Prog = prog
	cd.Abilities = []kop.Ability{ab}
	sweeper := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, cd)
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	src := sc.perm(id["Plains"], 0)
	p0 = []Obj{sc.perm(id["Eager Cadet"], 0)}
	for i := 0; i < 5; i++ {
		p1 = append(p1, sc.perm(id["Centaur Courser"], 1))
	}
	tok := sc.perm(id["Eager Cadet"], 1)
	sc.s.objs[tok].of |= ofToken
	p1 = append(p1, tok)
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: src, srcVer: s.objs[src].ver, card: sweeper, ab: 0, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.scrResEnter()
	// a card returning from "until this leaves" exile whose as-enters choice is
	// parked during the sweep, as Abdel Adrian's leave does in the recording.
	ret := sc.obj(id["Eager Cadet"], 1)
	s.objs[ret].zone = kop.ZoneExile
	run = func() bool {
		// the arm first, then the sweep parks the returning card's choice (as
		// Abdel Adrian's leave does): the case the ordering must handle.
		in := &prog[0]
		if !s.exec(sc.k, in) {
			return false
		}
		s.res.pc = 1
		s.g.pre = preAsk{obj: ret, state: preReturn}
		s.runProg(sc.k)
		return s.res.ask == raNone
	}
	return sc, run, p0, p1
}

// TestLibPlaceBottomSweepParksAtBottom: before the owner's arrange answer, a
// ChangeZoneAll to the library's bottom has the swept cards at the BOTTOM of
// their owners' libraries in move order: gorge's MoveZone appends them there,
// and the ask is posed with them still there. A single swept card asks
// nothing and is already at the bottom.
func TestLibPlaceBottomSweepParksAtBottom(t *testing.T) {
	sc, run, p0, p1 := hbScen(t)
	s := sc.s
	if run() || s.res.ask != raGenLibPlace {
		t.Fatalf("ask %d, want the library placement ask", s.res.ask)
	}
	if s.res.askP != 1 {
		t.Fatalf("asked seat %d, want p1 (three cards)", s.res.askP)
	}
	for _, tc := range []struct {
		seat uint8
		want []Obj
	}{{0, p0}, {1, p1}} {
		pl := &s.pl[tc.seat]
		base := pl.libLen() - len(tc.want)
		got := slices.Clone(pl.lib[int(pl.libLo)+base : int(pl.libHi)])
		if !slices.Equal(got, tc.want) {
			t.Fatalf("seat %d bottom %v, want the swept cards in move order %v", tc.seat, got, tc.want)
		}
	}
	// The parked return waits for the arrangement: once p1 orders its group,
	// the as-enters choice is posed.
	for s.res.ask == raGenLibPlace {
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		if d.N == 0 {
			t.Fatal("library placement ask with no options")
		}
		s.applyAskCold2(sc.k, d.Options[0])
	}
	if s.res.ask != raGenRetChoice || s.res.askP != 1 {
		t.Fatalf("ask %d for seat %d after the arrangement, want the return choice %d for seat 1", s.res.ask, s.res.askP, raGenRetChoice)
	}
}
