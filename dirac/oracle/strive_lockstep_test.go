package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Strive (CR 702.52a): "this spell costs <cost> more to cast for each target
// beyond the first". kgen lowers K:Strive:<plain mana> to the card's own
// StRaisePerTarget raise (lower_strive.go); gorge folds the charge into the
// cast's cost after the target answer (rules foldStriveCost). Both decks
// carry several Strive spells beside cheap creatures, so casts reach one, two
// and three or more targets; the expected answer at every decision is gorge's.

var kstriveDecks = [2][]cardset.Entry{
	{
		{Name: "Ajani's Presence", Count: 3}, {Name: "Rouse the Mob", Count: 3}, {Name: "Launch the Fleet", Count: 3},
		{Name: "Twinflame", Count: 3}, {Name: "Harness by Force", Count: 2},
		{Name: "Savannah Lions", Count: 8}, {Name: "Raging Goblin", Count: 6},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Nature's Panoply", Count: 3}, {Name: "Colossal Heroics", Count: 3}, {Name: "Hour of Need", Count: 3},
		{Name: "Blinding Flare", Count: 3},
		{Name: "Grizzly Bears", Count: 10}, {Name: "Raging Goblin", Count: 4},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
	},
}

var kstriveCards = map[string]bool{
	"Ajani's Presence": true, "Rouse the Mob": true, "Launch the Fleet": true, "Twinflame": true,
	"Harness by Force": true, "Nature's Panoply": true, "Colossal Heroics": true, "Hour of Need": true,
	"Blinding Flare": true,
}

// kstriveStats counts, from the gorge event log, the Strive casts that
// reached the stack by their number of targets.
type kstriveStats struct{ one, two, more int }

type kstriveTally struct {
	evAt int
	pend []state.ObjID // spells on the stack whose targets are not chosen yet
}

func (p *kstriveTally) scan(ref *GorgeSide, st *kstriveStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt, p.pend = 0, p.pend[:0]
	}
	for j := p.evAt; j < len(evs); j++ {
		if e := &evs[j]; e.Kind == events.PutOnStack {
			p.pend = append(p.pend, e.Obj)
		}
	}
	p.evAt = len(evs)
	keep := p.pend[:0]
	for _, id := range p.pend {
		o := g.Obj(id)
		if o == nil || o.Zone != state.ZStack {
			continue
		}
		if len(o.Targets) == 0 {
			keep = append(keep, id)
			continue
		}
		if kstriveCards[chName(g, id)] {
			switch n := len(o.Targets); {
			case n == 1:
				st.one++
			case n == 2:
				st.two++
			default:
				st.more++
			}
		}
	}
	p.pend = keep
}

func kstrivePolicy(seed uint64, st *kstriveStats) Policy {
	pol := castFirstPolicy(seed)
	var tl kstriveTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && st != nil {
			tl.scan(c.G, st)
		}
		return pol.Pick(c)
	})
}

// kstriveRun plays seeds lo..hi of both seatings, gorge leading. With report
// false it stops at the first divergence. mut edits the kernel table.
func kstriveRun(t *testing.T, lo, hi uint64, report bool, mut func(*kop.Table), st *kstriveStats) (games, divs, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("strive")
	if err != nil {
		t.Fatal(err)
	}
	for swap := 0; swap < 2; swap++ {
		md := kstriveDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kstriveDecks[1], kstriveDecks[0]}
		}
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
				t.Fatalf("swap %d deck %d: %d cards", swap, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mut != nil {
			mut(tbl)
		}
		for seed := lo; seed <= hi; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kstrive-a", "kstrive-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kstrivePolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if report {
					t.Errorf("swap %d seed %d: kernel stall %q", swap, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !report {
					return games, divs, stalls
				}
				t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls
}

// TestStriveLockstep: no divergence or stall, and Strive casts with one, two
// and three or more targets each happen.
func TestStriveLockstep(t *testing.T) {
	t.Parallel()
	var st kstriveStats
	games, divs, stalls := kstriveRun(t, 1, 100, true, nil, &st)
	t.Logf("%d games, %d divergences, %d stalls, %+v", games, divs, stalls, st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.one < 1 || st.two < 1 || st.more < 1 {
		t.Errorf("Strive casts with one target: %d, two: %d, three or more: %d (want >= 1 each)", st.one, st.two, st.more)
	}
}

// TestStriveLockstepMutation: with the per-target charge zeroed in the
// kernel table some seed diverges from gorge, proof the pool exercises the
// extra-target price.
func TestStriveLockstepMutation(t *testing.T) {
	t.Parallel()
	var st kstriveStats
	_, divs, _ := kstriveRun(t, 1, 100, false, func(tbl *kop.Table) {
		cs := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cs {
			abs := append([]kop.Ability(nil), cs[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatIncreaseCost && abs[j].Static.StFlags&kop.StRaisePerTarget != 0 {
					abs[j].Static.A, abs[j].Static.B = 0, 0
				}
			}
			cs[i].Abilities = abs
		}
		tbl.Cards = cs
	}, &st)
	if divs == 0 {
		t.Fatalf("no seed diverged with the Strive charge zeroed (%+v)", st)
	}
	if st.one < 1 || st.two < 1 || st.more < 1 {
		t.Logf("mutation run reached one %d, two %d, more %d", st.one, st.two, st.more)
	}
}
