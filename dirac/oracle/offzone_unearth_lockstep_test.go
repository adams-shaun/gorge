package oracle

import (
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted unearth lockstep (CR 702.84a): Sedris, the Traitor King's
// AffectedZone$ Graveyard AddKeyword$ Unearth static gives each creature card
// in its controller's graveyard unearth {2}{B}, and the kernel offers the
// activation exactly as gorge's derived-keyword walk offers it: the printed
// kwUnearth expander's body from the graveyard at sorcery speed, the return
// moving the card and applying the haste and end-step-exile riders.
//
// The pool is the off-battlefield statics fixture (offzonegrants). The
// vanilla creatures die in combat into the graveyard, where Sedris grants
// them unearth, and the policy plays the granted activations so the riders
// are exercised over the seeds.
const offZoneUnearthPool = "offzonegrants"

var offZoneUnearthSeen atomic.Int64

// offZoneUnearthDecks: the bears die in combat into p0's graveyard, where
// Sedris grants them unearth {2}{B} and the Swamps pay it; p1's vanilla
// creatures attack so something dies on both sides. Sedris casts for
// {3}{U}{B}{R}, so p0 runs Islands beside the Swamps and Mountains.
var offZoneUnearthDecks = [2][]cardset.Entry{
	{
		{Name: "Sedris, the Traitor King", Count: 4}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Swamp", Count: 8}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Raging Goblin", Count: 10}, {Name: "Hill Giant", Count: 8},
		{Name: "Mountain", Count: 16}, {Name: "Swamp", Count: 6},
	},
}

// offZoneUnearthPolicy is castFirstPolicy biased to the granted unearth
// activation: it takes any granted unearth activation gorge offers (3 in 4
// picks), otherwise prefers casting Sedris, the Traitor King and attacking,
// so the seeds reach the board the offer needs (the king out, a creature
// dead into its controller's graveyard, mana for {2}{B}) and exercise the
// return and its riders rather than the board racing.
func offZoneUnearthPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x0ff21e))
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		var ret, sed, atk []Canon
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			switch {
			case o.Kind == "ability" && strings.HasPrefix(o.Keyword, "Unearth"):
				ret = append(ret, x)
			case o.Kind == "cast":
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Card != nil && ob.Card.Faces[0].Name == "Sedris, the Traitor King" {
					sed = append(sed, x)
				}
			}
			if x.Kind() == ActAttack {
				atk = append(atk, x)
			}
		}
		if len(ret) > 0 && r.IntN(4) != 0 {
			offZoneUnearthSeen.Add(1)
			return ret[r.IntN(len(ret))]
		}
		if len(sed) > 0 && r.IntN(4) != 0 {
			return sed[r.IntN(len(sed))]
		}
		if len(atk) > 0 && r.IntN(4) != 0 {
			return atk[r.IntN(len(atk))]
		}
		return base.Pick(c)
	})
}

func TestOffZoneOfferLockstep(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, offZoneUnearthPool, offZoneUnearthDecks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("Sedris's deck is not playable in the offzonegrants pool")
	}
	before := dirac.UnearthGrantStats.Activations.Load()
	offZoneUnearthSeen.Store(0)
	lockstepRunPolicy(t, offZoneUnearthPool, offZoneUnearthDecks, 64, true, nil, nil, offZoneUnearthPolicy)
	if n := dirac.UnearthGrantStats.Activations.Load() - before; n == 0 {
		t.Fatal("no granted unearth activation ran; the lockstep proves nothing")
	} else {
		t.Logf("granted unearth activations: %d", n)
	}
}
