package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two Star Trek lockstep pools:
//
//   - borgexile: Assimilation Assessor and Borg Cube choose a creature card
//     exiled with them and assimilate it (a ChooseCard among the exile cards
//     reading the source link, CR 607.2a, then Assimilate Defined$
//     ChosenCard). Both seats run Assessors, so each exile holds cards the
//     other source exiled; Unsummon makes a source a new object (CR 400.7).
//   - guinanrem: Guinan, Ancient Advisor draws when its PutCounterAll landed
//     two or more counters (RememberAmount$, Count$RememberedNumber); Hardened
//     Scales lands two on one creature.
var (
	borgexileDecks = [][2][]cardset.Entry{
		{
			{
				{Name: "Assimilation Assessor", Count: 6}, {Name: "Borg Cube", Count: 3}, {Name: "Ornithopter", Count: 4}, {Name: "Memnite", Count: 4},
				{Name: "Phyrexian Walker", Count: 2}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2}, {Name: "Murder", Count: 4},
				{Name: "Swamp", Count: 9}, {Name: "Plains", Count: 4},
			},
			{
				{Name: "Assimilation Assessor", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Murder", Count: 2},
				{Name: "Swamp", Count: 7}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 5},
			},
		},
		{
			{
				{Name: "Assimilation Assessor", Count: 6}, {Name: "Borg Cube", Count: 3}, {Name: "Ornithopter", Count: 4}, {Name: "Memnite", Count: 4},
				{Name: "Phyrexian Walker", Count: 2}, {Name: "Sol Ring", Count: 2}, {Name: "Mind Stone", Count: 2}, {Name: "Unsummon", Count: 4},
				{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 2}, {Name: "Island", Count: 3},
			},
			{
				{Name: "Assimilation Assessor", Count: 4}, {Name: "Savannah Lions", Count: 8}, {Name: "Grizzly Bears", Count: 4}, {Name: "Murder", Count: 6},
				{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 4},
			},
		},
	}
	guinanremDecks = [][2][]cardset.Entry{
		{
			{
				{Name: "Guinan, Ancient Advisor", Count: 8}, {Name: "Hardened Scales", Count: 4}, {Name: "Grizzly Bears", Count: 4},
				{Name: "Savannah Lions", Count: 6}, {Name: "Llanowar Elves", Count: 4}, {Name: "Raise the Alarm", Count: 4},
				{Name: "Plains", Count: 5}, {Name: "Forest", Count: 5},
			},
			{
				{Name: "Guinan, Ancient Advisor", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Savannah Lions", Count: 8}, {Name: "Llanowar Elves", Count: 4},
				{Name: "Mountain", Count: 5}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 4},
			},
		},
	}
)

// trekTally counts what each pool exercised, from gorge's event log.
type trekTally struct {
	evAt int
	// borgexile: a graveyard exiled by an Assessor's enters trigger, a
	// creature exiled by Borg Cube, an assimilated card
	graveExile, cubeExile, assimilated int
	// guinanrem: counters put on a creature, two or more at once
	puts, puts2 int
}

func (p *trekTally) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.CounterChange:
			if e.Counter == "P1P1" && e.Amount > 0 {
				p.puts++
				if e.Amount >= 2 {
					p.puts2++
				}
			}
		case events.MoveZone:
			switch {
			case e.From == state.ZBattlefield && e.To == state.ZExile:
				p.cubeExile++ // the pool's only battlefield exile is Borg Cube's
			case e.From == state.ZExile && e.To == state.ZBattlefield:
				p.assimilated++
			case e.From == state.ZGraveyard && e.To == state.ZExile:
				p.graveExile++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// trekRun plays pool's matchups, each both ways round, for seeds seeds each.
// With mutated set the first divergence ends the run instead of failing.
func trekRun(t *testing.T, pool string, decks [][2][]cardset.Entry, seeds int, mutated bool) (p *trekTally, games int, diverged bool) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(pool)
	if err != nil {
		t.Fatal(err)
	}
	p = &trekTally{}
	var matchups [][2][]cardset.Entry
	for _, d := range decks {
		matchups = append(matchups, d, [2][]cardset.Entry{d[1], d[0]})
	}
	for mi, md := range matchups {
		var dk [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					dk[s] = append(dk[s], c)
				}
			}
			if len(dk[s]) != 40 {
				t.Fatalf("%s matchup %d deck %d: %d cards", pool, mi, s, len(dk[s]))
			}
		}
		var tbl *kop.Table
		if tbl, _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{pool + "-a", pool + "-b"}, dk, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := fbPolicy(seed) // plays at random among its plays
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			p.evAt = 0
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: p.check})
			CloseSide(k)
			games++
			if mutated {
				if res.Div != nil {
					return p, games, true
				}
				continue
			}
			if res.Stall != "" {
				t.Errorf("%s matchup %d seed %d: stalled: %s", pool, mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("%s matchup %d seed %d last actions %v", pool, mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("%s matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", pool, mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return p, games, false
}

func atLeast(t *testing.T, what string, n, want int) {
	t.Helper()
	if n < want {
		t.Errorf("%d %s, want at least %d", n, what, want)
	}
}

func TestBorgExileLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	p, games, _ := trekRun(t, "borgexile", borgexileDecks, 40, false)
	t.Logf("%d games: graveyards exiled %d, Borg Cube exiled %d, cards assimilated from exile %d", games, p.graveExile, p.cubeExile, p.assimilated)
	atLeast(t, "graveyard exiles", p.graveExile, 20)
	atLeast(t, "Borg Cube exiles", p.cubeExile, 3)
	atLeast(t, "assimilations from exile", p.assimilated, 10)
}

// TestBorgExileMutation: with every exiled card read as exiled with the
// reading source, an Assessor's choice offers the cards the other seat's
// Assessor exiled, which gorge's exiled-with link does not.
func TestBorgExileMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugExwAnySource = true
	defer func() { dirac.DebugExwAnySource = false }()
	if _, _, diverged := trekRun(t, "borgexile", borgexileDecks, 64, true); !diverged {
		t.Fatal("no divergence in 64 seeds with the exiled-with link ignored")
	}
}

func TestGuinanRememberAmountLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	p, games, _ := trekRun(t, "guinanrem", guinanremDecks, 100, false)
	t.Logf("%d games: +1/+1 counters put %d, two or more at once %d", games, p.puts, p.puts2)
	atLeast(t, "+1/+1 counter puts", p.puts, 30)
	atLeast(t, "puts of two or more", p.puts2, 5)
}

// TestGuinanRememberAmountMutation: with the requested count remembered in
// place of the landed one, a sweep Hardened Scales grew to two no longer
// draws.
func TestGuinanRememberAmountMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRem2Always = true
	defer func() { dirac.DebugRem2Always = false }()
	if _, _, diverged := trekRun(t, "guinanrem", guinanremDecks, 128, true); !diverged {
		t.Fatal("no divergence in 128 seeds with the requested count remembered")
	}
}
