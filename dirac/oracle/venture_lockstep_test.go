package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ventureCounts counts the dungeon events of a finished gorge log.
type ventureCounts struct {
	enters    int            // DungeonCreate: a dungeon entered the command zone
	rooms     int            // DungeonRoom: the marker moved
	completes int            // DungeonComplete: a dungeon was completed
	byKey     map[string]int // DungeonCreate by dungeon token script
	exiled    map[string]int // lands that moved from exile to the battlefield, by name (a Runestone Caverns pick)
}

func countVentureEvents(g *GorgeSide) (c ventureCounts) {
	c.byKey = map[string]int{}
	c.exiled = map[string]int{}
	for _, e := range g.E.L.Events {
		if e.Kind == events.MoveZone && e.From == state.ZExile && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().IsLand() {
				c.exiled[o.Face().Name]++
			}
		}
		switch e.Kind {
		case events.DungeonCreate:
			c.enters++
			c.byKey[e.Text]++
		case events.DungeonRoom:
			c.rooms++
		case events.DungeonComplete:
			c.completes++
		}
	}
	return c
}

// venturePodEntries is the venture fixture's four decks: the carriers that
// venture (on entering, attacking, damage, upkeep, activation) beside cheap
// creatures and burn, so each seat ventures repeatedly, picks among the three
// dungeons and walks their rooms (Lost Mine of Phandelver, Dungeon of the Mad
// Mage, Tomb of Annihilation).
func venturePodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Ranger's Hawk", Count: 2}, {Name: "Wandering Troubadour", Count: 2}, {Name: "Clattering Skeletons", Count: 2},
			{Name: "Zombie Ogre", Count: 2}, {Name: "Bar the Gate", Count: 2}, {Name: "Find the Path", Count: 2}, {Name: "Veteran Dungeoneer", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Lightning Bolt", Count: 2}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Triumphant Adventurer", Count: 2}, {Name: "Shortcut Seeker", Count: 2}, {Name: "Precipitous Drop", Count: 2},
			{Name: "Planar Ally", Count: 2}, {Name: "Eccentric Apprentice", Count: 2}, {Name: "Cloister Gargoyle", Count: 2}, {Name: "Secret Door", Count: 2},
			{Name: "Savannah Lions", Count: 4}, {Name: "Giant Growth", Count: 2}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
		},
		{
			{Name: "Intrepid Outlander", Count: 2}, {Name: "Midnight Pathlighter", Count: 2}, {Name: "Barrowin of Clan Undurr", Count: 2},
			{Name: "Fates' Reversal", Count: 2}, {Name: "Thorough Investigation", Count: 2}, {Name: "Displacer Beast", Count: 2}, {Name: "Radiant Solar", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Lightning Bolt", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
		},
		{
			{Name: "Dungeon Descent", Count: 2}, {Name: "Delver's Torch", Count: 2}, {Name: "Dungeon Map", Count: 2}, {Name: "Zalto, Fire Giant Duke", Count: 2},
			{Name: "Nadaar, Selfless Paladin", Count: 2}, {Name: "You Find a Cursed Idol", Count: 2}, {Name: "Wandering Troubadour", Count: 2},
			{Name: "Savannah Lions", Count: 4}, {Name: "Giant Growth", Count: 2}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
		},
	}
}

type venturePod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newVenturePod(t *testing.T) venturePod {
	t.Helper()
	return newVenturePodOf(t, venturePodEntries())
}

// newVenturePodOf is newVenturePod over the given four decks.
func newVenturePodOf(t *testing.T, decks [4][]cardset.Entry) venturePod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("venture")
	if err != nil {
		t.Fatal(err)
	}
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return venturePod{reg: reg, set: set, tbl: tbl, decks: decks}
}

// play plays one seed of the four-seat pod and returns the dungeon events and
// the kernel's ask counters. The lockstep compares the two sides' option lists
// at every decision, so the dungeon candidates' key order and each room
// choice's printed arrow order are checked decision by decision.
func (p venturePod) play(t *testing.T, seed uint64, maxTurns int32) (Result, ventureCounts, dirac.VentureStats) {
	t.Helper()
	decks := p.decks
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])},
		MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{expand(decks[2]), expand(decks[3])}, Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	st := &dirac.VentureStats{}
	dirac.DebugVentureStats = st
	defer func() { dirac.DebugVentureStats = nil }()
	var c ventureCounts
	rc.Finish = func(g *GorgeSide) { c = countVentureEvents(g) }
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
	return res, c, *st
}

