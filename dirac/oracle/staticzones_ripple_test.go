package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The AffectedZone$ Stack grant of ripple 4 (dirac/kgen/lower_stackkw.go):
// Thrumming Stone over duplicated cheap spells, so the top four cards of the
// caster's library often hold one that shares the cast spell's name. No card
// in either deck prints Ripple: gorge's printed K:Ripple expansion
// (engine/cards/kw_ripple.go) fires a trigger the kernel does not lower (out
// of scope: no standard-set card prints it), so a pool card with the printed
// keyword would diverge on its own.

// kstaticzonesRipple: Thrumming Stone ({5}, a legendary artifact every colour
// can cast) beside four-of cheap spells and the rocks; the opposing deck is
// the shared pool's burn deck.
var kstaticzonesRipple = [2][]cardset.Entry{
	{
		{Name: "Thrumming Stone", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Opt", Count: 4}, {Name: "Giant Growth", Count: 4},
		{Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	kstaticzonesDecks[1],
}

// rippleCastOrder is the policy's cast preference at priority: the grant's
// carrier first (no ripple without it), then the rocks (they speed the
// carrier out), then the cheap four-ofs (three same-name cards stay in the
// library behind each cast).
var rippleCastOrder = []string{"Thrumming Stone", "Sol Ring", "Mind Stone", "Lightning Bolt", "Shock", "Opt", "Giant Growth"}

// rippleStats is the probe's tally.
type rippleStats struct {
	may      int // the CR 603.5 optional ask posed at a granted ripple trigger's resolution (ChOptional)
	declined int // a may answered No: nothing resolves and no reveal follows
	election int // the free-cast election over the revealed same-name cards (ChGenCastFree)
	cast     int // a candidate picked over Done: the free cast begins (CR 608.2g)
	dropped  int // an election answered Done: every revealed card stays and bottoms
}

// rippleProbe drives the pool under a random walk and answers the ripple
// asks the way the rule intends (the cascade probe, cascade_test.go): the
// may is accepted (a decline resolves nothing and no reveal follows), and a
// free-cast election candidate is picked over Done. declineMay /
// declineElection exercise the two decline arms on a seed fraction, the way
// the cascade lockstep does.
type rippleProbe struct {
	st              *rippleStats
	base            Policy
	rs              *realSide
	declineMay      bool
	declineElection bool
}

func (p *rippleProbe) pick(c *PickCtx) Canon {
	switch p.rs.d.Sub {
	case dirac.ChOptional:
		p.st.may++
		if p.declineMay {
			for _, a := range c.D.Options {
				if a.Kind() == ActNo {
					p.st.declined++
					return a
				}
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActYes {
				return a
			}
		}
	case dirac.ChGenCastFree:
		p.st.election++
		if p.declineElection {
			p.st.dropped++
			return Pack(ActDone, RefNone, RefNone, 0)
		}
		for _, a := range c.D.Options {
			if a.Kind() != ActDone {
				p.st.cast++
				return a
			}
		}
		return Pack(ActDone, RefNone, RefNone, 0)
	}
	if c.G != nil && c.D.Kind == DPriority {
		for _, nm := range rippleCastOrder {
			for _, x := range c.D.Options {
				if x.Kind() == ActCast && refCardName(c.G, x.Src()) == nm {
					return x
				}
			}
		}
	}
	return p.base.Pick(c)
}

// staticZonesRippleRun plays seeds 1..seeds of the ripple pool. It returns
// whether a game diverged and the probe's tally.
func staticZonesRippleRun(t *testing.T, seeds uint64, report bool) (diverged bool, st rippleStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, "staticzones", kstaticzonesRipple)
	tbl, _, _, err := set.Matchup(kstaticzonesRipple[0], kstaticzonesRipple[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"staticzones-a", "staticzones-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &rippleProbe{st: &st, base: RandomPolicy(seed), rs: k.(*realSide),
			declineMay: seed%8 == 0, declineElection: seed%4 == 0 && seed%8 != 0}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

// TestStaticZonesRippleLockstep: every granted ripple resolution runs the
// kernel's native body -- the reveal, the may-cast election (its picks begin
// zero-mana-cost casts of the revealed same-name cards) and the CR 401.4
// arrangement of the revealed cards that stay.
func TestStaticZonesRippleLockstep(t *testing.T) {
	t.Parallel()
	diverged, st := staticZonesRippleRun(t, 200, true)
	t.Logf("ripple tally %+v", st)
	if diverged {
		return // reported above
	}
	if st.may < 10 || st.election < 10 || st.cast < 5 || st.declined < 2 || st.dropped < 2 {
		t.Errorf("ripple tally %+v, want may >= 10, election >= 10, cast >= 5, declined >= 2, dropped >= 2", st)
	}
}
