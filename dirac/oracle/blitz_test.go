package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// K:Blitz (CR 702.152a), lowered from the printed keyword: the hand's
// alternative cost (offered when the floating pool pays it), haste, "when this
// creature dies, draw a card" and the sacrifice at the beginning of the next
// end step. Played decision by decision against gorge over the blitz pool;
// the removal-free decks still kill blitzers in combat, and four carriers
// carry their own dies trigger.

var blitzDecks = [2][]cardset.Entry{
	{
		{Name: "Mayhem Patrol", Count: 5}, {Name: "Riveteers Requisitioner", Count: 4},
		{Name: "Plasma Jockey", Count: 3}, {Name: "Night Clubber", Count: 4},
		{Name: "Girder Goons", Count: 3}, {Name: "Dark Ritual", Count: 3}, {Name: "Seething Song", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 7},
	},
	{
		{Name: "Caldaia Guardian", Count: 5}, {Name: "Caldaia Strongarm", Count: 4},
		{Name: "Riveteers Decoy", Count: 3}, {Name: "Workshop Warchief", Count: 3},
		{Name: "Mezzio Mugger", Count: 3}, {Name: "Pugnacious Pugilist", Count: 3}, {Name: "Channel the Suns", Count: 4},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 6},
	},
}

// blitzPolicy plays randomly but casts the mana rituals it holds (three times
// in four) and takes a blitzed cast the moment one is offered (three times in
// four): gorge prices no blitzed cast in its planner, so the option exists
// only while the floating pool pays the blitz cost. The casts map counts what
// was offered and taken.
func blitzPolicy(seed uint64, casts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x626c6974)) // "blit"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast || CastModeName(a.Mode()&15) != "blitzed" {
					continue
				}
				casts["blitz offered"]++
				if prng.IntN(4) != 0 {
					casts["blitzed"]++
					return a
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && blitzRituals[refCardName(c.G, a.Src())] && prng.IntN(4) != 0 {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

var blitzRituals = map[string]bool{"Dark Ritual": true, "Seething Song": true, "Channel the Suns": true}

func TestBlitzLockstep(t *testing.T) {
	t.Parallel()

	run := runFixtureLockstepPolicy(t, "blitz", []fixtureDecks{blitzDecks}, 200, true, nil, blitzPolicy)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["blitzed"] < 200 {
		t.Errorf("only %d blitzed casts", run.casts["blitzed"])
	}
}

// TestBlitzMutation: skipping the haste grant, the end-step sacrifice or the
// dies-draw record of a blitzed permanent must each make the lockstep diverge.
func TestBlitzMutation(t *testing.T) {
	for _, m := range []struct {
		what string
		flag *bool
	}{
		{"a blitzed permanent gains no haste", &dirac.DebugBlitzNoHaste},
		{"a blitzed permanent is not sacrificed", &dirac.DebugBlitzNoSac},
		{"a blitzed permanent draws nothing when it dies", &dirac.DebugBlitzNoDraw},
	} {
		t.Run(m.what, func(t *testing.T) {
			*m.flag = true
			defer func() { *m.flag = false }()
			run := runFixtureLockstepPolicy(t, "blitz", []fixtureDecks{blitzDecks}, 150, false, nil, blitzPolicy)
			run.requireDivergence(t, m.what)
		})
	}
}
