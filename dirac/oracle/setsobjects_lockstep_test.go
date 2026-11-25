package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CMM/MH2 object-movement shapes kgen lowers, in lockstep, one matchup
// per shape over the setsobjects fixture pool:
//
//   - Perilous Vault, Urza's Ruinous Blast (lower_changezoneall.go
//     czallParams3): UseAllOriginZones$ True on a battlefield sweep with no
//     Shuffle$, target or Defined$ reads as gorge's unscoped every-player
//     sweep (changeZoneAllPlayers' scope flag feeds only the shuffle tail);
//   - Chrome Courier (lower_zone.go dig): Dig's ChangeType$ Card, a key
//     neither Forge's DigEffect nor gorge's effDig reads;
//   - Sidisi, Brood Tyrant (lower_trigzonebatch.go): ChangesZoneAll Library
//     to Graveyard, TrigToGraveyard From Library collapsed per batch;
//   - Braids, Conjurer Adept (lower_handmove.go handMoveDefinedOwner):
//     DefinedPlayer$ TriggeredPlayer on a Phase trigger, the upkeep's active
//     player moves from their own hand, at both seats.
//
// The same shapes lower Devourer of Memory and Unshakable Tail (the
// Library->Graveyard batch line, in Sidisi's matchup) and Underworld Cerberus
// (UseAllOriginZones$ on a graveyard-to-hand sweep with no Shuffle$).
//
// The expected answer at every decision is gorge's.

type soMatchup struct {
	card string
	deck []cardset.Entry
	opp  []cardset.Entry
}

var soOpp = []cardset.Entry{{Name: "Grizzly Bears", Count: 14}, {Name: "Shock", Count: 6}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10}}

var soMatchups = []soMatchup{
	{card: "Perilous Vault", deck: []cardset.Entry{
		{Name: "Perilous Vault", Count: 6}, {Name: "Urza's Ruinous Blast", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 6},
		{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 20},
	}, opp: soOpp},
	{card: "Chrome Courier", deck: []cardset.Entry{
		{Name: "Chrome Courier", Count: 10}, {Name: "Ornithopter", Count: 8}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
	}, opp: soOpp},
	{card: "Sidisi, Brood Tyrant", deck: []cardset.Entry{
		{Name: "Sidisi, Brood Tyrant", Count: 6}, {Name: "Devourer of Memory", Count: 4}, {Name: "Unshakable Tail", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
	}, opp: soOpp},
	{card: "Underworld Cerberus", deck: []cardset.Entry{
		{Name: "Underworld Cerberus", Count: 8}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	}, opp: []cardset.Entry{{Name: "Murder", Count: 8}, {Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 11}, {Name: "Forest", Count: 11}}},
	{card: "Braids, Conjurer Adept", deck: []cardset.Entry{
		{Name: "Braids, Conjurer Adept", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Ornithopter", Count: 4},
		{Name: "Island", Count: 16}, {Name: "Forest", Count: 4},
	}, opp: soOpp},
}

// soReach is what the games did, read from gorge's log.
type soReach struct {
	exiled    int    // nonland permanents moved battlefield -> exile
	blasts    int    // Urza's Ruinous Blast resolved
	vaults    int    // Perilous Vault exiled as its own cost
	gained3   int    // Chrome Courier's +3 life (an artifact was taken)
	milledCr  int    // creature cards moved library -> graveyard
	zombies   int    // tokens created (Sidisi's Zombies)
	braidsPut [2]int // Grizzly Bears/Ornithopter moved hand -> battlefield, per owner
	tailCast  int    // Unshakable Tail / Devourer of Memory: their library->graveyard line resolved (a Clue, or the pump)
	cerberus  int    // creature cards moved graveyard -> hand (Underworld Cerberus's dies line)
}

func soScan(g *GorgeSide) soReach {
	var r soReach
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.Resolve:
			switch name(ev.Obj) {
			case "Urza's Ruinous Blast":
				r.blasts++
			case "Unshakable Tail", "Devourer of Memory":
				r.tailCast++
			}
		case events.LifeChange:
			if ev.Amount == 3 {
				r.gained3++
			}
		case events.TokenCreate, events.CardToken:
			r.zombies++ // the only token maker in these decks is Sidisi
		case events.MoveZone:
			n := name(ev.Obj)
			switch {
			case ev.From == state.ZBattlefield && ev.To == state.ZExile && n == "Perilous Vault":
				r.vaults++
			case ev.From == state.ZBattlefield && ev.To == state.ZExile:
				r.exiled++
			case ev.From == state.ZLibrary && ev.To == state.ZGraveyard && (n == "Grizzly Bears" || n == "Sidisi, Brood Tyrant"):
				r.milledCr++
			case ev.From == state.ZGraveyard && ev.To == state.ZHand && n != "Unshakable Tail":
				r.cerberus++
			case ev.From == state.ZHand && ev.To == state.ZBattlefield && (n == "Grizzly Bears" || n == "Ornithopter"):
				if o := g.E.G.Obj(ev.Obj); o != nil && int(o.Owner) < 2 {
					r.braidsPut[o.Owner]++
				}
			}
		}
	}
	return r
}

func TestSetsObjectsLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("setsobjects")
	if err != nil {
		t.Fatalf("setsobjects tables: %v (run make dirac-tables)", err)
	}
	for _, mu := range soMatchups {
		t.Run(strings.NewReplacer(" ", "", ",", "", "'", "").Replace(mu.card), func(t *testing.T) {
			t.Parallel()
			decks := [2][]cardset.Entry{mu.deck, mu.opp}
			var gd, sides [2][]*cards.Card
			for s := range decks {
				gd[s] = expandEntries(t, reg, decks[s])
				if len(gd[s]) != 40 {
					t.Fatalf("deck %d: %d cards", s, len(gd[s]))
				}
			}
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatalf("%s: the pool's table: %v", mu.card, err)
			}
			var tot soReach
			for seed := uint64(1); seed <= 30; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchupSide(reg, [2]string{"so-" + mu.card + "-a", "so-" + mu.card + "-b"}, gd, sides, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				g, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				pol := poisonhighPolicy(seed, map[string]int{}, map[string]int{}) // casts and activates whenever it can
				res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
				CloseSide(k)
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Errorf("%s seed %d: kernel stalled: %s", mu.card, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu.card, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				r := soScan(g)
				tot.exiled += r.exiled
				tot.blasts += r.blasts
				tot.vaults += r.vaults
				tot.gained3 += r.gained3
				tot.milledCr += r.milledCr
				tot.zombies += r.zombies
				tot.braidsPut[0] += r.braidsPut[0]
				tot.braidsPut[1] += r.braidsPut[1]
				tot.tailCast += r.tailCast
				tot.cerberus += r.cerberus
			}
			t.Logf("%s: %+v", mu.card, tot)
			var bad bool
			switch mu.card {
			case "Perilous Vault":
				bad = tot.vaults < 3 || tot.blasts < 3 || tot.exiled < 10
			case "Chrome Courier":
				bad = tot.gained3 < 3
			case "Sidisi, Brood Tyrant":
				bad = tot.milledCr < 5 || tot.zombies < 3 || tot.tailCast < 3
			case "Underworld Cerberus":
				bad = tot.cerberus < 3
			case "Braids, Conjurer Adept":
				bad = tot.braidsPut[0] < 3 || tot.braidsPut[1] < 3
			}
			if bad {
				t.Errorf("reach %+v: the scenario the lowering must hold was not reached", tot)
			}
		})
	}
}
