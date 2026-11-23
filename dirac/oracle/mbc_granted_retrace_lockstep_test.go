package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.81a with CR 702.66a: Six gives the nonland permanent cards in its
// controller's graveyard retrace during that player's turn, and a granted
// retrace cast of a Delve card (Gurmag Angler) still delves: the retrace
// cast pays the printed cost plus the land discard, and delve applies to the
// spell on the stack whatever zone it was cast from. gorge offers the cast
// with the graveyard credit; the kernel once withheld it as an unmodelled
// retracegrant stall (dr-mbc-bbab pair 0, seed 629502, autopay).
func TestMBCDrMbcBbabReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_retrace_delve")
	// CR 702.185a with CR 702.81a: warp is a hand-only permission, so a granted
	// retrace cast of a Warp card (Codecracker Hound) is the plain one -- the
	// printed cost plus the land discard. The kernel once withheld it as an
	// unmodelled retracegrant stall (dr-mbc-bbab pair 59, seed 447610, autopay).
	stdReproNamed(t, "granted_retrace_warp")
}

// grantedRetraceDelveDecks: Six mills on attack and the land discard feeds
// the graveyard, and Dark Ritual floats the mana (mana abilities are never
// options, and a delve cast is offered only against the floating pool), so
// a binned Gurmag Angler is retrace-cast with delve.
var grantedRetraceDelveDecks = [2][]cardset.Entry{
	{
		{Name: "Six", Count: 6}, {Name: "Gurmag Angler", Count: 6}, {Name: "Dark Ritual", Count: 8},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Leatherback Baloth", Count: 8}, {Name: "Timberland Ancient", Count: 6},
		{Name: "Forest", Count: 26},
	},
}

// grantedRetraceDelvePolicy is castFirstPolicy that holds Dark Ritual until
// its caster controls Six in its own main phase, with the stack empty and a
// Gurmag Angler binned (the retrace cast's sorcery timing), then
// casts it, and takes a retrace cast of Gurmag Angler whenever gorge offers
// one, counting those.
func grantedRetraceDelvePolicy(n *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		r := rand.New(rand.NewPCG(seed, 0x6a6e676c))
		named := func(c *PickCtx, x Canon, name string) bool {
			o, ok := c.G.GorgeOption(x)
			if !ok || o.Kind != "cast" {
				return false
			}
			ob := c.G.E.G.Obj(o.Obj)
			return ob != nil && ob.Face() != nil && ob.Face().Name == name
		}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil {
				return base.Pick(c)
			}
			g := c.G.E.G
			p := state.PlayerID(c.D.Player)
			in := func(z state.Zone, name string) bool {
				for _, id := range g.Zone(z, p) {
					if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == name {
						return true
					}
				}
				return false
			}
			ready := g.Active == p && g.Step.IsMain() && len(g.Stack) == 0 && in(state.ZBattlefield, "Six") && in(state.ZGraveyard, "Gurmag Angler")
			var ret, rit, rest []Canon
			for _, x := range c.D.Options {
				o, ok := c.G.GorgeOption(x)
				switch {
				case ok && o.Mode == "retrace" && named(c, x, "Gurmag Angler"):
					ret = append(ret, x)
				case named(c, x, "Dark Ritual"):
					rit = append(rit, x)
				default:
					rest = append(rest, x)
				}
			}
			if len(ret) > 0 && r.IntN(8) != 0 {
				*n++
				return ret[r.IntN(len(ret))]
			}
			if ready && len(rit) > 0 {
				return rit[r.IntN(len(rit))]
			}
			if len(rit) > 0 && len(rest) > 0 {
				d := *c.D
				d.Options = rest // hold the ritual for the retrace cast
				return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			return base.Pick(c)
		})
	}
}

// TestGrantedRetraceDelveLockstep: the granted retrace cast of a Delve card is
// offered, announced, delved and paid identically on both sides.
func TestGrantedRetraceDelveLockstep(t *testing.T) {
	_, set, gd := lockstepPool(t, "mbc", grantedRetraceDelveDecks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("precondition: Six and Gurmag Angler's deck is not playable in the mbc pool")
	}
	var angler int
	before := dirac.RetraceGrantStats.Casts.Load()
	lockstepRunPolicy(t, "mbc", grantedRetraceDelveDecks, 64, true, nil, nil, grantedRetraceDelvePolicy(&angler))
	n := dirac.RetraceGrantStats.Casts.Load() - before
	if n == 0 || angler == 0 {
		t.Fatalf("granted retrace casts %d, Gurmag Angler retrace casts taken %d: the lockstep proves nothing", n, angler)
	}
	t.Logf("granted retrace casts: %d, Gurmag Angler retrace casts taken: %d", n, angler)
}

