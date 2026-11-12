package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The departure-read lockstep: Revolt$ True on the TMT
// Disappear triggers (gorge revoltThisTurn: a permanent the controller
// controlled left the battlefield this turn, the controller at the moment it
// left) and Count$LeftGraveyardThisTurn (gorge evalCov3HeadLeftZoneThisTurn:
// distinct cards that left a graveyard this turn). Both read the kernel's
// per-turn zone-add ring. Deck A holds the Revolt carriers with the
// permanent leavers and a steal (Control Magic: the controller it left under is not the owner); deck B holds the graveyard-leave carriers
// with the recursion (Raise Dead, Gravedigger) and burn.
var kleftzoneDecks = [][2][]cardset.Entry{{
	{
		{Name: "Foot Mystic", Count: 3}, {Name: "Insectoid Exterminator", Count: 3}, {Name: "Lord Dregg, Insect Invader", Count: 2},
		{Name: "Michelangelo, Game Master", Count: 2}, {Name: "Pizza Face, Gastromancer", Count: 2}, {Name: "West Wind Avatar", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Unsummon", Count: 2}, {Name: "Bake into a Pie", Count: 1}, {Name: "Hungry Ghoul", Count: 2},
		{Name: "Control Magic", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Unsummon", Count: 2}, {Name: "Control Magic", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 4},
	},
}, {
	// the recursion mirror: a creature card can leave a graveyard twice in
	// one turn (Shock it, Raise Dead it, cast it, Shock it, Raise Dead it)
	{
		{Name: "Living History", Count: 2}, {Name: "Primary Research", Count: 2}, {Name: "Wilt in the Heat", Count: 2},
		{Name: "Raise Dead", Count: 6}, {Name: "Gravedigger", Count: 2}, {Name: "Shock", Count: 6}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Living History", Count: 2}, {Name: "Primary Research", Count: 2}, {Name: "Wilt in the Heat", Count: 2},
		{Name: "Raise Dead", Count: 6}, {Name: "Gravedigger", Count: 2}, {Name: "Shock", Count: 6}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
}, {
	{
		{Name: "Living History", Count: 3}, {Name: "Primary Research", Count: 3}, {Name: "Wilt in the Heat", Count: 3},
		{Name: "Raise Dead", Count: 5}, {Name: "Gravedigger", Count: 5},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Gravedigger", Count: 3}, {Name: "Raise Dead", Count: 3},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 10},
	},
}}

// kleftzoneEndCarriers are the Disappear end-step triggers (Lord Dregg,
// Insectoid Exterminator, Michelangelo, Pizza Face, West Wind Avatar).
var kleftzoneEndCarriers = map[string]bool{"Lord Dregg, Insect Invader": true, "Insectoid Exterminator": true,
	"Michelangelo, Game Master": true, "Pizza Face, Gastromancer": true, "West Wind Avatar": true}

// kleftzoneTally classifies gorge's Revolt and LeftGraveyardThisTurn from its
// own log and zone-entry list, once per (game, turn, class).
type kleftzoneTally struct {
	game       int
	seen       map[[3]int32]bool
	revolt     int // end steps of a player with a Disappear carrier, Revolt true
	noRevolt   int // ... Revolt false
	stolen     int // turns where a permanent left the battlefield after being stolen (controlled by a non-owner)
	thief      map[state.ObjID]bool
	thiefGame  int
	leftGrave  int // turns with at least one card having left a graveyard
	leftTwice  int // turns where one card left a graveyard twice
	cardsLeft  int
	stalls     int
	entFull    int
	gamesStall map[int]bool
}

func (w *kleftzoneTally) once(turn, class int32) bool {
	key := [3]int32{int32(w.game), turn, class}
	if w.seen[key] {
		return false
	}
	w.seen[key] = true
	return true
}

func (w *kleftzoneTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	// Revolt for the active player, from the log (as gorge's revoltThisTurn)
	revolt, stolen := false, false
	for i := len(evs) - 1; i >= 0; i-- {
		ev := evs[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield {
			if o := g.Obj(ev.Obj); o != nil {
				if o.Controller == g.Active {
					revolt = true
				}
				if o.Controller != o.Owner {
					stolen = true
				}
			}
		}
	}
	// a permanent that left while stolen (Control Magic): remembered while it
	// sits on the battlefield under a non-owner
	if w.thief == nil || w.thiefGame != w.game {
		w.thief, w.thiefGame = map[state.ObjID]bool{}, w.game
	}
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Controller != o.Owner {
				w.thief[id] = true
			}
		}
	}
	for _, en := range g.Entered {
		if en.From == state.ZBattlefield && w.thief[en.Obj] && w.once(g.Turn, 10) {
			w.stolen++
		}
	}
	_ = stolen
	if g.Step == state.StepEnd {
		carrier := false
		for _, id := range g.Zone(state.ZBattlefield, g.Active) {
			if o := g.Obj(id); o != nil && o.Face() != nil && kleftzoneEndCarriers[o.Face().Name] {
				carrier = true
			}
		}
		if carrier {
			if revolt && w.once(g.Turn, 1) {
				w.revolt++
			} else if !revolt && w.once(g.Turn, 2) {
				w.noRevolt++
			}
		}
	}
	// distinct cards that left a graveyard this turn
	n, entries := 0, 0
	var ids []state.ObjID
	for _, en := range g.Entered {
		if en.From != state.ZGraveyard {
			continue
		}
		entries++
		dup := false
		for _, id := range ids {
			if id == en.Obj {
				dup = true
			}
		}
		if !dup {
			ids = append(ids, en.Obj)
			n++
		}
	}
	if n > 0 && w.once(g.Turn, 20+int32(n)) {
		w.leftGrave++
		w.cardsLeft += n
	}
	if entries > n && w.once(g.Turn, 30) {
		w.leftTwice++
	}
	return nil
}

// runLeftzone plays the constructed matchups. The first return is the
// tally, then the games, the divergences and the games that stalled on the
// zone-add ring (StallEntered) or any other kernel capacity.
func runLeftzone(t *testing.T, seeds int) (w *kleftzoneTally, games, divs, entStalls, otherStalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("leftzone")
	if err != nil {
		t.Fatal(err)
	}
	w = &kleftzoneTally{seen: map[[3]int32]bool{}}
	for mi, md := range kleftzoneDecks {
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
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kleftzone-a", "kleftzone-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castActivatePolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			w.game = games
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: w.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				divs++
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Logf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				if strings.Contains(res.Stall, "entered") || strings.Contains(res.Stall, "zone-add") {
					entStalls++
				} else {
					otherStalls++
					t.Logf("matchup %d seed %d: kernel capacity stall %q", mi, seed, res.Stall)
				}
			}
		}
	}
	return w, games, divs, entStalls, otherStalls
}