// ventureFail reports a seed's divergence with the action window that reached it.
func ventureFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// TestFourSeatVentureLockstep plays four-seat pods of the venture fixture:
// the carriers venture, a seat with no dungeon picks one of the three (Lost
// Mine, the Mad Mage, Tomb of Annihilation, in token-key order), a two-arrow
// room asks which room to enter, and a completed dungeon begins a new one.
// No divergence and no stall.
func TestFourSeatVentureLockstep(t *testing.T) {
	p := newVenturePod(t)
	var sum ventureCounts
	sum.byKey = map[string]int{}
	var asks dirac.VentureStats
	for _, seed := range ventureSeeds {
		res, c, st := p.play(t, seed, 40)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.enters += c.enters
		sum.rooms += c.rooms
		sum.completes += c.completes
		for k, n := range c.byKey {
			sum.byKey[k] += n
		}
		asks.DungeonAsks += st.DungeonAsks
		asks.RoomAsks += st.RoomAsks
		t.Logf("seed %d: %d decisions, over %v %s; %d dungeon(s) %v, %d room(s), %d completed, %d dungeon ask(s), %d room ask(s)", seed, res.Decisions, res.Over, res.Stall,
			c.enters, c.byKey, c.rooms, c.completes, st.DungeonAsks, st.RoomAsks)
	}
	if sum.enters == 0 {
		t.Fatal("no game ventured")
	}
	if asks.DungeonAsks == 0 || asks.RoomAsks == 0 {
		t.Fatalf("dungeon asks %d, room asks %d: want both above zero", asks.DungeonAsks, asks.RoomAsks)
	}
	if len(sum.byKey) < 3 {
		t.Fatalf("dungeons entered %v: want all three", sum.byKey)
	}
	if sum.completes == 0 {
		t.Fatal("no dungeon was completed")
	}
}

// TestVentureDungeonMutation is the mutation test: leaving the last
// candidate out of the dungeon ask changes the option list gorge poses at the
// first dungeon choice, so it must diverge there.
func TestVentureDungeonMutation(t *testing.T) {
	p := newVenturePod(t)
	dirac.DebugVentureNoLast = true
	defer func() { dirac.DebugVentureNoLast = false }()
	res, _, _ := p.play(t, ventureSeeds[0], 40)
	if res.Div == nil {
		t.Fatal("dropping a dungeon candidate diverged on no decision")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}

// TestVentureRunestoneLandMutation is the second mutation test: a land
// Runestone Caverns' Play picked is the only way that Play adds a land to the
// battlefield, so leaving it unplayed must diverge.
func TestVentureRunestoneLandMutation(t *testing.T) {
	p := newVenturePod(t)
	dirac.DebugVentureNoLand = true
	defer func() { dirac.DebugVentureNoLand = false }()
	res, _, _ := p.play(t, 2, 40)
	if res.Div == nil {
		t.Fatal("leaving a picked land unplayed diverged on no decision")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}

// ventureSeeds are the seeds the lockstep plays.
// Seed 2 plays a land Runestone Caverns offered; seed 5 completes a dungeon
// under Precipitous Drop (the state-based pass runs again on the completed count).
var ventureSeeds = []uint64{1, 2, 3, 5}

// TestVenturePinnedRepro pins seed 7 and the dungeon events it reaches: seven
// dungeons entered (three Mad Mage, one Lost Mine, three Tomb of Annihilation),
// twenty-eight marker moves and three completed dungeons, with seven dungeon
// asks and ten room asks the option lists are checked through. Moved from
// 29/5 by Oubliette's SacEachValid$ sacrifice (agent-20261119T194717Z): the
// room now takes each player's artifact, creature and land, so the pods'
// boards thin out and three walks end before their last room.
func TestVenturePinnedRepro(t *testing.T) {
	p := newVenturePod(t)
	res, c, st := p.play(t, 7, 40)
	if res.Div != nil {
		ventureFail(t, 7, res)
	}
	want := ventureCounts{enters: 7, rooms: 28, completes: 3,
		byKey: map[string]int{"dungeon_of_the_mad_mage": 3, "lost_mine_of_phandelver": 1, "tomb_of_annihilation": 3}}
	if c.enters != want.enters || c.rooms != want.rooms || c.completes != want.completes || len(c.byKey) != len(want.byKey) {
		t.Fatalf("seed 7 dungeon events = %+v, want %+v", c, want)
	}
	for k, n := range want.byKey {
		if c.byKey[k] != n {
			t.Fatalf("seed 7 dungeon %s entered %d times, want %d", k, c.byKey[k], n)
		}
	}
	if st.DungeonAsks != 7 || st.RoomAsks != 10 {
		t.Fatalf("seed 7 dungeon/room asks = %d/%d, want 7/10", st.DungeonAsks, st.RoomAsks)
	}
	t.Logf("seed 7: %d decisions, dungeon events %+v, asks %d/%d", res.Decisions, c, st.DungeonAsks, st.RoomAsks)
}
