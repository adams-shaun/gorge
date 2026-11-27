package oracle

import (
	"github.com/adams-shaun/gorge/engine/state"
	"math/rand/v2"
	"testing"
)

// The hosted Commander Effect-static / replacement shapes (kgen
// lower_lifeprevent.go, lower_effectemblem.go) played decision by decision
// against gorge over the hostedcmdreffectstatic pool.

const hostedCmdrPool = "hostedcmdreffectstatic"

type hostedCmdrEffectStaticCase struct {
	name  string
	decks []fixtureDecks
	watch map[string]bool
	seeds int
	casts map[string]int // the least casts / activations each card must see
	// activate: cards whose activated ability is taken the moment it is
	// offered (three times in four), counted as "<card> activation" in casts
	activate map[string]bool
	// aim: cards a target decision picks when one is offered, so the
	// reanimate-bounce-recast line is played in most games
	aim map[string]bool
	// gate: a card whose cast is offered only while an aim card is in the zone
	gate map[string]state.Zone
	// stalls: the pair meets a declared kernel stall (a CR 616.1 order ask
	// the kernel does not pose) and must never diverge before it
	stalls bool
}

var hostedCmdrLife = []fixtureDecks{{
	{
		{Name: "Roiling Vortex", Count: 4}, {Name: "Sulfuric Vortex", Count: 4},
		{Name: "Healer's Hawk", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Lone Missionary", Count: 8}, {Name: "Healer's Hawk", Count: 6},
		{Name: "Soulmender", Count: 6}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 16},
	},
}}

var hostedCmdrSticker = []fixtureDecks{{
	{
		{Name: "\"Name Sticker\" Goblin", Count: 6}, {Name: "Zombify", Count: 3}, {Name: "Cloudshift", Count: 3},
		{Name: "Faithless Looting", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 6}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Lone Missionary", Count: 8}, {Name: "Healer's Hawk", Count: 6},
		{Name: "Soulmender", Count: 6}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 16},
	},
}}

// Sulfuric Vortex beside Rhox Faithmender: gorge keeps the prevention line a
// replacement (not a CantGainLife restriction), so it competes with the
// doubler in the CR 616.1 order ask, which the kernel's static stalls on.
var hostedCmdrLifePrevent = []fixtureDecks{{
	{
		{Name: "Sulfuric Vortex", Count: 4}, {Name: "Rhox Faithmender", Count: 6},
		{Name: "Healer's Hawk", Count: 4}, {Name: "Lone Missionary", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Rhox Faithmender", Count: 6}, {Name: "Lone Missionary", Count: 6},
		{Name: "Soulmender", Count: 6}, {Name: "Healer's Hawk", Count: 4},
		{Name: "Plains", Count: 18},
	},
}}

// A goblin reanimated (Reanimate) from the graveyard (no trigger), bounced to hand and
// recast the same turn (a trigger: it enters from the hand).
var hostedCmdrStickerBounce = []fixtureDecks{{
	{
		{Name: "\"Name Sticker\" Goblin", Count: 3}, {Name: "Reanimate", Count: 6}, {Name: "Unsummon", Count: 6},
		{Name: "Faithless Looting", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 6},
		{Name: "Island", Count: 5},
	},
	{
		{Name: "Lone Missionary", Count: 8}, {Name: "Healer's Hawk", Count: 6},
		{Name: "Soulmender", Count: 6}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 16},
	},
}}

