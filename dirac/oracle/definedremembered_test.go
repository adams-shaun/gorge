package oracle

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: four Defined$ heads.
//
//   - Defined$ RememberedCard: Valgavoth's Onslaught (X counters on each
//     manifested creature) and the Rooms' manifest-dread doors (Weight Room,
//     Slimy Aquarium);
//   - a one-opponent ChoosePlayer then Player.Chosen: Discerning Financier
//     gives a Treasure (Prosperous Innkeeper's) to the one opponent. Gorge
//     answers the one-option choice in place, so the kernel poses nothing;
//   - Defined$ TriggeredCardLKICopy.Spider: Aunt May counters Spiders only;
//   - Defined$ TriggeredCardLKICopy & Self: Sumala Sentry counters the
//     turned-up permanent and herself (Vengeful Creeper and the manifested
//     creatures turn face up).
var kdefheads5Decks = [2][]cardset.Entry{
	{
		{Name: "Valgavoth's Onslaught", Count: 3}, {Name: "Moldering Gym", Count: 2}, {Name: "Sumala Sentry", Count: 3},
		{Name: "Aunt May", Count: 3}, {Name: "Vengeful Creeper", Count: 3}, {Name: "Giant Spider", Count: 2},
		{Name: "Woolly Spider", Count: 2}, {Name: "Mammoth Spider", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Elite Vanguard", Count: 2}, {Name: "Forest", Count: 9}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Underwater Tunnel", Count: 3}, {Name: "Discerning Financier", Count: 4}, {Name: "Prosperous Innkeeper", Count: 4},
		{Name: "Aunt May", Count: 2}, {Name: "Sumala Sentry", Count: 2}, {Name: "Vengeful Creeper", Count: 3},
		{Name: "Giant Spider", Count: 3}, {Name: "Mammoth Spider", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Elite Vanguard", Count: 2}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 4},
	},
}

// kdhTally counts, from gorge's event log, what the pool reached.
type kdhTally struct {
	evAt  int
	stack map[uint32]string // stack object -> the spell's name
	xPos  map[uint32]bool   // a stack object cast with X >= 1
	// Valgavoth's Onslaught resolutions with X >= 1, Room doors unlocked,
	// Discerning Financier activations, Aunt May triggers (the entering
	// creature a Spider / not), Sumala Sentry triggers, face-up turns
	onslX, unlocks, fin, amSpider, amOther, sumala, faceUp int
}

func (p *kdhTally) reset() {
	p.evAt = 0
	p.stack = map[uint32]string{}
	p.xPos = map[uint32]bool{}
}

func (p *kdhTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.PutOnStack:
			p.stack[uint32(e.Obj)] = chName(g, e.Obj)
		case events.CastInfo:
			if e.Amount >= 1 {
				p.xPos[uint32(e.Obj)] = true
			}
		case events.Resolve:
			if p.stack[uint32(e.Obj)] == "Valgavoth's Onslaught" && p.xPos[uint32(e.Obj)] {
				p.onslX++
			}
			delete(p.stack, uint32(e.Obj))
			delete(p.xPos, uint32(e.Obj))
		case events.DoorUnlock:
			p.unlocks++
		case events.TurnFaceUp:
			p.faceUp++
		case events.AbilityPush:
			if chName(g, e.Obj) == "Discerning Financier" {
				p.fin++
			}
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Aunt May":
				spider := false
				if len(e.IDs) > 0 {
					spider = strings.HasSuffix(chName(g, e.IDs[0]), "Spider")
				}
				if spider {
					p.amSpider++
				} else {
					p.amOther++
				}
			case "Sumala Sentry":
				p.sumala++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// kdhRun plays seeds 1..seeds; stopAtDiv ends it at the first divergence and
// mutate edits the generated table before the games.
func kdhRun(t *testing.T, seeds uint64, stopAtDiv bool, mutate func(*kop.Table)) (tally *kdhTally, games, divs, stalls int, first string) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("definedremembered")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdefheads5Decks {
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
	tbl, _, _, err := set.Matchup(kdefheads5Decks[0], kdefheads5Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	tally = &kdhTally{}
	for seed := uint64(1); seed <= seeds; seed++ {
		// PassElision: gorge's one-option ChoosePlayer is answered in place,
		// as the kernel (which poses no ask for it) elides it
		cfg := Config{GorgeAutopaySurface: seed%4 == 0, PassElision: true}
		m, err := NewMatchup(reg, [2]string{"kdh-a", "kdh-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.reset()
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tally.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if first == "" {
				first = "seed " + strconv.FormatUint(seed, 10) + ": " + d.What + " " + d.Field
			}
			lo := max(d.At-60, 0)
			t.Logf("seed %d %s %s at %d last actions %v\ndiff %v %s\nref %v\nker %v", seed, d.What, d.Field, d.At, actionStrings(res.Actions[max(lo, d.At-8):d.At]), d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			if stopAtDiv {
				return
			}
		}
	}
	return
}

func TestDefinedHeadsRememberedCardLockstep(t *testing.T) {
	t.Parallel()
	tally, games, divs, stalls, first := kdhRun(t, 300, false, nil)
	t.Logf("%d games, %d divergences, %d stalls; Onslaught resolutions X>=1 %d; Room unlocks %d; Financier activations %d; Aunt May triggers: Spider %d, other %d; Sumala triggers %d; face-up turns %d",
		games, divs, stalls, tally.onslX, tally.unlocks, tally.fin, tally.amSpider, tally.amOther, tally.sumala, tally.faceUp)
	if divs != 0 {
		t.Fatalf("%d divergences, first %s", divs, first)
	}
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"Onslaught resolutions with X >= 1", tally.onslX, 10},
		{"Room unlocks", tally.unlocks, 5},
		{"Financier activations", tally.fin, 10},
		{"Aunt May triggers", tally.amSpider + tally.amOther, 20},
		{"Aunt May triggers on a Spider", tally.amSpider, 1},
		{"Aunt May triggers on a non-Spider", tally.amOther, 1},
		{"Sumala triggers", tally.sumala, 3},
	} {
		if c.got < c.want {
			t.Errorf("%s: %d, want at least %d", c.name, c.got, c.want)
		}
	}
}

// TestDefinedHeadsRememberedCardMutation: with every generated SkipIf condition on a ref
// matching a filter inverted (the table twin of a kernel switch, so the hot
// path stays untouched), Aunt May counters the non-Spiders and the lockstep
// must diverge within 128 seeds.
func TestDefinedHeadsRememberedCardMutation(t *testing.T) {
	t.Parallel()
	_, games, divs, _, first := kdhRun(t, 128, true, func(tbl *kop.Table) {
		// the table shares the pool's condition slice with the
		// lockstep test running beside this one
		tbl.Conds = slices.Clone(tbl.Conds)
		for i := range tbl.Conds {
			if tbl.Conds[i].Kind == kop.CondRefMatches {
				tbl.Conds[i].Neg = !tbl.Conds[i].Neg
			}
		}
	})
	if divs == 0 {
		t.Fatalf("no divergence in %d games with every RefMatches condition inverted", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, first)
}
