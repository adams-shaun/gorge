package oracle

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The gap-effect-containers carriers' lockstep (dirac/kgen
// lower_cantblockplain.go, lower_noregen.go's noRegenMark, kernel
// OpMarkNoRegen): the Effect-container shapes the slice lowers today, one
// deck pair per carrier's synthetic tail bucket, both payment surfaces.
// Every bucket's enablers are regeneration sources or bodies for the
// can't-block grant to matter against, drawn from the same bucket the
// carrier sits in (TailBucketOf, one pool table per run):
//
//   - Ghostform, Runed Arch: the no-ValidBlocker$ CantBlockBy static over the
//     chain's own targets (dirac/kgen/lower_cantblockplain.go) — the
//     Unblockable keyword grant against Charging Troll's and Skitterskin's
//     regeneration walls.
//
//   - Furnace Brood, Hurr Jackal, Orcish Healer, Gravebind, Engulfing Flames,
//     Jaya Ballard: the Effect-delivered CantRegenerate mark (kernel
//     OpMarkNoRegen) against the bucket's regeneration (Zombie Musher, Wall
//     of Bone, Poultice Sliver, Boneknitter, Rhys the Exiled); Orcish
//     Healer's own Regenerate abilities and Soul Channeling feed the shields
//     the mark must refuse to spend.
//
//   - Kick in the Door: the ValidBlocker$ filter record (kernel ExHaste C bit
//     8) against the bucket's Walls and plain blockers. Only the
//     ForgetOnMoved$ Battlefield spelling lowers: Tower of Coireall,
//     Doorman, Joven's Tools and Firefright Mage lack it, and gorge's
//     remembered ObjID survives a zone change where the record's version
//     does not, so kgen refuses them.
//
//   - Duct Crawler, Screeching Griffin, Spin Engine, Shrewd Hatchling, Auriok Siege Sled, Burning-Tree Bloodscale: the
//     named-blocker record (C bit 9), "target creature can't block this
//     creature this turn" (Kozilek's Pathfinder's {C} ability is out of reach of the
//     basic lands every pool holds, so it has no pair).
//
// kgen refuses the no-ForgetOnMoved$ spellings of the plain CantBlockBy
// shape (Cephalid Pathmage, Krydle, Open into Wonder: gorge's Remembered set
// keeps the id across a move, the keyword grant does not, CR 400.7), so they
// are not here. A divergence, an unmodelled stall or a hang fails.
//
// A pair's pool is its first carrier's synthetic tail bucket
// (cardset.TailBucketOf), and every non-basic card of the deck must sit in
// the same bucket, or the test fails naming the stray card.
type ecPair struct {
	name  string
	pool  string // the first entry's tail bucket, set by ecSets
	decks [2][]cardset.Entry
}

var ecPairs = []ecPair{
	{name: "orcish-healer", decks: ecDecks(mf,
		cardset.Entry{Name: "Orcish Healer", Count: 6},
		cardset.Entry{Name: "Zombie Musher", Count: 4},
		cardset.Entry{Name: "Soul Channeling", Count: 2},
	)},
	{name: "furnace-brood", decks: ecDecks(mf,
		cardset.Entry{Name: "Furnace Brood", Count: 6},
		cardset.Entry{Name: "Grave Defiler", Count: 4},
	)},
	{name: "ghostform", decks: ecDecks(mf,
		cardset.Entry{Name: "Ghostform", Count: 6},
		cardset.Entry{Name: "Charging Troll", Count: 4},
		cardset.Entry{Name: "Chilling Apparition", Count: 2},
	)},
	{name: "hurr-jackal", decks: ecDecks(mf,
		cardset.Entry{Name: "Hurr Jackal", Count: 6},
		cardset.Entry{Name: "Wall of Bone", Count: 4},
		cardset.Entry{Name: "Silvos, Rogue Elemental", Count: 2},
	)},
	{name: "gravebind", decks: ecDecks(mf,
		cardset.Entry{Name: "Gravebind", Count: 6},
		cardset.Entry{Name: "Poultice Sliver", Count: 4},
		cardset.Entry{Name: "Clockwork Gnomes", Count: 2},
	)},
	{name: "runed-arch-jaya", decks: ecDecks(mf,
		cardset.Entry{Name: "Runed Arch", Count: 4},
		cardset.Entry{Name: "Jaya Ballard, Task Mage", Count: 4},
		cardset.Entry{Name: "Rhys the Exiled", Count: 2},
		cardset.Entry{Name: "Skitterskin", Count: 2},
	)},
	{name: "engulfing-flames", decks: ecDecks(mf,
		cardset.Entry{Name: "Engulfing Flames", Count: 6},
		cardset.Entry{Name: "Boneknitter", Count: 4},
		cardset.Entry{Name: "Will-o'-the-Wisp", Count: 2},
	)},
	{name: "kick-in-the-door", decks: ecDecks(mf,
		cardset.Entry{Name: "Kick in the Door", Count: 4},
		cardset.Entry{Name: "Wall of Vines", Count: 4},
		cardset.Entry{Name: "Deputized Protester", Count: 4},
		cardset.Entry{Name: "Mammoth Spider", Count: 4},
	)},
	{name: "duct-crawler", decks: ecDecks([]string{"Mountain", "Plains"},
		cardset.Entry{Name: "Duct Crawler", Count: 4},
		cardset.Entry{Name: "Angelic Wall", Count: 4},
		cardset.Entry{Name: "Mogg Flunkies", Count: 4},
		cardset.Entry{Name: "Ferocious Tigorilla", Count: 3},
	)},
	{name: "screeching-griffin", decks: ecDecks([]string{"Plains", "Mountain", "Forest"},
		cardset.Entry{Name: "Screeching Griffin", Count: 4},
		cardset.Entry{Name: "Bannerhide Krushok", Count: 4},
		cardset.Entry{Name: "Knight of New Benalia", Count: 4},
	)},
	{name: "spin-engine", decks: ecDecks([]string{"Mountain"},
		cardset.Entry{Name: "Spin Engine", Count: 4},
		cardset.Entry{Name: "Lightning Visionary", Count: 4},
		cardset.Entry{Name: "Nimble-Blade Khenra", Count: 4},
		cardset.Entry{Name: "Canyon Lurkers", Count: 3},
	)},
	{name: "shrewd-hatchling", decks: ecDecks([]string{"Mountain", "Island"},
		cardset.Entry{Name: "Shrewd Hatchling", Count: 4},
		cardset.Entry{Name: "Kraken Hatchling", Count: 4},
		cardset.Entry{Name: "Halberdier", Count: 4},
		cardset.Entry{Name: "Akki Blizzard-Herder", Count: 2},
	)},
	{name: "auriok-siege-sled", decks: ecDecks(mf,
		cardset.Entry{Name: "Auriok Siege Sled", Count: 4},
		cardset.Entry{Name: "Wall of Wood", Count: 4},
		cardset.Entry{Name: "Wall of Razors", Count: 4},
		cardset.Entry{Name: "Norwood Ranger", Count: 4},
	)},
}

