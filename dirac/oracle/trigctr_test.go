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

// The lockstep: TriggeredCard$CardCounters.<KIND|ALL>
// on a leaves-the-battlefield trigger reads the counters the object had on
// the battlefield (gorge evalRefProperty's trigger LKI; the kernel's e.li
// record), and TriggeredCard$CardNumColors on a SpellCast trigger reads the
// spell's colours.
//
//   - Marketback Walker draws per +1/+1 counter when it dies;
//   - Aerith Gainsborough spreads her +1/+1 counters when she dies;
//   - The Ooze makes a Mutagen per +1/+1 counter on a creature leaving to
//     any zone (destroyed, bounced, exiled);
//   - Shadow Urchin digs per counter of every kind (its blight's -1/-1s);
//   - Ancient Cornucopia gains a life per colour of the cast spell.
//
// Both decks carry The Ooze and Marketback Walker, so either side's removal
// (Murder, Terminate, Unsummon, Swords to Plowshares) moves a creature with
// counters while its owner's Ooze watches.
var ktrigctrDecks = [2][]cardset.Entry{
	{
		{Name: "Aerith Gainsborough", Count: 3}, {Name: "Marketback Walker", Count: 3},
		{Name: "The Ooze", Count: 2}, {Name: "Ancient Cornucopia", Count: 3},
		{Name: "Ajani's Pridemate", Count: 3}, {Name: "Healer's Hawk", Count: 3},
		{Name: "Watchwolf", Count: 3}, {Name: "Lightning Helix", Count: 2},
		{Name: "Snakeskin Veil", Count: 1}, {Name: "Swords to Plowshares", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 2},
	},
	{
		{Name: "Shadow Urchin", Count: 4}, {Name: "Marketback Walker", Count: 3},
		{Name: "The Ooze", Count: 2}, {Name: "Ancient Cornucopia", Count: 3},
		{Name: "Unsummon", Count: 3}, {Name: "Murder", Count: 3},
		{Name: "Terminate", Count: 3},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 4},
	},
}

// ktrigctrTally is what the played games reached, read off gorge's event
// log (gorge leads the lockstep).
type ktrigctrTally struct {
	games, stalls int
	// reads: a counter-head trigger whose object left the battlefield with
	// a non-zero count of the kind it reads; ooze/oozeNonGY: The Ooze's
	// share, and those leaving to a zone other than a graveyard
	reads, ooze, oozeNonGY int
	// corn2: Ancient Cornucopia resolutions gaining 2 or more life
	corn2  int
	played map[string]int
}

// ktrigctrScan replays the log's counter changes per object, and at each
// battlefield departure checks which counter-head trigger saw it.
func ktrigctrScan(g *GorgeSide, tl *ktrigctrTally) {
	if g == nil {
		return
	}
	gm := g.E.G
	evs := g.E.L.Events
	ctr := map[state.ObjID]map[string]int32{}
	var ooze, urchin [8]int // per owner, on the battlefield
	for i, ev := range evs {
		switch ev.Kind {
		case events.CounterChange:
			if ctr[ev.Obj] == nil {
				ctr[ev.Obj] = map[string]int32{}
			}
			ctr[ev.Obj][ev.Counter] += ev.Amount
		case events.MoveZone:
			o := gm.Obj(ev.Obj)
			if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
				continue
			}
			name, own := o.Card.Faces[0].Name, int(o.Owner)&7
			if ev.To == state.ZBattlefield {
				switch name {
				case "The Ooze":
					ooze[own]++
				case "Shadow Urchin":
					urchin[own]++
				}
			}
			if ev.From != state.ZBattlefield {
				continue
			}
			var p1, all int32
			for k, n := range ctr[ev.Obj] {
				if n > 0 {
					all += n
					if k == "P1P1" {
						p1 = n
					}
				}
			}
			creature := false
			for _, t := range o.Card.Faces[0].Types {
				if t == "Creature" {
					creature = true
				}
			}
			dies := ev.To == state.ZGraveyard
			if dies && p1 > 0 && (name == "Marketback Walker" || name == "Aerith Gainsborough") {
				tl.reads++
			}
			if creature && p1 > 0 && ooze[own] > 0 {
				tl.reads++
				tl.ooze++
				if !dies {
					tl.oozeNonGY++
				}
			}
			if creature && dies && all > 0 && urchin[own] > 0 {
				tl.reads++
			}
			delete(ctr, ev.Obj)
			switch name {
			case "The Ooze":
				ooze[own]--
			case "Shadow Urchin":
				urchin[own]--
			}
		case events.Resolve:
			if kremName(gm, ev.Obj) != "Ancient Cornucopia" {
				continue
			}
			for _, nx := range evs[i+1:] {
				if nx.Kind == events.Resolve || nx.Kind == events.Priority {
					break
				}
				if nx.Kind == events.LifeChange && nx.Amount >= 2 {
					tl.corn2++
				}
			}
		}
	}
}

