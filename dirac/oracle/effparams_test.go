package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// effparams pool, exercising the two Effect parameters the lowering used to
// refuse as unread:
//
//   - EffectOwner$ You -- Laughing Jasper Flint and Redcap Gutter-Dweller.
//     gorge resolves it with EffectOwnerPlayers, whose empty value and You
//     both name the resolving controller (engine/effects/effect.go:197-199,
//     engine/effects/context.go:1804-1805): the owner the kernel already
//     models.
//   - a MayPlay static's EffectZone$ Command -- Count on Luck and Endrider
//     Spikespitter. EffectZone$ restricts a static to its source's zone
//     (engine/rules/statics_cast.go:833-841); an Effect source is always in
//     the command zone, so Command never denies.
//
// The pool also carries outlaws (Assassin/Mercenary/Pirate/Rogue/Warlock
// creatures) for Jasper Flint's X, spare creatures for Redcap's sacrifice
// cost, cheap spells so exiled cards get cast, and start-your-engines
// creatures so Endrider can reach max speed.

// keffDecks are the two 40-card decks, mono-red with a black splash. Both
// sides play the carriers so either player can exercise the permission.
var keffDecks = [2][]cardset.Entry{
	{
		{Name: "Count on Luck", Count: 3}, {Name: "Endrider Spikespitter", Count: 2},
		{Name: "Laughing Jasper Flint", Count: 2}, {Name: "Redcap Gutter-Dweller", Count: 2},
		{Name: "Hired Poisoner", Count: 2}, {Name: "Hired Claw", Count: 2},
		{Name: "Mountain Bandit", Count: 2}, {Name: "Prickly Boggart", Count: 2},
		{Name: "Burnout Bashtronaut", Count: 2}, {Name: "Walking Sarcophagus", Count: 2},
		{Name: "Lightning Bolt", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Count on Luck", Count: 3}, {Name: "Endrider Spikespitter", Count: 2},
		{Name: "Laughing Jasper Flint", Count: 2}, {Name: "Redcap Gutter-Dweller", Count: 2},
		{Name: "Vampire Cutthroat", Count: 2}, {Name: "Vengeful Bloodwitch", Count: 2},
		{Name: "Swab Goblin", Count: 2}, {Name: "Burnout Bashtronaut", Count: 2},
		{Name: "Walking Sarcophagus", Count: 1}, {Name: "Gray Ogre", Count: 1},
		{Name: "Shock", Count: 3}, {Name: "Raging Goblin", Count: 2},
		{Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 5},
	},
}

// keffGame is what one game showed. The thresholds guard that the lowered
// permissions are the ones the games actually exercise.
type keffGame struct {
	countExiles int // cards exiled by Count on Luck's Dig
	countCasts  int // of those, casts from exile
	redcapSacs  int // Redcap Gutter-Dweller's sacrifice paths
	jasperBatch int // Laughing Jasper Flint's exile batches
	endrider    int // games where Endrider Spikespitter's max-speed trigger fired
}

// keffScan counts the mechanics in gorge's event log.
func keffScan(g *GorgeSide) keffGame {
	var km keffGame
	if g == nil {
		return km
	}
	name := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	exiled := map[state.ObjID]bool{} // Count on Luck's exiled cards
	trig := ""                       // the source of the latest trigger or ability pushed
	jasperBatch := false
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TriggerPush, events.AbilityPush:
			trig = name(ev.Obj)
			if ev.Kind == events.TriggerPush && trig == "Endrider Spikespitter" {
				km.endrider++
			}
			if trig == "Laughing Jasper Flint" {
				jasperBatch = false
			}
		case events.MoveZone:
			if ev.From == state.ZLibrary && ev.To == state.ZExile {
				switch trig {
				case "Count on Luck":
					km.countExiles++
					exiled[ev.Obj] = true
				case "Laughing Jasper Flint":
					if !jasperBatch {
						km.jasperBatch++
						jasperBatch = true
					}
				}
			}
			if trig == "Redcap Gutter-Dweller" && events.IsSacrifice(ev) && name(ev.Obj) != trig {
				km.redcapSacs++
			}
		case events.PutOnStack:
			if ev.From == state.ZExile && exiled[ev.Obj] {
				km.countCasts++
			}
		}
	}
	return km
}

// keffTally is one run's counters.
type keffTally struct {
	total    keffGame
	games    int
	diverged bool
	stalls   map[string]int
	played   map[string]int
}

// keffSetup loads the corpus, the generated pool and its matchup table.
func keffSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effparams")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(keffDecks[0], keffDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl
}

