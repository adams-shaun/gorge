package oracle

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: ChangeZone ForgetChanged$ True (the moved
// card leaves the remembered set) on the five Std carriers. Each branch is
// counted from gorge's event log (a resolution's segment: its Resolve event up
// to the next Priority or Resolve), never from the carrier's cast.
var kfcDecks = []struct {
	name  string
	seeds uint64
	decks [2][]cardset.Entry
}{{
	// Guidelight Pathmaker: the searched artifact enters (mana value <= 2) or
	// goes to hand (> 2), with Shuffle$ False and a Defined$ Remembered chain
	name: "guidelight", seeds: 128,
	decks: [2][]cardset.Entry{{
		{Name: "Guidelight Pathmaker", Count: 6}, {Name: "Memnite", Count: 3}, {Name: "Ornithopter", Count: 3}, {Name: "Sol Ring", Count: 2},
		{Name: "Mind Stone", Count: 2}, {Name: "Juggernaut", Count: 3}, {Name: "Gilded Lotus", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 9},
	}, {
		{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
	}},
}, {
	// Troop of Ponies: two basic lands found, one searched onto the
	// battlefield tapped (ForgetChanged$ on the pick), the other to hand
	name: "troop", seeds: 128,
	decks: [2][]cardset.Entry{{
		{Name: "Troop of Ponies", Count: 8}, {Name: "Savannah Lions", Count: 4}, {Name: "Healer's Hawk", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
	}, {
		{Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
	}},
}, {
	// Archdruid's Charm: a creature to hand or a land onto the battlefield
	// tapped, then the Remembered cleanup
	name: "archdruid", seeds: 128,
	decks: [2][]cardset.Entry{{
		{Name: "Archdruid's Charm", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4}, {Name: "Serra Angel", Count: 1},
		{Name: "Forest", Count: 15}, {Name: "Plains", Count: 6},
	}, {
		{Name: "Savannah Lions", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Mountain", Count: 12},
	}},
}, {
	// Bre of Clan Stoutarm and Breaching Dragonstorm: DigUntil exiles, the
	// optional free cast, and the exile-to-hand rider; a Dragon returns
	// Breaching Dragonstorm to hand
	name: "bre-breaching", seeds: 128,
	decks: [2][]cardset.Entry{{
		{Name: "Bre of Clan Stoutarm", Count: 5}, {Name: "Breaching Dragonstorm", Count: 5}, {Name: "Healer's Hawk", Count: 5},
		{Name: "Lightning Helix", Count: 3}, {Name: "Dragon Hatchling", Count: 2}, {Name: "Shivan Dragon", Count: 1}, {Name: "Serra Angel", Count: 1},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9},
	}, {
		{Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
	}},
}}

const kfcMinBranch = 10

// kfcBranches names every counted ForgetChanged move (each must reach kfcMinBranch).
var kfcBranches = []string{
	"guidelight: artifact onto the battlefield", "guidelight: artifact into hand",
	"troop: land onto the battlefield and land into hand",
	"archdruid: land onto the battlefield", "archdruid: creature into hand",
	"bre: exiled card into hand", "breaching: exiled card into hand",
}

// kfcScan adds one game's branch counts from gorge's event log.
func kfcScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	gm := g.E.G
	evs := g.E.L.Events
	for i := 0; i < len(evs); i++ {
		if evs[i].Kind != events.Resolve {
			continue
		}
		so := gm.Obj(evs[i].Obj)
		if so == nil || so.Ability == nil && !strings.HasSuffix(sourceCardName(gm, evs[i].Obj), "Charm") {
			continue
		}
		name := sourceCardName(gm, evs[i].Obj)
		libBf, libHand, exHand := 0, 0, 0
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			if evs[j].Kind != events.MoveZone {
				continue
			}
			switch {
			case evs[j].From == state.ZLibrary && evs[j].To == state.ZBattlefield:
				libBf++
			case evs[j].From == state.ZLibrary && evs[j].To == state.ZHand:
				libHand++
			case evs[j].From == state.ZExile && evs[j].To == state.ZHand:
				exHand++
			}
		}
		switch name {
		case "Guidelight Pathmaker":
			if libBf > 0 {
				n["guidelight: artifact onto the battlefield"]++
			}
			if libHand > 0 {
				n["guidelight: artifact into hand"]++
			}
		case "Troop of Ponies":
			if libBf > 0 && libHand > 0 {
				n["troop: land onto the battlefield and land into hand"]++
			}
		case "Archdruid's Charm":
			if libBf > 0 {
				n["archdruid: land onto the battlefield"]++
			}
			if libHand > 0 {
				n["archdruid: creature into hand"]++
			}
		case "Bre of Clan Stoutarm":
			if exHand > 0 {
				n["bre: exiled card into hand"]++
			}
		case "Breaching Dragonstorm":
			if exHand > 0 {
				n["breaching: exiled card into hand"]++
			}
		}
	}
}

func TestForgetchangedLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("forgetchanged")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played, n := map[string]int{}, map[string]int{}
	stalls := map[string]int{}
	for mi, mu := range kfcDecks {
		decks := kcastDeck(t, reg, mu.decks)
		tbl, _, _, err := set.Matchup(mu.decks[0], mu.decks[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= mu.seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kfc-a", "kfc-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			inner := eagerPolicy(seed, played)
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls[res.Stall]++
			}
			kfcScan(gs, n)
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d (%s) seed %d last actions %v", mi, mu.name, seed, actionStrings(res.Actions[lo:d.At]))
				kcastDump(t, gs)
				t.Errorf("matchup %d (%s) seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, mu.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	var ss []string
	for k := range stalls {
		ss = append(ss, k)
	}
	sort.Strings(ss)
	t.Logf("%d games, kernel stalls %v", games, stalls)
	if len(ss) > 0 {
		t.Errorf("kernel stalls: %v", ss)
	}
	for _, b := range kfcBranches {
		t.Logf("branch %-55s %d", b, n[b])
		if n[b] < kfcMinBranch {
			t.Errorf("branch %q: %d, want at least %d", b, n[b], kfcMinBranch)
		}
	}
}