var hostedCmdrNemesis = []fixtureDecks{{
	{
		{Name: "Screaming Nemesis", Count: 8}, {Name: "Healer's Hawk", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 14}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Lone Missionary", Count: 6},
		{Name: "Soulmender", Count: 6}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

var hostedCmdrEffectStaticCases = []hostedCmdrEffectStaticCase{
	{
		name: "life", decks: hostedCmdrLife, seeds: 200,
		watch:    map[string]bool{"Roiling Vortex": true, "Sulfuric Vortex": true, "Lone Missionary": true, "Soulmender": true},
		activate: map[string]bool{"Roiling Vortex": true},
		casts:    map[string]int{"Roiling Vortex": 5, "Sulfuric Vortex": 5, "Lone Missionary": 5, "Roiling Vortex activation": 5},
	},
	{
		name: "lifeprevent", decks: hostedCmdrLifePrevent, seeds: 200, stalls: true,
		watch: map[string]bool{"Sulfuric Vortex": true, "Rhox Faithmender": true, "Lone Missionary": true, "Healer's Hawk": true},
		casts: map[string]int{"Sulfuric Vortex": 5, "Rhox Faithmender": 20, "Lone Missionary": 5},
	},
	{
		name: "excludedorigins", decks: hostedCmdrSticker, seeds: 200,
		watch: map[string]bool{"\"Name Sticker\" Goblin": true, "Zombify": true, "Cloudshift": true, "Faithless Looting": true},
		casts: map[string]int{"\"Name Sticker\" Goblin": 20, "Zombify": 5, "Cloudshift": 5},
	},
	{
		name: "stickerbounce", decks: hostedCmdrStickerBounce, seeds: 300,
		watch: map[string]bool{"\"Name Sticker\" Goblin": true, "Reanimate": true, "Unsummon": true, "Faithless Looting": true},
		aim:   map[string]bool{"\"Name Sticker\" Goblin": true},
		gate:  map[string]state.Zone{"Reanimate": state.ZGraveyard, "Unsummon": state.ZBattlefield},
		casts: map[string]int{"\"Name Sticker\" Goblin": 20, "Reanimate": 5, "Unsummon": 5, "aimed ": 20},
	},
	{
		name: "nemesis", decks: hostedCmdrNemesis, seeds: 200,
		watch: map[string]bool{"Screaming Nemesis": true, "Lightning Bolt": true, "Lone Missionary": true, "Soulmender": true},
		casts: map[string]int{"Screaming Nemesis": 20, "Lightning Bolt": 20, "Lone Missionary": 5},
	},
}

// TestHostedCmdrEffectStaticLockstep: each carrier's pair plays clean (no
// divergence, no kernel stall) and reaches the cards it lowers.
func TestHostedCmdrEffectStaticLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, tc := range hostedCmdrEffectStaticCases {
		t.Run(tc.name, func(t *testing.T) {
			// the precondition the lockstep depends on: every deck plays in the
			// kernel (a refused card would make the pair a different game)
			for _, md := range tc.decks {
				_, set, gd := lockstepPool(t, hostedCmdrPool, md)
				for s, d := range gd {
					if !SetDeckSupported(set, d) {
						t.Fatalf("seat %d: a card of the %s deck is not playable in the %s pool", s, tc.name, hostedCmdrPool)
					}
				}
			}
			run := runFixtureLockstepPolicy(t, hostedCmdrPool, tc.decks, tc.seeds, !tc.stalls, nil, func(seed uint64, casts map[string]int) Policy {
				return hostedCmdrPolicy(seed, tc, casts)
			})
			t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
			if run.divs != 0 || tc.stalls && run.stalls == 0 || !tc.stalls && run.stalls != 0 {
				t.Errorf("%d divergences (%s), %d stalls (want some: %v)", run.divs, run.first, run.stalls, tc.stalls)
			}
			for n, want := range tc.casts {
				if run.casts[n] < want {
					t.Errorf("%s was cast %d times in %d games, want at least %d", n, run.casts[n], run.games, want)
				}
			}
		})
	}
}

// hostedCmdrPolicy is the watch policy, plus the case's activations taken
// the moment they are offered and counted, and its aimed picks: a target or
// discard choice of an aim card is taken when offered, and the cast of a card
// that needs an aim card in a zone (gate) is withheld until one is there.
func hostedCmdrPolicy(seed uint64, tc hostedCmdrEffectStaticCase, casts map[string]int) Policy {
	base := fixtureWatchPolicy(seed, tc.watch, casts)
	prng := rand.New(rand.NewPCG(seed, 0x61637476)) // "actv"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		switch c.D.Kind {
		case DPriority:
			for _, a := range c.D.Options {
				if n := refCardName(c.G, a.Src()); a.Kind() == ActActivate && tc.activate[n] && prng.IntN(4) != 0 {
					casts[n+" activation"]++
					return a
				}
			}
			if len(tc.gate) != 0 {
				d := *c.D
				d.Options = nil
				for _, a := range c.D.Options {
					if z, ok := tc.gate[refCardName(c.G, a.Src())]; a.Kind() == ActCast && ok && !hostedCmdrAimIn(c.G, tc.aim, z) {
						continue
					}
					d.Options = append(d.Options, a)
				}
				return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		case DTarget, DChoose:
			for _, a := range c.D.Options {
				for _, r := range []Ref{a.Tgt(), a.Src()} {
					if len(tc.aim) != 0 && int(r) < int(RefPlayer) && tc.aim[refCardName(c.G, r)] {
						casts["aimed "]++
						return a
					}
				}
			}
		}
		return base.Pick(c)
	})
}

// hostedCmdrAimIn reports an aim card in zone z on the table.
func hostedCmdrAimIn(g *GorgeSide, aim map[string]bool, z state.Zone) bool {
	for i := range g.E.G.Objs {
		if o := &g.E.G.Objs[i]; o.Zone == z && o.Card != nil && aim[o.Card.Faces[0].Name] {
			return true
		}
	}
	return false
}