// grantedRetraceKickerDecks: Six mills on attack and the land discard feeds
// the graveyard, and Dark Ritual floats the mana (mana abilities are never
// options, and the retrace cast is offered only against the floating pool),
// so a binned Gatekeeper of Malakir ({B}{B}, Kicker {B}) is retrace-cast. The
// plain cast asks no target: its ETB trigger reads "if it was kicked".
var grantedRetraceKickerDecks = [2][]cardset.Entry{
	{
		{Name: "Six", Count: 6}, {Name: "Gatekeeper of Malakir", Count: 6}, {Name: "Dark Ritual", Count: 8},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Leatherback Baloth", Count: 8}, {Name: "Timberland Ancient", Count: 6},
		{Name: "Forest", Count: 26},
	},
}

// grantedRetraceKickerPolicy is castFirstPolicy that holds Dark Ritual until
// its caster controls Six in its own main phase, with the stack empty and a
// Gatekeeper of Malakir binned (the retrace cast's target needs the ritual's
// float), then casts it, and takes a retrace cast of Gatekeeper of Malakir
// whenever gorge offers one, counting those.
func grantedRetraceKickerPolicy(n *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		r := rand.New(rand.NewPCG(seed, 0x6b69636b))
		named := func(c *PickCtx, x Canon, name string) bool {
			o, ok := c.G.GorgeOption(x)
			if !ok || o.Kind != "cast" {
				return false
			}
			ob := c.G.E.G.Obj(o.Obj)
			return ob != nil && ob.Face() != nil && ob.Face().Name == name
		}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil {
				return base.Pick(c)
			}
			g := c.G.E.G
			p := state.PlayerID(c.D.Player)
			in := func(z state.Zone, name string) bool {
				for _, id := range g.Zone(z, p) {
					if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == name {
						return true
					}
				}
				return false
			}
			ready := g.Active == p && g.Step.IsMain() && len(g.Stack) == 0 && in(state.ZBattlefield, "Six") && in(state.ZGraveyard, "Gatekeeper of Malakir")
			var ret, rit, rest []Canon
			for _, x := range c.D.Options {
				o, ok := c.G.GorgeOption(x)
				switch {
				case ok && o.Mode == "retrace" && named(c, x, "Gatekeeper of Malakir"):
					ret = append(ret, x)
				case named(c, x, "Dark Ritual"):
					rit = append(rit, x)
				default:
					rest = append(rest, x)
				}
			}
			if len(ret) > 0 && r.IntN(8) != 0 {
				*n++
				return ret[r.IntN(len(ret))]
			}
			if ready && len(rit) > 0 {
				return rit[r.IntN(len(rit))]
			}
			if len(rit) > 0 && len(rest) > 0 {
				d := *c.D
				d.Options = rest // hold the ritual for the retrace cast
				return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			return base.Pick(c)
		})
	}
}

// TestGrantedRetraceKickerLockstep: the granted retrace cast of a Kicker card
// is the plain cast -- the printed cost plus the land discard, the kicker
// unpaid -- offered, announced and paid identically on both sides, with no
// kicked retrace cast offered on either (a kicked cast is a hand-only offer).
func TestGrantedRetraceKickerLockstep(t *testing.T) {
	_, set, gd := lockstepPool(t, "mbc", grantedRetraceKickerDecks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("precondition: Six and Gatekeeper of Malakir's deck is not playable in the mbc pool")
	}
	var gate int
	before := dirac.RetraceGrantStats.Casts.Load()
	lockstepRunPolicy(t, "mbc", grantedRetraceKickerDecks, 64, true, nil, nil, grantedRetraceKickerPolicy(&gate))
	n := dirac.RetraceGrantStats.Casts.Load() - before
	if n == 0 || gate == 0 {
		t.Fatalf("granted retrace casts %d, Gatekeeper of Malakir retrace casts taken %d: the lockstep proves nothing", n, gate)
	}
	t.Logf("granted retrace casts: %d, Gatekeeper of Malakir retrace casts taken: %d", n, gate)
}
