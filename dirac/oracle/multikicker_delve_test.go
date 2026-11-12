package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Teval, Arbiter of Virtue grants Delve to every spell its controller casts
// (CR 702.66a). gorge's multikicked offer is the floating-pool gate priced
// with that delve credit folded in (offerCastableUsing -> delveCredit) and
// its potential walk declines the credit-bearing kicked mode, so the kernel
// must offer the kicked cast only when the pool reduced by the graveyard
// pays base + one payment, never against the untapped sources alone. The
// class lockstep's decks have no graveyard and no grant, so this shape needs
// its own fixture: Apex Hawks (Multikicker {1}{W}) beside Teval, with the
// basics supplying both costs.
func TestMultikickerGrantedDelveLockstep(t *testing.T) {
	t.Parallel()
	d := []cardset.Entry{
		{Name: "Apex Hawks", Count: 4}, {Name: "Teval, Arbiter of Virtue", Count: 6},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 7},
		{Name: "Swamp", Count: 7}, {Name: "Forest", Count: 6},
	}
	runMultikickerDelve(t, d, "Apex Hawks", "")
}

// The positive half: a multikicked Everflowing Chalice (Multikicker {2}, cost
// {0}) is offered on the graveyard credit alone, with Dark Ritual's mana
// floating. The oracle's lowering witness must price that same credit, or it
// taps (and floats) the credit's worth of extra mana and the projections
// diverge at the next decision.
func TestMultikickerGrantedDelvePositiveLockstep(t *testing.T) {
	t.Parallel()
	d := []cardset.Entry{
		{Name: "Everflowing Chalice", Count: 4}, {Name: "Teval, Arbiter of Virtue", Count: 6},
		{Name: "Dark Ritual", Count: 8}, {Name: "Swamp", Count: 8},
		{Name: "Island", Count: 7}, {Name: "Forest", Count: 7},
	}
	tl := runMultikickerDelve(t, d, "Everflowing Chalice", "Dark Ritual")
	if tl.casts["Everflowing Chalice"] == 0 {
		t.Error("no multikicked Everflowing Chalice was cast: the positive credit offer was never taken")
	}
}

func runMultikickerDelve(t *testing.T, d []cardset.Entry, want, ritual string) *mkDelveTally {
	t.Helper()
	tl := &mkDelveTally{casts: map[string]int{}, counts: map[int]int{}}
	runMultikicker(t, [2][]cardset.Entry{d, d}, 24, func(seed uint64) Policy {
		return mkDelvePolicy(seed, tl, want, ritual)
	})
	t.Logf("multikicked casts %v; counts %v; live-with-%s decisions %d; Teval casts %d",
		tl.casts, tl.counts, want, tl.liveWithHawks, tl.teval)
	if tl.teval == 0 {
		t.Error("Teval was never cast: the delve grant was never live")
	}
	if tl.liveWithHawks == 0 {
		t.Errorf("no decision had Teval's delve grant live with %s still in hand", want)
	}
	return tl
}

// mkDelveTally counts the states the delve fixture depends on.
type mkDelveTally struct {
	casts         map[string]int
	counts        map[int]int
	teval         int
	liveWithHawks int
}

// mkDelvePolicy casts Teval before any other spell and holds every Apex
// Hawks cast (plain or kicked) until the grant is live: the offer gate under
// test is the pool-only one, so an Apex Hawks cast before Teval would spend
// the card the grant's credit applies to. A kicked cast taken while Teval is
// live is the positive case; without the pool the kicked offer must not
// exist at all (the pre-fix divergence this fixture found).
func mkDelvePolicy(seed uint64, tl *mkDelveTally, want, ritual string) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6d6b646c))
	kick, _ := castModeCode("kicked")
	var opts []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers || c.D.Kind == DBlockers {
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
			return rnd.Pick(c)
		}
		if code, ok := chooseCode("multikick"); ok {
			opts = opts[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActChoose+CanonKind(code) {
					opts = append(opts, a)
				}
			}
			if len(opts) > 0 {
				if len(opts) > 1 && r.IntN(3) != 0 {
					a := opts[1+r.IntN(len(opts)-1)]
					tl.counts[int(a.Mode())]++
					return a
				}
				tl.counts[0]++
				return opts[0]
			}
		}
		// Teval first: its static is the grant the fixture is about.
		for _, a := range c.D.Options {
			if a.Kind() != ActCast || a.Mode() != 0 || mkCastName(c, a) != "Teval, Arbiter of Virtue" {
				continue
			}
			tl.teval++
			return a
		}
		live := mkTevalOnField(c)
		if live && mkHandHas(c, want) {
			tl.liveWithHawks++
			// float the ritual's mana before the kicked cast, so the pool
			// holds mana beside the graveyard credit
			if ritual != "" {
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && a.Mode() == 0 && mkCastName(c, a) == ritual {
						return a
					}
				}
			}
			// A kicked Apex Hawks can only be the pool-plus-delve offer once
			// the grant is live (the potential walk declines the credit).
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && a.Mode()&0x0f == kick && mkCastName(c, a) == want {
					tl.casts[want]++
					return a
				}
			}
		}
		// Hold every Apex Hawks cast until the grant is live: an early one
		// would leave no multikicker card for the credit.
		opts = opts[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && mkCastName(c, a) == want {
				continue
			}
			opts = append(opts, a)
		}
		if len(opts) == 0 {
			return rnd.Pick(c)
		}
		return opts[r.IntN(len(opts))]
	})
}

// mkCastName is the name of the card a cast option casts, "" for anything
// else.
func mkCastName(c *PickCtx, a Canon) string {
	if c.G == nil || a.Kind() != ActCast {
		return ""
	}
	o, ok := c.G.GorgeOption(a)
	if !ok || o.Obj == 0 {
		return ""
	}
	obj := c.G.E.G.Obj(o.Obj)
	if obj == nil || obj.Face() == nil {
		return ""
	}
	return obj.Face().Name
}

// mkHandHas reports whether the acting seat holds a card named name.
func mkHandHas(c *PickCtx, name string) bool {
	if c.G == nil {
		return false
	}
	for _, id := range c.G.E.G.Zone(state.ZHand, state.PlayerID(c.D.Player)) {
		o := c.G.E.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// mkTevalOnField reports whether the acting seat controls Teval, so a spell
// it casts carries the granted Delve.
func mkTevalOnField(c *PickCtx) bool {
	if c.G == nil {
		return false
	}
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
		o := c.G.E.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "Teval, Arbiter of Virtue" {
			return true
		}
	}
	return false
}
