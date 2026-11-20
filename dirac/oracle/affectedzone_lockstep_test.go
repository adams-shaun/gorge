package oracle

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The off-battlefield Continuous statics kgen now lowers
// (dirac/kgen/lower_offzone.go) in lockstep: Six's AffectedZone$ Graveyard
// Retrace grant, the one carrier whose grant changes a decision (the
// graveyard cast offer. The type and keyword grants are characteristic-only
// (the kernel projects them off the battlefield but offers no cast from them),
// so their decks read the projection: Ashes of the Fallen's graveyard chosen
// subtype (which must not reach a battlefield creature) and Leyline of
// Transformation's All-zone chosen subtype. Aminatou's hand miracle grant is
// not lowered: the lockstep showed gorge offering the granted miracle trigger
// (a trigger_optional where the kernel offers priority, in 10 of the first 21 seeds).
const affectedZonePool = "offzonegrants"

// affectedZoneSixDecks: the nonland permanents Six's grant targets reach the
// graveyard by dying, and the red creatures are cast from there for their
// printed cost plus the land discard (CR 702.81a).
var affectedZoneSixDecks = [2][]cardset.Entry{
	{
		{Name: "Six", Count: 6}, {Name: "Ornithopter", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 16},
	},
}

// affectedZonePolicy is castFirstPolicy biased to the granted retrace cast:
// it takes any retrace cast gorge offers (Ornithopter's retrace is free apart
// from the land discard, so no payment choice can diverge).
func affectedZonePolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x0ff20e))
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		var ret []Canon
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			if o.Kind == "cast" && o.Mode == "retrace" {
				ret = append(ret, x)
			}
		}
		if len(ret) > 0 && r.IntN(8) != 0 {
			return ret[r.IntN(len(ret))]
		}
		return base.Pick(c)
	})
}

func TestAffectedZoneLockstep(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, affectedZonePool, affectedZoneSixDecks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("Six's deck is not playable in the offzonegrants pool")
	}
	before := dirac.RetraceGrantStats.Casts.Load()
	lockstepRunPolicy(t, affectedZonePool, affectedZoneSixDecks, 64, true, nil, nil, affectedZonePolicy)
	if n := dirac.RetraceGrantStats.Casts.Load() - before; n == 0 {
		t.Fatal("no retrace cast was offered by the grant; the lockstep proves nothing")
	} else {
		t.Logf("granted retrace casts: %d", n)
	}
}

// affectedZoneAshesDecks: the creatures die in combat into the graveyard, where
// Ashes of the Fallen gives them the chosen type, while the same player's
// battlefield creatures must stay their own type (the grant is
// AffectedZone$ Graveyard, never a battlefield anthem).
var affectedZoneAshesDecks = [2][]cardset.Entry{
	{
		{Name: "Ashes of the Fallen", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 16},
	},
}

// affectedZoneLeylineDecks: Leyline of Transformation's All-zone grant over
// creatures in hand, on the stack, in the graveyard and on the battlefield.
var affectedZoneLeylineDecks = [2][]cardset.Entry{
	{
		{Name: "Leyline of Transformation", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Island", Count: 12}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 16},
	},
}

// affectedZoneChooseGiantPolicy is castFirstPolicy that answers a creature
// type choice with Giant. The chosen type shows in the projection only when
// it is a word of the pool's vocabulary a battlefield creature lacks, which a
// random pick among every creature type almost never is (a leak of the grant
// onto the battlefield showed in 1 game of 300): Giant is Hill Giant's type
// and not Grizzly Bears'.
func affectedZoneChooseGiantPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	h := labelValue("Giant")
	var kinds []CanonKind
	for _, w := range []string{"type", "creature_type"} {
		if code, ok := chooseCode(w); ok {
			kinds = append(kinds, ActChoose+CanonKind(code))
		}
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, x := range c.D.Options {
			if slices.Contains(kinds, x.Kind()) && x.Mode() == uint8(h) && x.Tgt() == Ref(h>>8) {
				return x
			}
		}
		return base.Pick(c)
	})
}

// affectedZoneCase is one grant carrier's lockstep: its decks and policy.
type affectedZoneCase struct {
	name   string
	decks  [2][]cardset.Entry
	policy func(uint64) Policy
}

// TestAffectedZoneGrantsLockstep runs the type and keyword grant carriers.
// A grant that leaks onto the battlefield or reads a wrong zone shows as a
// projection divergence (Ashes of the Fallen's seed 55 was the first).
func TestAffectedZoneGrantsLockstep(t *testing.T) {
	t.Parallel()
	for _, c := range []affectedZoneCase{
		{"Ashes", affectedZoneAshesDecks, affectedZoneChooseGiantPolicy},
		{"Leyline", affectedZoneLeylineDecks, affectedZoneChooseGiantPolicy},
	} {
		_, set, gd := lockstepPool(t, affectedZonePool, c.decks)
		if !SetDeckSupported(set, gd[0]) {
			t.Fatalf("%s's deck is not playable in the offzonegrants pool", c.name)
		}
		t.Run(c.name, func(t *testing.T) {
			reg, set, gd := lockstepPool(t, affectedZonePool, c.decks)
			clean, casts, div := affectedZoneSeeds(t, reg, set, c, gd)
			t.Logf("%s: %d clean, %d divergent, %d casts/activations", c.name, clean, div, casts)
		})
	}
}

// affectedZoneSeeds plays seeds 1..100 of a grant carrier's decks. Every seed
// plays now: an opening-hand Leyline's as-enters choice is asked while it is
// still in hand, so accepting the begin-the-game offer no longer diverges.
func affectedZoneSeeds(t *testing.T, reg *cards.Registry, set cardset.Set, c affectedZoneCase, gd [2][]*cards.Card) (clean, casts, div int) {
	t.Helper()
	return lockstepSeeds(t, reg, set, affectedZonePool, c.decks, gd, 1, 100, true, nil, nil, c.policy)
}
