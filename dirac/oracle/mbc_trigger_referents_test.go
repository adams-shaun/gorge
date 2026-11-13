package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCTriggerReferents10Lowers: the four spellings of the MBC
// trigger-referents-10 family play identically in gorge and the kernel. Each
// subtest plays a card that carries the spelling, in games where it fires:
//
//   - TriggeredDefendingPlayer$CardsInHand (an SVar value head): Mr. Foxglove
//     draws the defending player's hand less its own.
//   - OptionalDecider$ TriggeredCardController on a Draw: a Selvala-shaped
//     probe (Selvala's own mana ability is another family); the entering
//     creature's controller answers, not the probe's.
//   - ValidCause$ SpellAbility.OppCtrl on a Discarded Card.Self trigger:
//     Guerrilla Tactics, discarded by an opponent's Mind Rot.
//   - TriggeredTarget in a DamageDoneOnce body over players: Anowon, the Ruin
//     Thief mills the damaged player (Rev, Tithe Extractor's spelling, whose
//     Effect$ ForgetOnMoved is another family).
//
// Every subtest requires its trigger to fire, so a game that never reaches
// the card is not a pass; without the lowering the card is refused and the
// set cannot be built.

// refsGame is one subtest: two decks, the card whose trigger must fire, and
// how often.
type refsGame struct {
	carrier string
	decks   [2][]string
	seeds   uint64
	fires   int
	attack  bool
}

func refsDeck(n map[string]int, fill string) []string {
	var names []string
	for _, k := range slices.Sorted(mapsKeys(n)) {
		for i := 0; i < n[k]; i++ {
			names = append(names, k)
		}
	}
	for len(names) < 40 {
		names = append(names, fill)
	}
	return names
}

func mapsKeys(m map[string]int) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// refsSet builds the kernel table of the decks' cards from corpus dir.
func refsSet(t *testing.T, corpus string, decks [2][]string) cardset.Set {
	t.Helper()
	var names []string
	for _, d := range decks {
		names = append(names, d...)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", corpus, "-card-list", list, "-pool", "printed-delve", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "printed-delve.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	return cardset.AllowUnverified(cardset.NewDataSet(pd))
}

// refsPlay runs g's lockstep games over reg and the kernel table of corpus,
// and returns how often the carrier's trigger went on the stack.
func refsPlay(t *testing.T, reg *cards.Registry, corpus string, g refsGame) (fired int, divs []string) {
	t.Helper()
	set := refsSet(t, corpus, g.decks)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(g.decks[0]), entries(g.decks[1]))
	if err != nil {
		t.Fatalf("%s: %v", g.carrier, err)
	}
	var cardDecks [2][]*cards.Card
	for s := range g.decks {
		for _, n := range g.decks[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], c)
		}
	}
	for seed := uint64(1); seed <= g.seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"refs-a", "refs-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				if ev.Kind != events.TriggerPush {
					continue
				}
				if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == g.carrier {
					fired++
				}
			}
			return nil
		}
		pol := Policy(RandomPolicy(seed))
		if g.attack {
			pol = combatCastPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			divs = append(divs, fmt.Sprintf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail))
		}
	}
	t.Logf("%s's trigger fired %d times in %d games", g.carrier, fired, g.seeds)
	return fired, divs
}

// refsLockstep is refsPlay that fails on any divergence or too few firings.
func refsLockstep(t *testing.T, reg *cards.Registry, corpus string, g refsGame) {
	t.Helper()
	fired, divs := refsPlay(t, reg, corpus, g)
	for _, d := range divs {
		t.Error(d)
	}
	if fired < g.fires {
		t.Errorf("%s's trigger fired %d times in %d games, want at least %d", g.carrier, fired, g.seeds, g.fires)
	}
}

