package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Mode$ Drawn trigger whose ValidCard$ scopes the draw by the DRAWN CARD'S
// OWNER is a player scope on the drawing player (a card is drawn from its
// owner's library into its owner's hand, CR 108.4a; gorge drawnMatches reads
// ValidCard$ against the drawn object). Tataru Taru fires for an opponent's
// draw that is not that player's own turn, once each turn; the draw Curses
// (Curse of Fool's Wisdom, Psychic Possession) fire for the player their own
// Aura enchants.

// kdrawnOwnerA runs Tataru Taru beside instant-speed draws (the opponent's
// off-turn draws come from seat B's Opt / Vampiric Touch / Into the Story
// casts during A's turn); kdrawnOwnerB runs the two draw Curses and its own
// instant draws.
var kdrawnOwnerA = []cardset.Entry{
	{Name: "Tataru Taru", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Into the Story", Count: 2},
	{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Plains", Count: 12}, {Name: "Island", Count: 10},
}

var kdrawnOwnerB = []cardset.Entry{
	{Name: "Curse of Fool's Wisdom", Count: 4}, {Name: "Psychic Possession", Count: 4},
	{Name: "Vampiric Touch", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Into the Story", Count: 2},
	{Name: "Hill Giant", Count: 2}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
}

// both seat orders
var kdrawnOwnerMatchups = [][2][]cardset.Entry{{kdrawnOwnerA, kdrawnOwnerB}, {kdrawnOwnerB, kdrawnOwnerA}}

// kdrawnOwnerTally counts, on gorge's side, the Drawn-trigger pushes of each
// carrier (Tataru Taru's Treasure line, the Curses' life-gain / may-draw
// lines).
type kdrawnOwnerTally struct {
	games, tataru, curse, possession int
}

func kdrawnOwnerCheck(tal *kdrawnOwnerTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := &evs[seen]
			if ev.Kind != events.TriggerPush {
				continue
			}
			o := ref.E.G.Obj(ev.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Tataru Taru":
				tal.tataru++
			case "Curse of Fool's Wisdom":
				tal.curse++
			case "Psychic Possession":
				tal.possession++
			}
		}
		return nil
	}
}

// TestDrawnOwnerScopeLockstep: 64 seeds of each seat order with gorge as the
// expected answer: the owner-scoped Drawn lines fire identically in both
// engines (Tataru's off-turn-opponent Treasure with its once-each-turn limit,
// the Curses' enchanted-player life-gain and may-draw), 0 divergences and 0
// kernel stalls, and every carrier actually triggered.
func TestDrawnOwnerScopeLockstep(t *testing.T) {
	t.Parallel()
	var tal kdrawnOwnerTally
	sonRun(t, "drawnownerscope", kdrawnOwnerMatchups, 64, true, nil, kdrawnOwnerCheck(&tal))
	t.Logf("%d games: %d Tataru Taru pushes, %d Curse of Fool's Wisdom pushes, %d Psychic Possession pushes", tal.games, tal.tataru, tal.curse, tal.possession)
	if tal.tataru == 0 || tal.curse == 0 || tal.possession == 0 {
		t.Errorf("triggers never fired: Tataru %d, Curse %d, Possession %d (want each > 0)", tal.tataru, tal.curse, tal.possession)
	}
}

// TestDrawnOwnerScopeMutation: clearing the not-active-drawer condition on the
// lowered Tataru line (the kernel fires for the ACTIVE player's own draws too,
// which gorge never does) must make some seed diverge.
func TestDrawnOwnerScopeMutation(t *testing.T) {
	mut := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbTriggered && abs[j].Trig.Event == kop.TrigDrawn {
					abs[j].Trig.Flags6 &^= kop.Tf6DrawerNotActive
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	if n := sonRun(t, "drawnownerscope", kdrawnOwnerMatchups, 64, false, mut, nil); n == 0 {
		t.Fatal("clearing Tf6DrawerNotActive: no seed diverged")
	}
}

// drawnOwnerPodDecks is the four-seat script's line-up: seat 0's Tataru Taru,
// seats 1 and 2 each hold two Into the Story (a {5}{U}{U} draw-two) with seven
// Islands already out; seat 3 is the vanilla fourth player. The fixed start
// puts the lands and the Tataru on the battlefield and the draw instants in
// hand, so the scripted turn has no randomness before the draws.
func drawnOwnerPodDecks() [][]cardset.Entry {
	seat0 := []cardset.Entry{{Name: "Tataru Taru", Count: 4}, {Name: "Plains", Count: 36}}
	islander := func() []cardset.Entry {
		return []cardset.Entry{{Name: "Into the Story", Count: 2}, {Name: "Island", Count: 38}}
	}
	seat3 := []cardset.Entry{{Name: "Mountain", Count: 40}}
	return [][]cardset.Entry{seat0, islander(), islander(), seat3}
}

// drawnOwnerPodObs records one game of the script: the forced casts (seat and
// turn), and the most Treasure tokens seat 0 ever had out.
type drawnOwnerPodObs struct {
	casts    [4]int
	castTurn [4]int32
	treasure int
}

// drawnOwnerTreasures counts seat 0's Treasure tokens.
func drawnOwnerTreasures(g *state.Game) int {
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		if o := g.Obj(id); o != nil && cardName(o.Card) == "Treasure Token" {
			n++
		}
	}
	return n
}

// drawnOwnerPodTable builds the kernel table over the pod's cards and the
// four-seat matchup's deck ids.
func drawnOwnerPodTable(t *testing.T) (*cards.Registry, *kop.Table, cardset.Set, [][]kop.CardID) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	decks := drawnOwnerPodDecks()
	seen := map[string]bool{}
	var pool []*cards.Card
	for _, d := range decks {
		for _, e := range d {
			if seen[e.Name] {
				continue
			}
			seen[e.Name] = true
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			pool = append(pool, c)
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, "drawnownerscope-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, ids, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Tataru Taru", "Into the Story"} {
		found := false
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == n {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s is not in the table (refused?)", n)
		}
	}
	if dirac.TwoSeatOnly(tbl) {
		t.Fatalf("the table is two-seat only: %v", dirac.TwoSeatCauses(tbl))
	}
	return reg, tbl, set, ids
}

// drawnOwnerStart is the pod's fixed opening: seat 0 has a Tataru Taru and two
// Plains out; seats 1 and 2 have seven Islands out and both draw instants in
// hand; seat 3 has a Mountain out.
func drawnOwnerStart() *rules.StartSpec {
	return &rules.StartSpec{Seats: []rules.StartSeat{
		{Battlefield: []int{0, 4, 5}},
		{Battlefield: []int{2, 3, 4, 5, 6, 7, 8}, Hand: []int{0, 1}},
		{Battlefield: []int{2, 3, 4, 5, 6, 7, 8}, Hand: []int{0, 1}},
		{Battlefield: []int{0}},
	}}
}

// drawnOwnerPolicy plays the pod's script: during seat 0's turn each of seats
// 1 and 2 casts one draw instant (once -- both forced casts share a turn, so
// the once-each-turn limit is what the Treasure count tests); every other
// decision passes (asks fall back to the seeded random). obs records the
// forced casts and the most Treasure tokens seat 0 ever had out.
func drawnOwnerPolicy(seed uint64, m *Matchup, obs *drawnOwnerPodObs) Policy {
	base := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		g := c.G.E.G
		if n := drawnOwnerTreasures(g); n > obs.treasure {
			obs.treasure = n
		}
		if c.D.Kind != DPriority {
			return base.Pick(c)
		}
		if g.Active == 0 && (c.D.Player == 1 || c.D.Player == 2) && obs.casts[c.D.Player] == 0 {
			for _, o := range c.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Into the Story" {
					obs.casts[c.D.Player]++
					obs.castTurn[c.D.Player] = g.Turn
					return o
				}
			}
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActPass {
				return o
			}
		}
		return base.Pick(c)
	})
}