// ktrigctrRun plays seeds 1..seeds; it stops at the first divergence.
func ktrigctrRun(t *testing.T, seeds uint64, report bool) (*ktrigctrTally, *Divergence) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigctr")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range ktrigctrDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ktrigctrDecks[0], ktrigctrDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tl := &ktrigctrTally{played: map[string]int{}}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"ktrigctr-a", "ktrigctr-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		res := Lockstep(m, seed, k, trackingEagerPolicy(seed, tl.played, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		tl.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			tl.stalls++
			t.Logf("seed %d: stall %s", seed, res.Stall)
		}
		if res.Div != nil {
			if report {
				lo := max(res.Div.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
			}
			return tl, res.Div
		}
		ktrigctrScan(gs, tl)
	}
	return tl, nil
}

// TestTrigctrLockstep: 256 games, 0 divergences and 0 stalls, with enough
// non-zero LKI counter reads (The Ooze's leaves to every zone among them)
// and multicolour Cornucopia gains.
func TestTrigctrLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	tl, div := ktrigctrRun(t, 256, true)
	if div != nil {
		t.Fatalf("diverged at decision %d: %s %s %v %s\nref %v\nker %v", div.At, div.What, div.Field, div.Diff, div.Detail, div.RefOptions, div.KerOptions)
	}
	t.Logf("%d games, %d stalls: %d non-zero counter reads (The Ooze %d, %d to a non-graveyard zone), %d Cornucopia gains of 2+",
		tl.games, tl.stalls, tl.reads, tl.ooze, tl.oozeNonGY, tl.corn2)
	if tl.stalls != 0 {
		t.Errorf("kernel capacity stalls %d, want 0", tl.stalls)
	}
	if tl.reads < 30 {
		t.Errorf("%d non-zero counter reads, want at least 30", tl.reads)
	}
	if tl.oozeNonGY < 5 {
		t.Errorf("The Ooze saw %d non-graveyard leaves with counters, want at least 5", tl.oozeNonGY)
	}
	if tl.corn2 < 20 {
		t.Errorf("%d Cornucopia gains of 2 or more, want at least 20", tl.corn2)
	}
}

// TestTrigctrMutation reads the live object instead of the trigger's LKI
// (dirac.DebugTrigCtrLive): a departed creature's counters read 0, so the
// draws, tokens and digs shrink and the lockstep must diverge.
func TestTrigctrMutation(t *testing.T) {
	old := dirac.DebugTrigCtrLive
	dirac.DebugTrigCtrLive = true
	defer func() { dirac.DebugTrigCtrLive = old }()
	tl, div := ktrigctrRun(t, 128, false)
	if div == nil {
		t.Fatalf("no divergence in %d seeds with the live counter read", tl.games)
	}
	t.Logf("diverged on seed %d/128 at decision %d: %s %s", tl.games, div.At, div.What, div.Field)
}
