package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Saga lockstep (CR 714, saga): constructed matchups over the generated
// saga pool. The Sagas have chapter triggers on the lore counter (simple
// token/creature effects, a targeted creature destroy, a discard, an
// optional +1/+1 and a sweeper); Summon: Shiva is a Saga CREATURE (it
// attacks, blocks and dies like any other creature while its chapters
// trigger), a proliferate spell adds one lore off-turn and Disperse bounces
// a Saga to reset its counters. The check counts, from gorge's state, every
// chapter the lore count crossed (by number, per card and per controller),
// every Saga sacrificed and every departure, and asserts that a Saga that
// left the battlefield was recast and crossed chapter 1 again on its new
// object.
//
// Proliferate adds ONE lore counter, so the pool has no multi-chapter
// crossing (multi stays 0); the amt >= 2 branch of chapterCrossed is covered
// directly by dirac/kw_saga_test.go#TestSagaMultiCross.

var ksagaDecks = [][2][]cardset.Entry{{
	{
		{Name: "History of Benalia", Count: 3}, {Name: "The Birth of Meletis", Count: 3},
		{Name: "Elspeth's Nightmare", Count: 3}, {Name: "Phyrexian Scriptures", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Vampire Cutthroat", Count: 3}, {Name: "Giant Spider", Count: 3},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Volt Charge", Count: 4}, {Name: "Vivisurgeon's Insight", Count: 3}, {Name: "Disperse", Count: 4},
		{Name: "Summon: Shiva", Count: 3}, {Name: "Air Elemental", Count: 2}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Walking Corpse", Count: 3}, {Name: "Island", Count: 9},
		{Name: "Mountain", Count: 5},
	},
}, {
	{
		{Name: "History of Benalia", Count: 4}, {Name: "The Birth of Meletis", Count: 4},
		{Name: "Elspeth's Nightmare", Count: 3}, {Name: "Phyrexian Scriptures", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 4},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Volt Charge", Count: 4}, {Name: "Vivisurgeon's Insight", Count: 4}, {Name: "Disperse", Count: 3},
		{Name: "Summon: Shiva", Count: 3}, {Name: "Air Elemental", Count: 3}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Walking Corpse", Count: 3}, {Name: "Island", Count: 9},
		{Name: "Mountain", Count: 4},
	},
}}

var ksagaCarriers = []string{
	"History of Benalia", "The Birth of Meletis", "Elspeth's Nightmare", "Phyrexian Scriptures",
	"Summon: Shiva", "Volt Charge", "Vivisurgeon's Insight", "Disperse",
}

// ksagaSagas is every Saga in the pool, in a fixed order for the per-card
// assertions (a map range would make the failure text order vary).
var ksagaSagas = []string{
	"History of Benalia", "The Birth of Meletis", "Elspeth's Nightmare",
	"Phyrexian Scriptures", "Summon: Shiva",
}

// ksagaSagaFinal is every Saga's final chapter in the pool.
var ksagaSagaFinal = map[string]int{
	"History of Benalia": 3, "The Birth of Meletis": 3,
	"Elspeth's Nightmare": 3, "Phyrexian Scriptures": 3, "Summon: Shiva": 3,
}

type ksagaCounter struct {
	chapters [4]int // chapter number reached, by the lore crossing
	byName   map[string]*[4]int
	byCtrl   [2][4]int // chapter crossings by the Saga's controller
	lore     map[state.ObjID]int
	seen     map[state.ObjID]bool
	// leftNames remembers every card name that left the battlefield
	// without being sacrificed (a Disperse bounce, or a creature dying);
	// a chapter-1 crossing on a NEW object of that name is a recast.
	leftNames map[string]bool
	sac       int
	sacByName map[string]int
	left      int
	recast    int
	offTurn   int // lore added while its controller was not the active player
	multi     int // one event crossed two or more chapters
}

func newKsagaCounter() *ksagaCounter {
	return &ksagaCounter{
		byName:    map[string]*[4]int{},
		lore:      map[state.ObjID]int{},
		seen:      map[state.ObjID]bool{},
		leftNames: map[string]bool{},
		sacByName: map[string]int{},
	}
}

func loreOf(o *state.Object) int {
	for _, c := range o.Counters {
		if c.Kind == "LORE" {
			return int(c.N)
		}
	}
	return 0
}

