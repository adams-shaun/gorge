package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The lockstep: a reflexive trigger's TriggerRemembered$
// value, captured as the "when you do" trigger is put on the stack.
//
//   - Ill-Timed Explosion: X damage to each creature, X the highest mana value
//     among the cards its Discard<2/Card> unless-cost discarded
//     (GreatestCardManaCost);
//   - The Ancient One: the targeted player mills cards equal to the discarded
//     card's mana value (CardManaCost);
//   - Miasma Demon: up to that many targets get -2/-2, that many the cards it
//     discarded (Amount).
//
// Cards of mana value 1..6 beside them vary the discarded value.
var ktrigremDecks = [2][]cardset.Entry{
	{
		{Name: "Ill-Timed Explosion", Count: 4}, {Name: "Air Elemental", Count: 2}, {Name: "Hill Giant", Count: 3},
		{Name: "Mahamoti Djinn", Count: 1}, {Name: "Lightning Bolt", Count: 2}, {Name: "Wind Drake", Count: 3},
		{Name: "Merfolk of the Pearl Trident", Count: 3}, {Name: "Raging Goblin", Count: 3}, {Name: "Goblin Piker", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "The Ancient One", Count: 4}, {Name: "Miasma Demon", Count: 3}, {Name: "Vampire Nighthawk", Count: 3},
		{Name: "Merfolk of the Pearl Trident", Count: 3}, {Name: "Wind Drake", Count: 3}, {Name: "Phantom Warrior", Count: 2},
		{Name: "Air Elemental", Count: 2}, {Name: "Mahamoti Djinn", Count: 1}, {Name: "Unsummon", Count: 2},
		{Name: "Divination", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 7},
	},
}

// ktrTally counts, from gorge's event log, the "when you do" resolutions the
// pool reached and the captured value each carried.
type ktrTally struct {
	evAt int
	// Ill-Timed Explosion's reflexive resolutions that dealt damage, and the
	// damage X of each (the DamageProvenance run of one resolution); The
	// Ancient One's mills, and the cards each milled
	boom, mills int
	boomX       map[int]int
	millN       map[int]int
	inBoom      bool
	curMill     int
}

func (p *ktrTally) reset() {
	p.flush()
	p.evAt, p.inBoom = 0, false
}

// flush closes the mill run of the resolution that just ended.
func (p *ktrTally) flush() {
	if p.curMill > 0 {
		p.mills++
		p.millN[p.curMill]++
		p.curMill = 0
	}
}

func (p *ktrTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if events.IsMill(*e) {
			p.curMill++
			continue
		}
		switch e.Kind {
		case events.Resolve:
			p.flush()
			p.inBoom = false
		case events.DamageProvenance:
			if !p.inBoom && chName(g, e.Obj) == "Ill-Timed Explosion" {
				p.inBoom = true
				p.boom++
				p.boomX[int(e.Amount)]++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

func ktrSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigrem")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktrigremDecks {
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
	return reg, set, decks
}

func ktrRun(t *testing.T, seeds uint64, stopAtDiv bool) (tally *ktrTally, games, divs, stalls int, first string) {
	reg, set, decks := ktrSetup(t)
	tbl, _, _, err := set.Matchup(ktrigremDecks[0], ktrigremDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally = &ktrTally{boomX: map[int]int{}, millN: map[int]int{}}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"ktr-a", "ktr-b"}, decks, 1)
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

func TestTrigremLockstep(t *testing.T) {
	t.Parallel()
	tally, games, divs, stalls, first := ktrRun(t, 300, false)
	tally.flush()
	t.Logf("%d games, %d divergences, %d stalls; Ill-Timed Explosion \"when you do\" resolutions with damage %d (X -> count %v); Ancient One mills %d (cards -> count %v)",
		games, divs, stalls, tally.boom, tally.boomX, tally.mills, tally.millN)
	if divs != 0 {
		t.Fatalf("%d divergences, first %s", divs, first)
	}
	if tally.boom < 15 || len(tally.boomX) < 3 {
		t.Errorf("Ill-Timed Explosion reached %d resolutions with %d distinct X, want >= 15 and >= 3", tally.boom, len(tally.boomX))
	}
	if tally.mills < 15 || len(tally.millN) < 3 {
		t.Errorf("The Ancient One reached %d mills with %d distinct amounts, want >= 15 and >= 3", tally.mills, len(tally.millN))
	}
}

// TestTrigremMutation proves the pool exercises the captured value: with the
// kernel's reflexive trigger carrying 0 instead of B, some seed diverges.
func TestTrigremMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugReflexAmtZero = true
	defer func() { dirac.DebugReflexAmtZero = false }()
	if _, _, divs, _, _ := ktrRun(t, 128, true); divs > 0 {
		return
	}
	t.Fatal("no seed diverged with the reflexive trigger amount dropped: the pool does not exercise it")
}
