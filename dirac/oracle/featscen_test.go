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
	// the bomb spell's own object, followed into both branches (a graveyard
	// already holds discarded copies, so a card-name check proves nothing)
	src := top("bomb-on-stack").Source
	for _, c := range []struct {
		name string
		z    kop.Zone
	}{{"bomb-countered", kop.ZoneGraveyard}, {"bomb-resolved", kop.ZoneBattlefield}} {
		s = by[c.name]
		o := &pj[c.name].Objs[src]
		if pj[c.name].NStack != 0 || o.Card != idOf(s, "bomb") || o.Zone != c.z || o.Owner != 1-s.Seat {
			t.Errorf("%s: NStack %d, the bomb spell's object is card %d in zone %d owner %d", c.name, pj[c.name].NStack, o.Card, o.Zone, o.Owner)
		}
	}
	// the removal positions hold one spell, and my counter is castable
	castable := func(s *FeatScenario, role string) bool {
		id, p := idOf(s, role), pj[s.Name]
		for _, a := range s.Dec.Options[:s.Dec.N] {
			if a.Kind() == dirac.AkCast && p.Objs[a.Source()].Card == id && p.Objs[a.Source()].Zone == kop.ZoneHand {
				return true
			}
		}
		return false
	}
	for _, n := range []string{"cheapkill-on-oneone", "bigkill-on-bomb", "counter-prevents-lethal"} {
		if s := by[n]; pj[n].NStack != 1 || !castable(s, "counter") {
			t.Errorf("%s: NStack %d, counter castable %v", n, pj[n].NStack, castable(s, "counter"))
		}
	}
	if s := by["bomb-on-stack"]; !castable(s, "counter") {
		t.Errorf("bomb-on-stack: counter not castable")
	}
	s = by["cheapkill-on-oneone"]
	if tg := top(s.Name).Targets[0]; int(tg) >= int(pj[s.Name].NObj) || pj[s.Name].Objs[tg].Card != idOf(s, "oneone") || pj[s.Name].Objs[tg].Ctrl != s.Seat {
		t.Errorf("cheapkill-on-oneone: target %d is not the seat's 1/1", tg)
	}
	s = by["counter-prevents-lethal"]
	if tg := top(s.Name); dirac.TCode(tg.Targets[0]) != dirac.PlayerTarget0+dirac.TCode(s.Seat) || tg.Card != idOf(s, "burn") ||
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

func TestFeatLandkillClass(t *testing.T) {
	for tg, want := range map[string]string{
		"Permanent.nonLand":             "artifact",
		"Permanent":                     "land",
		"Land":                          "land",
		"Artifact,Enchantment":          "artifact",
		"Enchantment":                   "enchantment",
		"Creature":                      "",
		"Permanent.nonLand+nonArtifact": "enchantment",
	} {
		if got := landkillClass(tg); got != want {
			t.Errorf("landkillClass(%q) = %q, want %q", tg, got, want)
		}
	}
}