// TestFourSeatDrawnOwnerScope: four seats, fixed start. During seat 0's turn
// the two opponent seats each cast their draw instant (both drawers are
// opponents of Tataru Taru's controller and neither is the active player), so
// the trigger sees two draws and gorge's ActivationLimit$ 1 makes exactly one
// tapped Treasure -- the once-each-turn limit holds across all of the
// opponents' off-turn draws. gorge is the expected answer; the kernel must
// agree every decision.
func TestFourSeatDrawnOwnerScope(t *testing.T) {
	t.Parallel()
	reg, tbl, set, ids := drawnOwnerPodTable(t)
	labels := []string{"drawnownerscope-a", "drawnownerscope-b", "drawnownerscope-c", "drawnownerscope-d"}
	m0, err := KernelMatchupN(reg, tbl, labels, ids, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	m0.Start = drawnOwnerStart()
	const seeds = 8
	good := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		m := *m0
		obs := &drawnOwnerPodObs{}
		k, err := SetSide(set, tbl, cfg)(&m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(&m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
		res := LockstepLed(&m, seed, g, k, drawnOwnerPolicy(seed, &m, obs), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("4 seats, seed %d: diverged at decision %d: %s %s %v\nref %q\nker %q\n%s", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions, d.Detail)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("4 seats, seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if obs.casts[1] > 0 && obs.casts[2] > 0 && obs.castTurn[1] == obs.castTurn[2] && obs.treasure == 1 {
			good++
		}
		t.Logf("seed %d: casts seat1 %d (turn %d), seat2 %d (turn %d), %d Treasure", seed, obs.casts[1], obs.castTurn[1], obs.casts[2], obs.castTurn[2], obs.treasure)
	}
	if good == 0 {
		t.Fatalf("no game had both opponents drawing off turn with exactly one Treasure made")
	}
}
