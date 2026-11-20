package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The landwalk-grant lockstep (fixture landwalkgrants): an Aura's landwalk
// grant, a PumpAll's, a level band's, the IgnoreLandWalk static (Crevasse,
// Great Wall, Staff of the Ages, Street Savvy) and granted skulk (Behind the
// Scenes, MacCready, Skeleton Key, The Master, Mesmerist) beside creatures with
// printed landwalk and a spread of powers. The kernel's LandwalkStats prove each
// half decided a block, so a green game is not one where nothing happened.

// landwalkPodDecks are the pod's four decks: blue-black (swampwalk and
// islandwalk Auras, Undertow and Quagmire, The Master), red-green (mountain,
// forest and island walkers and their Auras, Deadfall, Crevasse, Nature's
// Cloak), white with a black splash (MacCready, Behind the Scenes, Great Wall,
// Staff of the Ages) and green-white (Lord Magnus, Street Savvy).
func landwalkPodDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Lost Soul", Count: 3}, {Name: "Bog Wraith", Count: 2}, {Name: "Leshrac's Rite", Count: 2}, {Name: "Fishliver Oil", Count: 2},
			{Name: "Undertow", Count: 2}, {Name: "Quagmire", Count: 2}, {Name: "Skeleton Key", Count: 1}, {Name: "The Master, Mesmerist", Count: 1},
			{Name: "Ur-Drago", Count: 1}, {Name: "Halimar Wavewatch", Count: 2}, {Name: "Typhoid Rats", Count: 2}, {Name: "Wind Drake", Count: 2},
			{Name: "Coral Merfolk", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
		},
		{
			{Name: "Mountain Goat", Count: 3}, {Name: "Mountain Yeti", Count: 2}, {Name: "River Boa", Count: 2}, {Name: "Gorilla Chieftain", Count: 2},
			{Name: "Burrowing", Count: 2}, {Name: "Cave Sense", Count: 2}, {Name: "Volcanic Strength", Count: 2}, {Name: "Dryad's Favor", Count: 2},
			{Name: "Deadfall", Count: 2}, {Name: "Crevasse", Count: 2}, {Name: "Nature's Cloak", Count: 1}, {Name: "Hill Giant", Count: 2},
			{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "MacCready, Lamplight Mayor", Count: 2}, {Name: "Behind the Scenes", Count: 2}, {Name: "Great Wall", Count: 2}, {Name: "Savannah Lions", Count: 3},
			{Name: "Glory Seeker", Count: 3}, {Name: "Lost Soul", Count: 2}, {Name: "Typhoid Rats", Count: 2}, {Name: "Staff of the Ages", Count: 2},
			{Name: "Skeleton Key", Count: 1}, {Name: "Gosta Dirk", Count: 1}, {Name: "Bog Wraith", Count: 2}, {Name: "White Knight", Count: 2},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 3},
		},
		{
			{Name: "Lord Magnus", Count: 2}, {Name: "Street Savvy", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Gorilla Chieftain", Count: 3},
			{Name: "Savannah Lions", Count: 3}, {Name: "River Boa", Count: 2}, {Name: "Skeleton Key", Count: 1}, {Name: "Staff of the Ages", Count: 1},
			{Name: "Nature's Cloak", Count: 1}, {Name: "Glory Seeker", Count: 2}, {Name: "Dryad's Favor", Count: 2},
			{Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
		},
	}
}

// playLandwalk plays one seed of the landwalkgrants fixture at seats (2 or 4)
// over the pod's decks and returns the result and the kernel's evasion counts.
func playLandwalk(t *testing.T, seats int, seed uint64, maxTurns int32) (Result, dirac.LandwalkStats) {
	t.Helper()
	all := landwalkPodDecks()
	return playLandwalkDecks(t, all[:seats], seed, maxTurns, nil)
}

// playLandwalkDecks is playLandwalk over the given decks (finish, when set,
// reads the finished gorge game).
func playLandwalkDecks(t *testing.T, decks [][]cardset.Entry, seed uint64, maxTurns int32, finish func(*GorgeSide)) (Result, dirac.LandwalkStats) {
	t.Helper()
	seats := len(decks)
	reg := synthReg(t)
	set, err := cardset.RawByName("landwalkgrants")
	if err != nil {
		t.Fatal(err)
	}
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	for i, d := range decks {
		if n := len(expand(d)); n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
	}
	tbl, _, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])}, Seed: seed, PassElision: true, Surface: "full"}
	if seats == 4 {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expand(decks[2]), expand(decks[3])}
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	rc.Finish = finish
	st := &dirac.LandwalkStats{}
	dirac.DebugLandwalkStats = st
	defer func() { dirac.DebugLandwalkStats = nil }()
	k, err := SetSide(set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc), *st
}