func (c *ksagaCounter) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	onBF := map[state.ObjID]bool{}
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			onBF[id] = true
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			name := o.Face().Name
			if _, ok := ksagaSagaFinal[name]; !ok {
				continue
			}
			n := loreOf(o)
			prev := c.lore[id]
			if n > prev {
				if n-prev >= 2 {
					c.multi++
				}
				if g.Active != o.Controller {
					c.offTurn++
				}
				for ch := prev + 1; ch <= n && ch < len(c.chapters); ch++ {
					c.chapters[ch]++
					c.byCtrl[o.Controller][ch]++
					if c.byName[name] == nil {
						c.byName[name] = new([4]int)
					}
					c.byName[name][ch]++
					if ch == 1 && c.leftNames[name] && !c.seen[id] {
						c.recast++
					}
				}
			}
			c.lore[id] = n
			c.seen[id] = true
		}
	}
	for id := range c.seen {
		if onBF[id] {
			continue
		}
		o := g.Obj(id)
		name := ""
		if o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		// counters are removed as the object leaves the battlefield, so the
		// last count seen there is the one the SBA read
		if o != nil && o.Zone == state.ZGraveyard && name != "" && c.lore[id] >= ksagaSagaFinal[name] {
			c.sac++
			c.sacByName[name]++
		} else {
			c.left++
			if name != "" {
				c.leftNames[name] = true
			}
		}
		delete(c.seen, id)
	}
	return nil
}

func TestSagaLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("saga")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	var chapters [4]int
	var byCtrl [2][4]int
	byName := map[string]*[4]int{}
	sacByName := map[string]int{}
	sac, left, multi, recast, offTurn := 0, 0, 0, 0, 0
	for mi, md := range ksagaDecks {
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
		for seed := uint64(1); seed <= 32; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ksaga-a", "ksaga-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			cnt := newKsagaCounter()
			pol := castActivatePlayedPolicy(seed, played)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: cnt.check})
			CloseSide(k)
			games++
			for i := range chapters {
				chapters[i] += cnt.chapters[i]
			}
			for p := range byCtrl {
				for ch := range byCtrl[p] {
					byCtrl[p][ch] += cnt.byCtrl[p][ch]
				}
			}
			for n, v := range cnt.byName {
				if byName[n] == nil {
					byName[n] = new([4]int)
				}
				for ch := range v {
					byName[n][ch] += v[ch]
				}
			}
			for n, v := range cnt.sacByName {
				sacByName[n] += v
			}
			sac += cnt.sac
			left += cnt.left
			multi += cnt.multi
			recast += cnt.recast
			offTurn += cnt.offTurn
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, chapter crossings %v (by controller %v), sacrifices %d %v, left %d, recast %d, off-turn lore %d, multi-crossings %d, carrier plays %v",
		games, chapters[1:], byCtrl, sac, sacByName, left, recast, offTurn, multi, played)
	for _, n := range ksagaCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
	for ch := 1; ch <= 3; ch++ {
		if chapters[ch] < 20 {
			t.Errorf("chapter %d crossed %d times, want at least 20", ch, chapters[ch])
		}
	}
	if sac < 20 {
		t.Errorf("Sagas sacrificed: %d, want at least 20", sac)
	}
	for _, n := range ksagaSagas {
		v := byName[n]
		if v == nil {
			t.Errorf("%s never crossed a chapter", n)
			continue
		}
		for ch := 1; ch <= 3; ch++ {
			if v[ch] == 0 {
				t.Errorf("%s never crossed chapter %d", n, ch)
			}
		}
		if sacByName[n] == 0 {
			t.Errorf("%s was never sacrificed at its final chapter", n)
		}
	}
	// Summon: Shiva is the Saga CREATURE, and the only Saga player 1
	// controls: all three chapters must cross for that seat too.
	if byCtrl[1][1] == 0 || byCtrl[1][2] == 0 || byCtrl[1][3] == 0 {
		t.Errorf("player 1 never crossed all three Saga chapters: %v", byCtrl[1][1:])
	}
	if recast == 0 {
		t.Errorf("no Saga that left the battlefield was recast and crossed chapter 1 on its new object")
	}
	if offTurn == 0 {
		t.Errorf("no lore counter was added off-turn (proliferate)")
	}

}

// TestSagaMutation: with the chapter-crossing check switched off
// (dirac.DebugIgnoreChapterCross) the same games diverge.
func TestSagaMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("saga")
	if err != nil {
		t.Fatal(err)
	}
	md := ksagaDecks[0]
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
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugIgnoreChapterCross = true
	defer func() { dirac.DebugIgnoreChapterCross = false }()
	for seed := uint64(1); seed <= 8; seed++ {
		cfg := Config{}
		m, err := NewMatchup(reg, [2]string{"ksaga-a", "ksaga-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d: the mutation diverges at decision %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("ignoring chapter crossings diverged in no game: the lockstep does not see the chapter trigger")
}