// keffRun plays the matchup for seeds 1..seeds over tbl (already mutated by
// the caller when wanted) and returns the tally.
func keffRun(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, seeds uint64) keffTally {
	t.Helper()
	tal := keffTally{stalls: map[string]int{}, played: map[string]int{}}
	var decks [2][]*cards.Card
	for s, d := range keffDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"keff-a", "keff-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var km keffGame
		rc := RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25,
			Finish: func(g *GorgeSide) { km = keffScan(g) }}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, tal.played), rc)
		CloseSide(k)
		tal.games++
		tal.total.countExiles += km.countExiles
		tal.total.countCasts += km.countCasts
		tal.total.redcapSacs += km.redcapSacs
		tal.total.jasperBatch += km.jasperBatch
		if km.endrider > 0 {
			tal.total.endrider++
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			tal.stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Logf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			tal.diverged = true
			return tal
		}
	}
	return tal
}

func (tal keffTally) branchLog() string {
	var names []string
	for n := range tal.played {
		names = append(names, n)
	}
	sort.Strings(names)
	var b []string
	for _, n := range names {
		b = append(b, fmt.Sprintf("%s=%d", n, tal.played[n]))
	}
	return strings.Join(b, " ")
}

func TestEffparamsLockstep(t *testing.T) {
	t.Parallel()

	reg, set, tbl := keffSetup(t)
	tal := keffRun(t, reg, set, tbl, 256)
	t.Logf("%d games: %+v; stalls %v; plays %s", tal.games, tal.total, tal.stalls, tal.branchLog())
	if tal.diverged {
		t.Fatalf("lockstep diverged")
	}
	if len(tal.stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", tal.stalls)
	}
	if tal.total.countExiles < 20 {
		t.Errorf("Count on Luck exiled %d cards, want at least 20", tal.total.countExiles)
	}
	if tal.total.countCasts < 5 {
		t.Errorf("Count on Luck's exiled cards cast %d times, want at least 5", tal.total.countCasts)
	}
	if tal.total.redcapSacs < 10 {
		t.Errorf("Redcap Gutter-Dweller sacrificed %d times, want at least 10", tal.total.redcapSacs)
	}
	if tal.total.jasperBatch < 10 {
		t.Errorf("Laughing Jasper Flint exiled %d batches, want at least 10", tal.total.jasperBatch)
	}
	t.Logf("Endrider Spikespitter reached max speed in %d of %d games", tal.total.endrider, tal.games)
}

// keffMayPlayDurationDropped deep-copies a generated table and rewrites
// Count on Luck's OpMayPlay duration from DurEOT to DurPermanent: the kernel
// keeps "you may play that card this turn" past the cleanup that ends it in
// gorge. (Dropping the MayPlayExileOnMoved flag instead is not enough: the
// kernel filters a permission by the card's zone, so a stale remembered id
// whose card already left exile poses no extra option.) The duration is
// exercised every game that exiles a card it does not cast, so the lockstep
// must diverge within 128 seeds.
func keffMayPlayDurationDropped(t *testing.T, tbl *kop.Table) *kop.Table {
	t.Helper()
	cp := *tbl
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cp.Cards {
		abs := make([]kop.Ability, len(cp.Cards[i].Abilities))
		copy(abs, cp.Cards[i].Abilities)
		for j := range abs {
			if abs[j].Prog != nil {
				abs[j].Prog = append([]kop.Instr(nil), abs[j].Prog...)
			}
		}
		cp.Cards[i].Abilities = abs
	}
	found := false
	for i := range cp.Cards {
		if cp.Cards[i].Name != "Count on Luck" {
			continue
		}
		for a := range cp.Cards[i].Abilities {
			for j := range cp.Cards[i].Abilities[a].Prog {
				in := &cp.Cards[i].Abilities[a].Prog[j]
				if in.Op != kop.OpMayPlay || in.A.N != kop.DurEOT {
					continue
				}
				in.A = kop.Const(kop.DurPermanent)
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("Count on Luck: no OpMayPlay with the EOT duration in the table")
	}
	return &cp
}

func TestEffparamsMutation(t *testing.T) {
	t.Parallel()

	reg, set, tbl := keffSetup(t)
	mut := keffMayPlayDurationDropped(t, tbl)
	tal := keffRun(t, reg, set, mut, 128)
	if !tal.diverged {
		t.Fatalf("keeping Count on Luck's MayPlay permission past EOT never diverged in 128 seeds")
	}
	t.Logf("mutation diverged after %d games; counters %+v", tal.games, tal.total)
}