// landwalkSeeds are the seeds the lockstep plays at each seat count.
var landwalkSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

func landwalkLockstep(t *testing.T, seats int) {
	t.Helper()
	var sum dirac.LandwalkStats
	for _, seed := range landwalkSeeds {
		res, st := playLandwalk(t, seats, seed, 40)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Stall, res.Over, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.GrantedEvades += st.GrantedEvades
		sum.AttachedEvades += st.AttachedEvades
		sum.IgnoredBlocks += st.IgnoredBlocks
		sum.GrantedSkulk += st.GrantedSkulk
		t.Logf("seed %d: %d decisions, over %v %s; %+v", seed, res.Decisions, res.Over, res.Stall, st)
	}
	if sum.GrantedEvades == 0 {
		t.Error("no game made an attacker unblockable through a granted landwalk")
	}
	if sum.AttachedEvades == 0 {
		t.Error("no game made an attacker unblockable through an Aura's or an Equipment's granted landwalk")
	}
	if sum.IgnoredBlocks == 0 {
		t.Error("no game let a blocker block through an IgnoreLandWalk static")
	}
	if sum.GrantedSkulk == 0 {
		t.Error("no game restricted a block through a granted skulk")
	}
}

// TestLandwalkGrantsFourSeatLockstep plays four-seat pods of the fixture over
// the seeds: no divergence, no stall, and across them a granted landwalk made
// an attack unblockable, an IgnoreLandWalk static allowed a block, and a
// granted skulk restricted one.
func TestLandwalkGrantsFourSeatLockstep(t *testing.T) { landwalkLockstep(t, 4) }

// TestLandwalkGrantsTwoSeatLockstep is the same over two-seat games.
func TestLandwalkGrantsTwoSeatLockstep(t *testing.T) { landwalkLockstep(t, 2) }

// TestIgnoreLandWalkMutation: with every IgnoreLandWalk static switched off
// the kernel forbids a block gorge allows, so a seed where one decided a
// block diverges.
func TestIgnoreLandWalkMutation(t *testing.T) {
	seed := ignoreLandWalkSeed
	if _, st := playLandwalk(t, 4, seed, 40); st.IgnoredBlocks == 0 {
		t.Fatalf("seed %d: no block was allowed by an IgnoreLandWalk static", seed)
	}
	dirac.DebugIgnoreLandWalkOff = true
	defer func() { dirac.DebugIgnoreLandWalkOff = false }()
	res, _ := playLandwalk(t, 4, seed, 40)
	if res.Div == nil {
		t.Fatal("switching IgnoreLandWalk off diverged on no decision")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}

// ignoreLandWalkSeed is a four-seat seed where an IgnoreLandWalk static lets a
// blocker block an attacker whose landwalk line has a land to hit.
const ignoreLandWalkSeed uint64 = 1

// masterDecks are two decks built around The Master, Mesmerist: its tap
// ability gives an opponent's creature skulk and its trigger (the withSkulk
// filter word) reads any skulk creature's combat damage to an opponent; the
// second deck runs creatures of a spread of powers to block with.
func masterDecks() [][]cardset.Entry {
	return [][]cardset.Entry{
		{
			{Name: "The Master, Mesmerist", Count: 4}, {Name: "Skeleton Key", Count: 4}, {Name: "MacCready, Lamplight Mayor", Count: 2},
			{Name: "Behind the Scenes", Count: 2}, {Name: "Typhoid Rats", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Glory Seeker", Count: 4},
			{Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 4},
		},
		{
			{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Gorilla Chieftain", Count: 4}, {Name: "Mountain Goat", Count: 4},
			{Name: "Volcanic Strength", Count: 4}, {Name: "Burrowing", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
		},
	}
}

// TestWithSkulkLockstep: the withSkulk word (The Master, Mesmerist's trigger)
// reads granted skulk (Skeleton Key, Behind the Scenes, MacCready, The Master's
// own ability) in lockstep with gorge. Some seed puts a +1/+1 counter on The
// Master, which only that trigger does, so the word matched.
func TestWithSkulkLockstep(t *testing.T) {
	counted := 0
	for seed := uint64(1); seed <= 40; seed++ {
		grew := false
		res, _ := playLandwalkDecks(t, masterDecks(), seed, 30, func(g *GorgeSide) {
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
					if o := g.E.G.Obj(id); o.Face().Name == "The Master, Mesmerist" && o.Counter("P1P1") > 0 {
						grew = true
					}
				}
			}
		})
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if grew {
			counted++
		}
	}
	if counted == 0 {
		t.Fatal("no seed put a counter on The Master through its withSkulk trigger")
	}
	t.Logf("%d of 40 seeds grew The Master through a skulk creature's combat damage", counted)
}
