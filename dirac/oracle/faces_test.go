package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The multi-face lockstep: constructed matchups over the
// generated faces pool, so the expected answer at every decision is
// gorge's. A modal DFC's back land play ("modal_land") is preferred when
// offered, so the games reach the back faces.

var facesDecks = [][2][]cardset.Entry{{
	{
		{Name: "Akoum Warrior", Count: 2}, {Name: "Kazuul's Fury", Count: 2}, {Name: "Song-Mad Treachery", Count: 1},
		{Name: "Kazandu Mammoth", Count: 2}, {Name: "Khalni Ambush", Count: 2}, {Name: "Tangled Florahedron", Count: 2},
		{Name: "Vastwood Fortification", Count: 2}, {Name: "Stump Stomp", Count: 2}, {Name: "Bala Ged Recovery", Count: 1},
		{Name: "Cragcrown Pathway", Count: 2}, {Name: "Branchloft Pathway", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Gray Ogre", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Skyclave Cleric", Count: 2}, {Name: "Blackbloom Rogue", Count: 2}, {Name: "Malakir Rebirth", Count: 2},
		{Name: "Umara Wizard", Count: 2}, {Name: "Jwari Disruption", Count: 2}, {Name: "Kabira Takedown", Count: 2},
		{Name: "Zof Consumption", Count: 1}, {Name: "Makindi Stampede", Count: 1}, {Name: "Beyeen Veil", Count: 1},
		{Name: "Silundi Vision", Count: 1}, {Name: "Brightclimb Pathway", Count: 2}, {Name: "Clearwater Pathway", Count: 1},
		{Name: "Darkbore Pathway", Count: 1}, {Name: "Savannah Lions", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Coral Merfolk", Count: 2}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 4},
	},
}, {
	{
		{Name: "Barkchannel Pathway", Count: 2}, {Name: "Riverglide Pathway", Count: 2}, {Name: "Needleverge Pathway", Count: 2},
		{Name: "Bala Ged Recovery", Count: 2}, {Name: "Revitalizing Repast", Count: 2}, {Name: "Ondu Inversion", Count: 1},
		{Name: "Silundi Vision", Count: 1}, {Name: "Giant Spider", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Coral Merfolk", Count: 3}, {Name: "Gray Ogre", Count: 3}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Blightstep Pathway", Count: 2}, {Name: "Hengegate Pathway", Count: 2}, {Name: "Darkbore Pathway", Count: 2},
		{Name: "Malakir Rebirth", Count: 2}, {Name: "Zof Consumption", Count: 2}, {Name: "Makindi Stampede", Count: 2},
		{Name: "Kabira Takedown", Count: 2}, {Name: "Walking Corpse", Count: 2}, {Name: "Midnight Scavengers", Count: 2}, {Name: "Savannah Lions", Count: 4},
		{Name: "Skyclave Cleric", Count: 2}, {Name: "Blackbloom Rogue", Count: 2}, {Name: "Swamp", Count: 7}, {Name: "Plains", Count: 7},
	},
}, {
	// Adventure and split cards. A face cast is gorge's option only
	// when the floating pool pays it (faces.go): an {X} spell's lowered
	// witness taps for the largest X, so a smaller X leaves mana floating.
	{
		{Name: "Blaze", Count: 2}, {Name: "Stream of Life", Count: 2}, {Name: "Rimrock Knight", Count: 2}, {Name: "Embereth Shieldbreaker", Count: 2},
		{Name: "Young Red Dragon", Count: 2}, {Name: "Bellowing Bruiser", Count: 1}, {Name: "Curious Pair", Count: 2},
		{Name: "Garenbrig Carver", Count: 2}, {Name: "Beanstalk Giant", Count: 2},
		{Name: "Fire", Count: 2}, {Name: "Collision", Count: 2}, {Name: "Dead", Count: 2},
		{Name: "Thrash", Count: 1}, {Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Blaze", Count: 4}, {Name: "Giant Killer", Count: 2}, {Name: "Murderous Rider", Count: 2},
		{Name: "Foulmire Knight", Count: 2}, {Name: "Order of Midnight", Count: 2}, {Name: "Ardenvale Tactician", Count: 2},
		{Name: "Faerie Guidemother", Count: 1}, {Name: "Shepherd of the Flock", Count: 1}, {Name: "Lonesome Unicorn", Count: 1},
		{Name: "Silverflame Squire", Count: 1}, {Name: "Smitten Swordmaster", Count: 1}, {Name: "Dead", Count: 2},
		{Name: "Carnival", Count: 1}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
}, {
	// land-front Adventures (the Final Fantasy towns, gorge
	// cases): the Adventure is cast from the hand while the land is
	// a land play; resolved, the land rests in exile and is played from
	// there (CR 715.3d). The {X} spells float the mana again.
	{
		{Name: "Blaze", Count: 4}, {Name: "Stream of Life", Count: 2}, {Name: "Lindblum, Industrial Regency", Count: 4},
		{Name: "Zanarkand, Ancient Metropolis", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Gray Ogre", Count: 2}, {Name: "Mountain", Count: 11}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Mind Spring", Count: 4}, {Name: "Jidoor, Aristocratic Capital", Count: 3}, {Name: "Ishgard, the Holy See", Count: 3},
		{Name: "Value Town", Count: 3}, {Name: "Coral Merfolk", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Hill Giant", Count: 2}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 4},
	},
}, {
	// modal DFCs with a nonland back (gorge, "modal_spell"):
	// Kolvori's The Ringhart Crest (an artifact that enters as the back
	// face) and Pestilent Cauldron's Restorative Burst (a sorcery that
	// exiles itself: the card rests in exile as its front, CR 712.8a).
	{
		{Name: "Stream of Life", Count: 4}, {Name: "Kolvori, God of Kinship", Count: 4}, {Name: "Pestilent Cauldron", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Midnight Scavengers", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Stream of Life", Count: 4}, {Name: "Kolvori, God of Kinship", Count: 3}, {Name: "Pestilent Cauldron", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Giant Spider", Count: 3},
		{Name: "Midnight Scavengers", Count: 2}, {Name: "Forest", Count: 9}, {Name: "Swamp", Count: 7},
	},
}}

// facesPolicy plays a modal DFC's back land and casts a face whenever
// offered, else random.
func facesPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand && a.Mode() == 1 || a.Kind() == ActCastFace {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

// faceCastPolicy casts a face whenever offered, else plays a land or casts
// a spell when it can (an {X} spell's smaller X floats the mana a face
// cast needs), else random.
func faceCastPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace {
				return a
			}
			if a.Kind() == ActPlayLand || a.Kind() == ActCast {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// facesCounts is the occurrence count of the face routes a slice of the
// faces pool exercised.
type facesCounts struct {
	backs          int
	landAdventures int
	landFromExile  int // a land-front Adventure's land played from exile (CR 715.3d)
	faceCasts      [16]int
	emptyOffers    [16]int // face-cast offers made while the actor's pool was empty
	emptyTaken     [16]int // of those offers, the ones cast
}

// facesCensus counts the lowered face offers made while the actor's
// pool is empty (the census path that the pool-only mutation must break).
// Source refs are game-local, so begin resets the per-game map.
type facesCensus struct {
	ref    *GorgeSide
	frames map[uint64]bool
	empty  map[uint8]map[Ref]bool
	offers [16]int
	taken  [16]int
}

func (c *facesCensus) begin() {
	c.ref = nil
	c.frames = map[uint64]bool{}
	c.empty = map[uint8]map[Ref]bool{}
	c.offers = [16]int{}
	c.taken = [16]int{}
}

func (c *facesCensus) check(ref *GorgeSide, k Side) *Divergence {
	c.ref = ref
	fr := ref.fr
	if fr == nil || fr.d == nil || fr.d.Kind != "priority" || c.frames[fr.seq] {
		return nil
	}
	c.frames[fr.seq] = true
	if ref.E.G.Players[state.PlayerID(fr.d.Player)].Pool.Total() != 0 {
		return nil
	}
	for _, code := range []uint8{faceCodeAdventure, faceCodeRecast, faceCodeSplit, faceCodeModal} {
		for _, r := range kernelFaceRefs(k, code) {
			if c.empty[code] == nil {
				c.empty[code] = map[Ref]bool{}
			}
			if !c.empty[code][r] {
				c.offers[code]++
			}
			c.empty[code][r] = true
		}
	}
	return nil
}

func (c *facesCensus) scan(acts []Canon) {
	for _, a := range acts {
		if a.Kind() != ActCastFace {
			continue
		}
		code := a.Mode() & 15
		if c.empty[code][a.Src()] {
			c.taken[code]++
			delete(c.empty[code], a.Src())
		}
	}
}

// runFacesLockstep runs the given matchups at their established seed counts
// (32 seeds for matchups 0-1, 512 for 2-4) and returns their counts.
func runFacesLockstep(t *testing.T, mis []int) facesCounts {
	t.Helper()
	reg, set := facesPool(t)
	// each matchup's seeds as parallel chunks; the per-matchup lines log
	// after the whole run
	var cs []seedChunk
	for _, mi := range mis {
		n := facesSeeds(mi)
		for lo := uint64(1); lo <= n; lo += facesChunk {
			cs = append(cs, seedChunk{mi: mi, lo: lo, hi: min(lo+facesChunk-1, n)})
		}
	}
	per := map[int]*facesCounts{}
	for _, mi := range mis {
		per[mi] = &facesCounts{}
	}
	runChunks(t, cs, true, func(t *testing.T, c seedChunk, mu *sync.Mutex) {
		var part facesCounts
		facesPlay(t, reg, set, c.mi, c.lo, c.hi, false, &part)
		mu.Lock()
		defer mu.Unlock()
		per[c.mi].add(&part)
	})
	var got facesCounts
	for _, mi := range mis {
		p := per[mi]
		got.add(p)
		t.Logf("matchup %d: %d back-face land plays, %d land-front Adventure casts, face casts %v", mi, p.backs, p.landAdventures, p.faceCasts)
	}
	return got
}

// facesChunk is the seeds one parallel subtest of runFacesLockstep plays.
const facesChunk = 128

// facesSeeds is matchup mi's established seed count.
func facesSeeds(mi int) uint64 {
	if mi >= 2 {
		return 512 // a face cast needs mana left floating: rarer
	}
	return 32
}

// add sums o's counts into c.
func (c *facesCounts) add(o *facesCounts) {
	c.backs += o.backs
	c.landAdventures += o.landAdventures
	c.landFromExile += o.landFromExile
	for i := range c.faceCasts {
		c.faceCasts[i] += o.faceCasts[i]
		c.emptyOffers[i] += o.emptyOffers[i]
		c.emptyTaken[i] += o.emptyTaken[i]
	}
}

// facesPool loads the corpus and the generated faces set.
func facesPool(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("faces")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// runFacesSeeds is runFacesLockstep in one loop; with mutation set a
// divergence is not an error: the run stops at the first one and reports it.
func runFacesSeeds(t *testing.T, mis []int, mutation bool) (facesCounts, bool) {
	t.Helper()
	reg, set := facesPool(t)
	var got facesCounts
	for _, mi := range mis {
		before := got
		if facesPlay(t, reg, set, mi, 1, facesSeeds(mi), mutation, &got) {
			return got, true
		}
		t.Logf("matchup %d: %d back-face land plays, %d land-front Adventure casts, face casts %v", mi,
			got.backs-before.backs, got.landAdventures-before.landAdventures, faceCastDelta(got.faceCasts, before.faceCasts))
	}
	return got, false
}

// facesPlay plays seeds lo..hi of matchup mi into got; with mutation it
// stops at the first divergence and reports it.
func facesPlay(t *testing.T, reg *cards.Registry, set cardset.Set, mi int, lo, hi uint64, mutation bool, got *facesCounts) bool {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: true}
	var cen facesCensus
	md := facesDecks[mi]
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := lo; seed <= hi; seed++ {
		m, err := NewMatchup(reg, [2]string{"faces-a", "faces-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := cfg
		if mi >= 2 && seed <= 480 {
			// gorge's lowering surface: the autopay surface has no
			// floating mana to pay a face cast with (it drops every
			// plain cast its planner does not price: gorge.go); the
			// last seeds play the autopay surface
			cfg = Config{}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := facesPolicy(seed)
		if mi >= 2 {
			pol = faceCastPolicy(seed)
		}
		if seed%4 == 0 {
			pol = RandomPolicy(seed) // the back land only as often as chance picks it
		}
		cen.begin()
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: cen.check})
		cen.scan(res.Actions)
		for code := range cen.offers {
			got.emptyOffers[code] += cen.offers[code]
			got.emptyTaken[code] += cen.taken[code]
		}
		CloseSide(k)
		// onAdventure tracks a card cast as its Adventure: after that
		// cast resolves it rests in exile, so a later play_land of the
		// same object is the CR 715.3d play from exile (a recast or a
		// land play takes it off the adventure).
		var onAdventure [MaxInst]bool
		for _, a := range res.Actions {
			switch a.Kind() {
			case ActCastFace:
				switch a.Mode() & 15 {
				case faceCodeAdventure:
					onAdventure[a.Src()] = true
				case faceCodeRecast:
					onAdventure[a.Src()] = false
				}
			case ActPlayLand:
				if onAdventure[a.Src()] {
					got.landFromExile++
					onAdventure[a.Src()] = false
				}
			}
			if a.Kind() == ActPlayLand && a.Mode() == 1 {
				got.backs++
			}
			if a.Kind() == ActCastFace {
				got.faceCasts[a.Mode()&15]++
				if mi == 3 && a.Mode()&15 == faceCodeAdventure {
					got.landAdventures++ // every Adventure card of matchup 3 has a land front
				}
				if mi == 3 && a.Mode()&15 == faceCodeRecast {
					t.Errorf("matchup %d seed %d: a land main face recast from exile", mi, seed)
				}
			}
		}
		if d := res.Div; d != nil && mutation {
			t.Logf("matchup %d seed %d diverged at decision %d: %s %s", mi, seed, d.At, d.What, d.Field)
			return true
		} else if d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return false
}

// faceCastDelta is the per-code difference of two face cast tallies.
func faceCastDelta(a, b [16]int) [16]int {
	var d [16]int
	for i := range d {
		d[i] = a[i] - b[i]
	}
	return d
}

// checkFaces asserts a slice's counts against the minimums measured for it.
func checkFaces(t *testing.T, got, want facesCounts) {
	t.Helper()
	t.Logf("%d back-face land plays; face casts %v (modal_spell %d, adventure_alt %d, adventure_recast %d, split_alt %d)", got.backs, got.faceCasts,
		got.faceCasts[faceCodeModal], got.faceCasts[faceCodeAdventure], got.faceCasts[faceCodeRecast], got.faceCasts[faceCodeSplit])
	t.Logf("%d land-front Adventure casts, %d land plays from exile", got.landAdventures, got.landFromExile)
	t.Logf("empty-pool face offers taken: adventure %d/%d, recast %d/%d, split %d/%d, modal %d/%d",
		got.emptyTaken[faceCodeAdventure], got.emptyOffers[faceCodeAdventure],
		got.emptyTaken[faceCodeRecast], got.emptyOffers[faceCodeRecast],
		got.emptyTaken[faceCodeSplit], got.emptyOffers[faceCodeSplit],
		got.emptyTaken[faceCodeModal], got.emptyOffers[faceCodeModal])
	if got.backs < want.backs {
		t.Errorf("%d back-face land plays, want at least %d", got.backs, want.backs)
	}
	if got.landAdventures < want.landAdventures {
		t.Errorf("%d land-front Adventure casts, want at least %d", got.landAdventures, want.landAdventures)
	}
	if got.landFromExile < want.landFromExile {
		t.Errorf("%d land plays from exile, want at least %d", got.landFromExile, want.landFromExile)
	}
	for code := range want.faceCasts {
		if got.faceCasts[code] < want.faceCasts[code] {
			t.Errorf("face route %d cast %d times, want at least %d", code, got.faceCasts[code], want.faceCasts[code])
		}
	}
	for code := range want.emptyTaken {
		if got.emptyTaken[code] < want.emptyTaken[code] {
			t.Errorf("face route %d empty-pool offers taken %d (offers %d), want at least %d",
				code, got.emptyTaken[code], got.emptyOffers[code], want.emptyTaken[code])
		}
	}
}

// TestFacesLockstep runs matchups 0-1: the modal DFC lands and pathways.
// The minimums are measured for this slice; the old whole-pool minimums are
// still met by the sum across the three tests (987 back-face land plays;
// face routes 364 adventure, 23 recast, 12 split, 51 modal).
func TestFacesLockstep(t *testing.T) {
	t.Parallel()

	got := runFacesLockstep(t, []int{0, 1})
	checkFaces(t, got, facesCounts{backs: 987})
}

// TestFacesLockstepB runs matchups 2-3: Adventure and split cards, then the
// land-front Adventures.
func TestFacesLockstepB(t *testing.T) {
	t.Parallel()

	got := runFacesLockstep(t, []int{2, 3})
	checkFaces(t, got, facesCounts{
		// the exiled land is now a legal play, so one seed's
		// trajectory casts one fewer Adventure from the hand (194/363).
		landAdventures: 194,
		landFromExile:  10,
		faceCasts: [16]int{
			faceCodeAdventure: 363,
			faceCodeRecast:    23,
			faceCodeSplit:     12,
		},
		// the census offers these while the pool is empty and the
		// lockstep takes them (the DebugFacePoolOnly mutation breaks it).
		emptyTaken: [16]int{
			faceCodeAdventure: 8000,
			faceCodeRecast:    4500,
			faceCodeSplit:     1000,
		},
	})
}

// TestFacesLockstepC runs matchup 4: modal DFCs with a nonland back.
func TestFacesLockstepC(t *testing.T) {
	t.Parallel()

	got := runFacesLockstep(t, []int{4})
	checkFaces(t, got, facesCounts{
		faceCasts: [16]int{faceCodeModal: 51},
		// 2800: measured (2890 of 3243 offers). Since gorge
		// 99519fd0a the potential planner checks a restricted source against the
		// face being cast, so a Crest no longer joins a back face's witness and
		// strands it; the kernel's faceRestrDeadEnd mirror is gone
		emptyTaken: [16]int{faceCodeModal: 2800},
	})
}

// TestFacesPoolOnlyMutation: the pool-only face offer (DebugFacePoolOnly)
// must diverge from gorge, which prices every face route from untapped
// sources: the census is load-bearing for the Adventure, split
// (matchup 2) and modal (matchup 4) routes.
func TestFacesPoolOnlyMutation(t *testing.T) {
	dirac.DebugFacePoolOnly = true
	defer func() { dirac.DebugFacePoolOnly = false }()
	for _, mi := range []int{2, 4} {
		if _, div := runFacesSeeds(t, []int{mi}, true); !div {
			t.Errorf("matchup %d: the pool-only face offer never diverged", mi)
		}
	}
}
