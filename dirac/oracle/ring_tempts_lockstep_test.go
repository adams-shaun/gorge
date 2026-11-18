package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// ktemptDecks: the Ring (CR 701.54). Call of the Ring tempts at each upkeep,
// the other tempters tempt on resolution, Frodo and One Ring to Rule Them All
// read the count and the bearer, and creatures of different power give the
// bearer a real choice (and the evasion something to bite on).
func ktemptDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Call of the Ring", Count: 3},
		{Name: "Boromir, Warden of the Tower", Count: 1},
		{Name: "Frodo, Adventurous Hobbit", Count: 1},
		{Name: "Inherited Envelope", Count: 1},
		{Name: "Claim the Precious", Count: 2},
		{Name: "Soothing of Sméagol", Count: 1},
		{Name: "Birthday Escape", Count: 1},
		{Name: "Samwise the Stouthearted", Count: 1},
		{Name: "Slip On the Ring", Count: 1},
		{Name: "War of the Last Alliance", Count: 1},
		{Name: "One Ring to Rule Them All", Count: 1},
		{Name: "Savannah Lions", Count: 3},
		{Name: "Walking Corpse", Count: 3},
		{Name: "Wind Drake", Count: 2},
		{Name: "Vampire Nighthawk", Count: 2},
		{Name: "Serra Angel", Count: 2},
		{Name: "Plains", Count: 5},
		{Name: "Swamp", Count: 5},
		{Name: "Island", Count: 4},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// ktemptPolicy casts, activates and plays lands whenever it can and declares
// every attack it is offered, so the Ring-bearer attacks, is blocked and
// connects; everything else is random. It records, from gorge's state, the
// highest tempted count, the bearer asks (and those posed after a bearer left
// play), and each level's emblem trigger the first time it is on the stack.
func ktemptPolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		bearer := map[state.PlayerID]state.ObjID{}
		died := map[state.PlayerID]bool{}
		seen := map[state.ObjID]bool{}
		note := func(k string) {
			if *st == nil {
				*st = map[string]int{}
			}
			(*st)[k]++
		}
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				g := c.G.E.G
				for p := range g.Players {
					pid := state.PlayerID(p)
					if n := int(g.Players[p].RingTempted); n > 0 && !counted[-1-p*100-n] {
						counted[-1-p*100-n] = true
						note("tempted>=" + string(rune('0'+min(n, 9))))
					}
					b := g.Players[p].RingBearer
					if bearer[pid] != 0 && b == 0 {
						died[pid] = true
					}
					bearer[pid] = b
				}
				for _, id := range g.Stack {
					if o := g.Obj(id); o != nil && !seen[id] && o.Source == 0 && o.Ability != nil {
						seen[id] = true
						switch o.Ability.API {
						case "Draw":
							note("level2-loot")
						case "RingBlockerSacrifice":
							note("level3-blocker-sacrifice")
						case "LoseLife":
							note("level4-lose-life")
						}
					}
				}
				if c.D.Kind == DChoose && !counted[c.Seq] {
					for _, a := range c.D.Options {
						if o, ok := c.G.GorgeOption(a); ok && o.Kind == "ring_bearer" {
							counted[c.Seq] = true
							note("bearer-ask")
							if died[state.PlayerID(c.D.Player)] {
								note("bearer-ask-after-death") // the asked seat had a Ring-bearer that has since left play
							}
							break
						}
					}
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack:
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

// TestRingTemptsLockstep: the kernel's Ring (dirac/act_ring.go) agrees with
// gorge decision by decision over seeded games where Call of the Ring tempts
// each upkeep: the bearer is chosen (a real ask over two or more creatures),
// reaches each level in order, a new bearer is asked for once the old one has
// left play, and the emblem's loot, blocker-sacrifice and drain triggers
// resolve alike. Zero divergences is the assertion; the coverage checks prove
// the games reach the Ring.
func TestRingTemptsLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "ringtempts", ktemptDecks(), 60, true, nil, nil, ktemptPolicy(st))
	for _, k := range []string{"tempted>=1", "tempted>=2", "tempted>=3", "tempted>=4", "bearer-ask", "bearer-ask-after-death", "level2-loot", "level3-blocker-sacrifice", "level4-lose-life"} {
		if (*st)[k] == 0 {
			t.Errorf("gorge never reached %q in the games; the lockstep does not cover it (%v)", k, *st)
		}
	}
	t.Logf("coverage: %v", *st)
}

// TestRingTemptsLockstepMutation: with the emblem's levels 2-4 dropped the
// lockstep must diverge, so the games do put the loot, the blocker sacrifice
// and the drain on gorge's stack.
func TestRingTemptsLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoRingLevels = true
	defer func() { dirac.DebugNoRingLevels = false }()
	if !lockstepDiverges(t, "ringtempts", ktemptDecks(), 60) {
		t.Fatal("no seed diverged with the Ring's levels dropped: the games never reach them")
	}
}