func TestMBCTriggerReferents10Lowers(t *testing.T) {
	t.Parallel()
	corpus := corpusDirOrSkip(t)
	reg := synthReg(t)
	games := []refsGame{
		{
			carrier: "Mr. Foxglove", seeds: 24, fires: 3, attack: true,
			decks: [2][]string{
				refsDeck(map[string]int{"Mr. Foxglove": 10, "Forest": 7, "Plains": 7, "Island": 7}, "Forest"),
				refsDeck(map[string]int{"Grizzly Bears": 8, "Forest": 14}, "Forest"),
			},
		},
		{
			carrier: "Guerrilla Tactics", seeds: 24, fires: 3,
			decks: [2][]string{
				refsDeck(map[string]int{"Guerrilla Tactics": 12, "Mountain": 16}, "Mountain"),
				refsDeck(map[string]int{"Mind Rot": 12, "Swamp": 16}, "Swamp"),
			},
		},
		{
			carrier: "Anowon, the Ruin Thief", seeds: 24, fires: 3, attack: true,
			decks: [2][]string{
				refsDeck(map[string]int{"Anowon, the Ruin Thief": 12, "Island": 8, "Swamp": 8}, "Swamp"),
				refsDeck(map[string]int{"Grizzly Bears": 4, "Forest": 14}, "Forest"),
			},
		},
	}
	for _, g := range games {
		t.Run(g.carrier, func(t *testing.T) {
			refsLockstep(t, reg, corpus, g)
		})
	}
}

// selvalaProbe is Selvala, Heart of the Wilds' trigger without its mana
// ability (the mana walk cannot activate "{G}, {T}: add X mana in any
// combination of colors", so the full card stays refused): whenever another
// creature enters, its controller may draw a card if its power is greater
// than each other creature's power.
const selvalaProbe = "Name:Wilds Draw Probe\nManaCost:1 G G\nTypes:Creature Elf Scout\nPT:2/3\n" +
	"T:Mode$ ChangesZone | Destination$ Battlefield | Origin$ Any | TriggerZones$ Battlefield | ValidCard$ Creature.Other | Execute$ ProbeDraw | TriggerDescription$ Probe: another creature enters; its controller may draw if it has the greatest power.\n" +
	"SVar:ProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ TriggeredCardController | OptionalDecider$ TriggeredCardController | ConditionCheckSVar$ EnteredPower | ConditionSVarCompare$ EQBestPower\n" +
	"SVar:EnteredPower:TriggeredCard$Valid Card.greatestPower\n" +
	"SVar:BestPower:Count$Valid Card.greatestPower\n" +
	"Oracle:Probe.\n"

// probeCorpus is a corpus directory holding the probe script and the real
// cards the decks need (copied from the shared corpus into a temp dir, never
// committed), so both sides read the probe from one script.
func probeCorpus(t *testing.T, real []string, probes ...string) string {
	t.Helper()
	src := corpusDirOrSkip(t)
	dir := t.TempDir()
	put := func(name, text string) {
		file := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return '_'
		}, name) + ".txt"
		d := filepath.Join(dir, "cardsfolder", file[:1])
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, file), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range real {
		file := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return '_'
		}, n) + ".txt"
		b, err := os.ReadFile(filepath.Join(src, "cardsfolder", file[:1], file))
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		put(n, string(b))
	}
	for _, p := range probes {
		put(strings.TrimPrefix(strings.SplitN(p, "\n", 2)[0], "Name:"), p)
	}
	if err := os.CopyFS(filepath.Join(dir, "tokenscripts"), os.DirFS(filepath.Join(src, "tokenscripts"))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cards.lock"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func selvalaProbeGame() refsGame {
	return refsGame{
		carrier: "Wilds Draw Probe", seeds: 24, fires: 10,
		decks: [2][]string{
			refsDeck(map[string]int{"Wilds Draw Probe": 8, "Grizzly Bears": 8, "Forest": 16}, "Forest"),
			refsDeck(map[string]int{"Grizzly Bears": 12, "Hill Giant": 4, "Forest": 10, "Mountain": 8}, "Forest"),
		},
	}
}

// TestMBCTriggerReferents10LowersDeciderProbe: the OptionalDecider$
// TriggeredCardController Draw plays identically, and the entering
// creature's controller, not the probe's, is the one asked.
func TestMBCTriggerReferents10LowersDeciderProbe(t *testing.T) {
	g := selvalaProbeGame()
	dir := probeCorpus(t, []string{"Grizzly Bears", "Hill Giant", "Forest", "Mountain"}, selvalaProbe)
	reg, err := cards.OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	refsLockstep(t, reg, dir, g)

	// the mutation: the probe's controller answers every ask, which the
	// opponent's entering creatures show as a divergence
	defer func(old bool) { dirac.DebugMayDeciderCtrl = old }(dirac.DebugMayDeciderCtrl)
	dirac.DebugMayDeciderCtrl = true
	if _, divs := refsPlay(t, reg, dir, g); len(divs) == 0 {
		t.Errorf("no divergence with the probe's controller answering every ask: the games never ask the opponent")
	}
}