func TestLeftzoneLockstep(t *testing.T) {
	t.Parallel()
	w, games, divs, entStalls, otherStalls := runLeftzone(t, 256)
	t.Logf("%d games, %d divergences, %d StallEntered, %d other stalls; Revolt end steps: %d true, %d false; %d turns with a stolen permanent leaving; %d turns with cards leaving a graveyard (%d cards), %d turns where one card left twice",
		games, divs, entStalls, otherStalls, w.revolt, w.noRevolt, w.stolen, w.leftGrave, w.cardsLeft, w.leftTwice)
	if divs != 0 {
		t.Errorf("%d divergences", divs)
	}
	if otherStalls != 0 {
		t.Errorf("%d kernel capacity stalls other than the zone-add ring", otherStalls)
	}
	if entStalls*50 > games {
		t.Errorf("%d of %d games stalled on the zone-add ring (> 2%%): the ring is too small for this pool", entStalls, games)
	}
	if w.revolt < 30 {
		t.Errorf("%d Revolt end steps true, want >= 30", w.revolt)
	}
	if w.noRevolt < 30 {
		t.Errorf("%d Revolt end steps false, want >= 30", w.noRevolt)
	}
	if w.leftGrave < 20 {
		t.Errorf("%d turns with a nonzero LeftGraveyardThisTurn, want >= 20", w.leftGrave)
	}
}

// TestLeftzoneMutation flips each reader in turn: the Revolt controller to
// the entry's owner, the Revolt player filter off, and the graveyard-leave
// count to one per entry instead of one per object. The player filter must
// diverge from gorge here. The other two cannot on this pool: no FDN card
// returns another player's departed permanent to the battlefield (the owner
// read differs from the controller read only there) and no game has a card
// leave a graveyard twice in one turn; kernel TestLeftzoneRing pins both
// against a hand-built ring.
func TestLeftzoneMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mu := range []struct {
		name   string
		flag   *bool
		mustDv bool
	}{{"DebugRevoltAnyone", &dirac.DebugRevoltAnyone, true}, {"DebugRevoltOwner", &dirac.DebugRevoltOwner, false},
		{"DebugLeftZoneNoDedupe", &dirac.DebugLeftZoneNoDedupe, false}} {
		t.Run(mu.name, func(t *testing.T) {
			old := *mu.flag
			*mu.flag = true
			defer func() { *mu.flag = old }()
			_, games, divs, _, _ := runLeftzone(t, 256)
			if divs == 0 && mu.mustDv {
				t.Fatalf("%s: no divergence in %d games", mu.name, games)
			}
			t.Logf("%s diverged in %d of %d games", mu.name, divs, games)
		})
	}
}
