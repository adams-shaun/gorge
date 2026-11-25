package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Effect-container MayPlay lockstep: a no-target Continuous MayPlay$
// static lowers to an emblem's MayPlayFrom permission (Chainer, Nightmare
// Adept's graveyard grant) and a remembered exiled grant with Duration$
// UntilYourNextTurn keeps its record through the opponent's turn (Nivix,
// Aerie of the Firemind); Mission Briefing's chosen-card grant casts the
// chosen graveyard card and its replacement exiles the spell when it leaves
// the stack. The fixture pool carries each carrier beside the creatures,
// burn, draw and basics the decks play; the policy activates or casts the
// carrier and takes a non-hand cast offer, so a missing permission shows up
// as a decision divergence or a missing activation.
const effectContainerMayPlayPool = "effectcontainermayplay"

var effectContainerMayPlayChainerDecks = [2][]cardset.Entry{
	{
		{Name: "Chainer, Nightmare Adept", Count: 4}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Hill Giant", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

var effectContainerMayPlayNivixDecks = [2][]cardset.Entry{
	{
		{Name: "Nivix, Aerie of the Firemind", Count: 4}, {Name: "Shock", Count: 12},
		{Name: "Divination", Count: 8}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// One Mission Briefing copy: the kernel's ExileOnMoved$ remembered grant is
// exact for one card (dirac/cast_mayplayremembered.go), and two concurrent
// Effect grants would conflate the global remembered pool into an unmodelled
// stall -- a limitation of the remembered-grant class, not of this lowering.
var effectContainerMayPlayMissionDecks = [2][]cardset.Entry{
	{
		{Name: "Mission Briefing", Count: 1}, {Name: "Divination", Count: 16}, {Name: "Island", Count: 23},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

var effectContainerMayPlayExtraTurnDecks = [2][]cardset.Entry{
	{
		{Name: "Nivix, Aerie of the Firemind", Count: 4}, {Name: "Time Walk", Count: 4},
		{Name: "Shock", Count: 8}, {Name: "Divination", Count: 6},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// One Yawgmoth's Will and one Magus of the Will copy: two concurrent
// grave-to-exile replacements compete on one move and pose gorge's CR 616.1
// order ask, which the kernel's emblem walk never poses (a stall), so the
// pool decks carry one carrier each, the Mission Briefing precedent.
var effectContainerMayPlayYawgDecks = [2][]cardset.Entry{
	{
		{Name: "Yawgmoth's Will", Count: 1}, {Name: "Shock", Count: 8},
		{Name: "Divination", Count: 8}, {Name: "Mountain", Count: 10},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 7},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

var effectContainerMayPlayMagusDecks = [2][]cardset.Entry{
	{
		{Name: "Magus of the Will", Count: 1}, {Name: "Shock", Count: 8},
		{Name: "Divination", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// effectContainerMayPlayStats counts the decisions the policy takes through
// the carrier: an activation, a non-hand cast (graveyard or exile), the
// carrier's own casts (a spell carrier's hand casts), and the casts of the
// prefer card (the extra-turn spell the policy casts before a granted
// zone cast).
type effectContainerMayPlayStats struct {
	activates, zoneCasts, carrierCasts, preferCasts int
}

// effectContainerMayPlayPolicy activates card whenever it is offered, then
// casts prefer (an empty name: none) over anything else, takes a
// graveyard/exile cast over any other action, and otherwise plays lands or a
// hand cast at random.
func effectContainerMayPlayPolicy(card, prefer string, st *effectContainerMayPlayStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		base := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			pick := func(opts []Canon) Canon {
				d := *c.D
				d.Options = opts
				return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			if c.D.Kind != DPriority || c.G == nil {
				return base.Pick(c)
			}
			var act, zone, land, cast, zoneCarrier, handCarrier, pre []Canon
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActActivate:
					if o, ok := c.G.GorgeOption(a); ok && effectContainerMayPlayName(c.G, o.Obj) == card {
						act = append(act, a)
					}
				case ActCast, ActCastFace:
					o, ok := c.G.GorgeOption(a)
					if !ok {
						continue
					}
					carrier := effectContainerMayPlayName(c.G, o.Obj) == card
					if prefer != "" && !carrier && effectContainerMayPlayName(c.G, o.Obj) == prefer {
						pre = append(pre, a)
					}
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && (ob.Zone == state.ZGraveyard || ob.Zone == state.ZExile) {
						zone = append(zone, a)
						if carrier {
							zoneCarrier = append(zoneCarrier, a)
						}
					} else {
						cast = append(cast, a)
						if carrier {
							handCarrier = append(handCarrier, a)
						}
					}
				case ActPlayLand:
					land = append(land, a)
				}
			}
			taken := func(opts []Canon, carriers []Canon) Canon {
				a := pick(opts)
				for _, z := range carriers {
					if z == a {
						st.carrierCasts++ // the carrier itself was cast
					}
				}
				return a
			}
			switch {
			case len(act) > 0:
				st.activates++
				return act[0]
			case len(pre) > 0:
				a := pick(pre)
				st.preferCasts++
				return a
			case len(zone) > 0:
				st.zoneCasts++
				return taken(zone, zoneCarrier)
			case len(land) > 0:
				return pick(land)
			case len(cast) > 0:
				return taken(cast, handCarrier)
			}
			return base.Pick(c)
		})
	}
}

func effectContainerMayPlayName(g *GorgeSide, id state.ObjID) string {
	if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

func effectContainerMayPlayRun(t *testing.T, decks [2][]cardset.Entry, card string) effectContainerMayPlayStats {
	t.Helper()
	st, clean, casts, div := effectContainerMayPlayGames(t, decks, card, "")
	if div > 0 {
		t.Fatalf("%s: %d of %d games diverged", card, div, clean+div)
	}
	if st.activates == 0 {
		t.Fatalf("%s was never activated; the lockstep proves nothing (%d casts/activations)", card, casts)
	}
	t.Logf("%s: %d clean games, %d activations, %d non-hand casts, %d casts/activations",
		card, clean, st.activates, st.zoneCasts, casts)
	return st
}

// effectContainerMayPlayRunCast is effectContainerMayPlayRun for a spell
// carrier: the path through the carrier is its own casts, not activations.
func effectContainerMayPlayRunCast(t *testing.T, decks [2][]cardset.Entry, card string) effectContainerMayPlayStats {
	t.Helper()
	st, clean, casts, div := effectContainerMayPlayGames(t, decks, card, "")
	if div > 0 {
		t.Fatalf("%s: %d of %d games diverged", card, div, clean+div)
	}
	if st.carrierCasts == 0 {
		t.Fatalf("%s was never cast; the lockstep proves nothing (%d casts/activations)", card, casts)
	}
	t.Logf("%s: %d clean games, %d carrier casts, %d non-hand casts, %d casts/activations",
		card, clean, st.carrierCasts, st.zoneCasts, casts)
	return st
}

func effectContainerMayPlayGames(t *testing.T, decks [2][]cardset.Entry, card, prefer string) (effectContainerMayPlayStats, int, int, int) {
	t.Helper()
	reg, set, gd := lockstepPool(t, effectContainerMayPlayPool, decks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatalf("%s's deck is not playable in the %s pool", card, effectContainerMayPlayPool)
	}
	var st effectContainerMayPlayStats
	clean, casts, div := lockstepSeeds(t, reg, set, effectContainerMayPlayPool, decks, gd, 1, 24, true, nil, nil,
		effectContainerMayPlayPolicy(card, prefer, &st))
	return st, clean, casts, div
}

func TestEffectContainerMayPlayLockstep(t *testing.T) {
	t.Parallel()
	t.Run("zone permission", func(t *testing.T) {
		// Chainer's emblem permission is the graveyard cast the activation
		// grants; the discarding activation puts a creature there.
		if st := effectContainerMayPlayRun(t, effectContainerMayPlayChainerDecks, "Chainer, Nightmare Adept"); st.zoneCasts == 0 {
			t.Fatal("Chainer's graveyard permission offered no cast")
		}
	})
	t.Run("remembered exile until next turn", func(t *testing.T) {
		// Nivix's activation exiles the top card; the policy casts it back
		// from exile while the until-your-next-turn record is live.
		if st := effectContainerMayPlayRun(t, effectContainerMayPlayNivixDecks, "Nivix, Aerie of the Firemind"); st.zoneCasts == 0 {
			t.Fatal("Nivix's exiled permission offered no cast")
		}
	})
	t.Run("chosen card cast and its stack exit", func(t *testing.T) {
		// Mission Briefing's Effect grants the chosen graveyard card's cast
		// (stackExileChosenMark's permission) and its replacement exiles the
		// spell when it leaves the stack: the policy casts Mission Briefing,
		// its surveil fills the graveyard, and the chosen Divination is cast
		// back from the graveyard, leaving the stack to exile.
		if st := effectContainerMayPlayRunCast(t, effectContainerMayPlayMissionDecks, "Mission Briefing"); st.zoneCasts == 0 {
			t.Fatal("Mission Briefing's chosen-card permission offered no cast")
		}
	})
	t.Run("graveyard-to-exile replacement", func(t *testing.T) {
		// The Will group's emblem carries the graveyard permission beside the
		// any-zone dies-to-exile replacement: Yawgmoth's Will's cast fills
		// the graveyard with the spells played so far, the policy casts them
		// back, and the card it exiles instead of a graveyard (the Will
		// itself, a Shock played back and discarded) is the replacement's
		// read; Magus of the Will activates the same shape from the
		// battlefield. Gaea's Will lowers identically (the kgen table); its
		// suspend cast is not reached by any policy here.
		if st := effectContainerMayPlayRunCast(t, effectContainerMayPlayYawgDecks, "Yawgmoth's Will"); st.zoneCasts == 0 {
			t.Fatal("Yawgmoth's Will's graveyard permission offered no cast")
		}
		if st := effectContainerMayPlayRun(t, effectContainerMayPlayMagusDecks, "Magus of the Will"); st.zoneCasts == 0 {
			t.Fatal("Magus of the Will's graveyard permission offered no cast")
		}
	})
	t.Run("until next turn ends before an extra turn", func(t *testing.T) {
		// Nivix's grant is registered on the controller's turn; a Time Walk
		// resolving the same turn makes the controller's turn the next one,
		// so the grant ends at that turn's cleanup and the exiled card is
		// never offered during the extra turn. The policy casts the Time
		// Walk before the granted cast so the record survives to the
		// boundary; the until-next-turn boundary is the counter-scenario the
		// `active == controller` keep arm got wrong.
		reg, set, gd := lockstepPool(t, effectContainerMayPlayPool, effectContainerMayPlayExtraTurnDecks)
		if !SetDeckSupported(set, gd[0]) {
			t.Fatal("the extra-turn deck is not playable in the pool")
		}
		var st effectContainerMayPlayStats
		clean, casts, div := lockstepSeeds(t, reg, set, effectContainerMayPlayPool, effectContainerMayPlayExtraTurnDecks, gd, 1, 24, true, nil, nil,
			effectContainerMayPlayPolicy("Nivix, Aerie of the Firemind", "Time Walk", &st))
		if div > 0 {
			t.Fatalf("Nivix + Time Walk: %d of %d games diverged", div, clean+div)
		}
		if st.activates == 0 || st.preferCasts == 0 {
			t.Fatalf("the scenario never ran (%d activations, %d Time Walks over %d casts/activations)",
				st.activates, st.preferCasts, casts)
		}
	})
}
