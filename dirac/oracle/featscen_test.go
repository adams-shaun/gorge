package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestFeatScenariosBuild(t *testing.T) {
	c, _ := testCorpus(t)
	ss, err := FeatScenarios(rawSet("fdn"), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != len(FeatScenarioNames) {
		t.Fatalf("%d scenarios, want %d", len(ss), len(FeatScenarioNames))
	}
	by := map[string]*FeatScenario{}
	pj := map[string]*dirac.Projection{}
	for i := range ss {
		s := &ss[i]
		if s.Name != FeatScenarioNames[i] {
			t.Fatalf("scenario %d is %q, want %q", i, s.Name, FeatScenarioNames[i])
		}
		if s.State == nil || s.Pool == nil || s.Dec.N == 0 {
			t.Fatalf("%s: incomplete scenario", s.Name)
		}
		t.Logf("%s: turn %d roles %v", s.Name, s.State.Turn(), s.Roles)
		by[s.Name] = s
		p := new(dirac.Projection)
		s.State.Project(p)
		pj[s.Name] = p
	}
	// idOf is the card id of a scenario's role card, found by name on its objects.
	idOf := func(s *FeatScenario, role string) kop.CardID {
		p := pj[s.Name]
		for i := 0; i < int(p.NObj); i++ {
			if s.State.Table().Cards[p.Objs[i].Card].Name == s.Roles[role] {
				return p.Objs[i].Card
			}
		}
		t.Fatalf("%s: role %s (%s) is on no object", s.Name, role, s.Roles[role])
		return 0
	}
	top := func(name string) *dirac.ProjStack {
		p := pj[name]
		if p.NStack == 0 {
			t.Fatalf("%s: empty stack", name)
		}
		return &p.Stack[p.NStack-1]
	}
	inZone := func(s *FeatScenario, role string, z kop.Zone, owner uint8) bool {
		id, p := idOf(s, role), pj[s.Name]
		for i := 0; i < int(p.NObj); i++ {
			if o := &p.Objs[i]; o.Card == id && o.Zone == z && o.Owner == owner {
				return true
			}
		}
		return false
	}
	// the four bomb positions are one turn of one game
	for _, n := range []string{"bomb-on-stack", "bomb-countered", "bomb-resolved"} {
		if by[n].State.Turn() != by["bomb-precast"].State.Turn() {
			t.Errorf("%s is on turn %d, bomb-precast on turn %d", n, by[n].State.Turn(), by["bomb-precast"].State.Turn())
		}
	}
	s := by["bomb-on-stack"]
	if p := pj[s.Name]; p.NStack != 1 || top(s.Name).Card != idOf(s, "bomb") || top(s.Name).Ctrl != 1-s.Seat {
		t.Errorf("bomb-on-stack: NStack %d top %+v", p.NStack, top(s.Name))
	}
	s = by["bomb-countered"]
	if pj[s.Name].NStack != 0 || !inZone(s, "bomb", kop.ZoneGraveyard, 1-s.Seat) {
		t.Errorf("bomb-countered: NStack %d, bomb in opp graveyard %v", pj[s.Name].NStack, inZone(s, "bomb", kop.ZoneGraveyard, 1-s.Seat))
	}
	s = by["bomb-resolved"]
	if !inZone(s, "bomb", kop.ZoneBattlefield, 1-s.Seat) {
		t.Errorf("bomb-resolved: bomb is not on the opponent's battlefield")
	}
	s = by["cheapkill-on-oneone"]
	if tg := top(s.Name).Targets[0]; int(tg) >= int(pj[s.Name].NObj) || pj[s.Name].Objs[tg].Card != idOf(s, "oneone") || pj[s.Name].Objs[tg].Ctrl != s.Seat {
		t.Errorf("cheapkill-on-oneone: target %d is not the seat's 1/1", tg)
	}
	s = by["counter-prevents-lethal"]
	if tg := top(s.Name); tg.Targets[0] != dirac.PlayerTarget0+s.Seat || tg.Card != idOf(s, "burn") ||
		int(s.Pool.Effect(tg.Card).Amount) < int(pj[s.Name].Life[s.Seat]) {
		t.Errorf("counter-prevents-lethal: top %+v life %d", tg, pj[s.Name].Life[s.Seat])
	}
}

func TestFeatScenariosDeterministic(t *testing.T) {
	c, _ := testCorpus(t)
	a, err := FeatScenarios(rawSet("fdn"), c)
	if err != nil {
		t.Fatal(err)
	}
	b, err := FeatScenarios(rawSet("fdn"), c)
	if err != nil {
		t.Fatal(err)
	}
	for i := range a {
		var pa, pb dirac.Projection
		a[i].State.Project(&pa)
		b[i].State.Project(&pb)
		if pa != pb || a[i].Dec != b[i].Dec {
			t.Errorf("%s: two builds differ", a[i].Name)
		}
	}
}