// mf is the Mountain and Forest padding the first pairs use.
var mf = []string{"Mountain", "Forest"}

// ecBucketPool names the synthetic pool of the pair's first card (its
// carrier) and fails when another non-basic card sits in a different bucket:
// the pool holds one bucket's cards, so the witness is searched in it, never
// named.
func ecBucketPool(t *testing.T, p ecPair) string {
	t.Helper()
	carrier := p.decks[0][0].Name
	want := cardset.TailBucketOf(carrier)
	for _, e := range p.decks[0] {
		if cardset.IsSynthBasic(e.Name) {
			continue
		}
		if got := cardset.TailBucketOf(e.Name); got != want {
			t.Fatalf("%s: %s is in tail bucket %d, the carrier %s in %d", p.name, e.Name, got, carrier, want)
		}
	}
	return fmt.Sprintf("tail-%02d", want)
}

// ecDecks builds the pair's deck lists from the named entries, padded with
// Mountains and Forests (SynthBasics: every synthetic pool holds them); both
// seats hold the same list, like every lockstep deck pair here.
func ecDecks(lands []string, entries ...cardset.Entry) [2][]cardset.Entry {
	d := append([]cardset.Entry(nil), entries...)
	n := 0
	for _, e := range d {
		n += e.Count
	}
	for i := 0; n < 40; i++ {
		d = append(d, cardset.Entry{Name: lands[i%len(lands)], Count: 1})
		n++
	}
	return [2][]cardset.Entry{d, append([]cardset.Entry(nil), d...)}
}

// ecSets loads each pair's pool once; a checkout with no tables skips, like
// every other table-reading test.
func ecSets(t *testing.T) map[string]cardset.Set {
	t.Helper()
	out := map[string]cardset.Set{}
	for i := range ecPairs {
		p := &ecPairs[i]
		p.pool = ecBucketPool(t, *p)
		if _, ok := out[p.pool]; ok {
			continue
		}
		s, err := cardset.SynthByName(p.pool)
		if err != nil {
			if errors.Is(err, cardset.ErrNoCorpus) {
				t.Skipf("no card tables: %v", err)
			}
			if s == nil {
				t.Skipf("no synthetic tables for %s: %v", p.pool, err)
			}
			t.Fatal(err)
		}
		out[p.pool] = s
	}
	return out
}

// TestEffectContainersLockstep plays the carriers' bucket decks (13
// pairs), seeds 1..30 for 390 games, each seed's surface alternating gorge's
// autopay and full payment, every decision random.
func TestEffectContainersLockstep(t *testing.T) {
	sets := ecSets(t)
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ecPairs {
		p := p
		t.Run(p.name, func(t *testing.T) {
			set := sets[p.pool]
			var decks [2][]*cards.Card
			for s, d := range p.decks {
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
					t.Fatalf("%s deck %d: %d cards", p.name, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(p.decks[0], p.decks[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := 1; seed <= 30; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"ec-a", "ec-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, uint64(seed))
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, uint64(seed), k, RandomPolicy(uint64(seed)), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Errorf("%s seed %d: %s", p.name, seed, res.Stall)
					return
				}
				if d := res.Div; d != nil {
					t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						p.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					return
				}
			}
		})
	}
}
