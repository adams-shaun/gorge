package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantCensusActivation: The World Tree's activation beside twelve
// lands its 6-land static granted "{T}: Add one mana of any color" is
// offered (gorge potentialAbilityPlan prices the {W}{W}{U}{U}{B}{B}{R}{R}
// {G}{G} from the join census, no prefix search; tail-08 pair 78, decision
// 912): the grant search's multiset walk over twelve any-colour sources
// exhausts gorge's node budget before WWUUBBRRGG, so a search-priced offer
// would be withheld and the priority menu would fall one activation short
// of gorge's.
func TestGrantCensusActivation(t *testing.T) {
	set := cardsettest.Synth(t, "tail-08")
	tbl, a, _, err := set.Matchup(
		[]cardset.Entry{{Name: "The World Tree", Count: 1}, {Name: "Plains", Count: 19}},
		[]cardset.Entry{{Name: "Plains", Count: 20}},
	)
	if err != nil {
		t.Fatal(err)
	}
	sc := newScenTableLib(t, tbl, a[1], Config{NoElide: true})
	wt := sc.perm(a[0], 0) // The World Tree
	for i := 0; i < 12; i++ {
		sc.perm(a[1], 0)
	}
	s, k := sc.s, sc.k
	for sl := range s.perms {
		s.perms[sl].flags &^= pfTapped // the fixtures enter untapped (the World Tree's ETB replacement)
	}
	if s.over || s.g.stalled != 0 {
		t.Fatalf("setup: over=%v stalled=%d", s.over, s.g.stalled)
	}
	if s.g.stalled == 0 && !s.grantGapLive(k, 0) {
		t.Fatalf("the grant gap is not live: the scenario tests the grant cap, not the offer walk")
	}
	// the join census prices the composed cost, the activation's own source
	// excluded (its {T} is the activation's tap)
	cd := &k.t.Cards[s.cardAt(s.objs[wt].slot)]
	var cost kop.ManaCost
	for ai := range cd.Abilities {
		if cd.Abilities[ai].Kind == kop.AbActivated {
			cost = cd.Abilities[ai].Cost.Mana
		}
	}
	if cost.MV() != 10 {
		t.Fatalf("The World Tree's activated cost is %v, want the printed {W}{W}{U}{U}{B}{B}{R}{R}{G}{G}", cost)
	}
	if !s.grantCensusPays(k, 0, MakeAction(AkActivate, TCode(wt), 0, 1), &cost, s.objs[wt].slot) {
		t.Fatalf("the join census does not price the activation's composed cost from twelve any-colour lands")
	}
	d := s.Step()
	offered := false
	for i := 0; i < int(d.N); i++ {
		if o := d.Options[i]; o.Kind() == AkActivate && o.Source() == TCode(wt) && o.Mode() == 1 {
			offered = true
		}
	}
	if !offered {
		t.Errorf("The World Tree's activation not offered beside twelve any-colour lands; opts %v", d.Options[:d.N])
	}
}
