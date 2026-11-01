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

// The targeted-prepare lockstep (CR 722.3a-c): a grant or
// an unprepare that names ANOTHER permanent. The decks carry
//
//   - Biblioplex Tomekeeper's ETB charm (up to one: target creature becomes
//     prepared / unprepared), Skycoach Waypoint ({3},{T}: target creature
//     becomes prepared) and Infinite Coursework (its ETB unprepares the
//     enchanted creature);
//   - preparation creatures (a targeted grant mints the exile copy) and
//     plain creatures (gorge skips the grant: the no-op);
//   - the copy cast from exile after a targeted grant, and a copy unlinked
//     by an unprepare that ceases.
//
// Counters: the kernel's PrepareStats for what its ops saw, gorge's event log
// for grants, unprepares and casts. The mutation keeps the copy castable.
var preptargetDecks = [2][]cardset.Entry{
	{
		{Name: "Biblioplex Tomekeeper", Count: 3}, {Name: "Infinite Coursework", Count: 3},
		{Name: "Abigale, Poet Laureate", Count: 2}, {Name: "Emeritus of Conflict", Count: 2},
		{Name: "Emeritus of Woe", Count: 2}, {Name: "Honorbound Page", Count: 2},
		{Name: "Leech Collector", Count: 2}, {Name: "Infirmary Healer", Count: 2},
		{Name: "Elite Interceptor", Count: 2}, {Name: "Gray Ogre", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Bake into a Pie", Count: 1},
		{Name: "Skycoach Waypoint", Count: 3}, {Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4},
		{Name: "Mountain", Count: 2}, {Name: "Island", Count: 2},
	},
	{
		{Name: "Biblioplex Tomekeeper", Count: 2}, {Name: "Infinite Coursework", Count: 2},
		{Name: "Tam, Observant Sequencer", Count: 2}, {Name: "Kirol, History Buff", Count: 2},
		{Name: "Woodwork Prodigy", Count: 2}, {Name: "Skycoach Conductor", Count: 3},
		{Name: "Encouraging Aviator", Count: 3}, {Name: "Pompous Battlemage", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Skycoach Waypoint", Count: 3}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 4},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
	},
}

var preptargetCarriers = map[string]bool{"Biblioplex Tomekeeper": true, "Skycoach Waypoint": true, "Infinite Coursework": true}

// preptargetTally reads gorge's event log. The cursor resets when a side is
// rebuilt (the log only grows otherwise).
type preptargetTally struct {
	evAt, objAt  int
	prepared     map[state.ObjID]bool        // source -> the designation is on
	srcOf        map[state.ObjID]state.ObjID // exile copy -> its source
	tgtGrant     map[state.ObjID]bool        // source -> its latest grant came from a carrier
	carrier      bool                        // the last Resolve was a carrier's
	grants       int                         // false->true grants (any origin)
	tgtGrants    int                         // ... resolved by a carrier
	unprepares   int                         // Amount -1 of a prepared source outside a cast
	casts        int                         // prepared copies pushed
	castsAfterTg int                         // ... whose source was last granted by a carrier
	unlinked     int                         // exile copies ceased whose source was unprepared by an effect
	effUnprep    map[state.ObjID]bool        // source -> unprepared by an effect, copy not yet cast
}

func (p *preptargetTally) reset() {
	p.evAt, p.objAt = 0, 0
	p.prepared = map[state.ObjID]bool{}
	p.srcOf = map[state.ObjID]state.ObjID{}
	p.tgtGrant = map[state.ObjID]bool{}
	p.effUnprep = map[state.ObjID]bool{}
	p.carrier = false
}

func (p *preptargetTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt || len(g.Objs) < p.objAt {
		p.reset()
	}
	for ; p.objAt < len(g.Objs); p.objAt++ {
		if o := &g.Objs[p.objAt]; o.IsCopy && o.PreparedSource != 0 && o.Zone == state.ZExile {
			p.srcOf[o.ID] = o.PreparedSource
		}
	}
	cast := map[state.ObjID]bool{}
	var unprep, ceased []state.ObjID // sources
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Resolve:
			p.carrier = false
			// an ability object names its source; a spell is its own
			if o := g.Obj(e.Obj); o != nil {
				if o.Ability != nil {
					o = g.Obj(o.Source)
				}
				if o != nil && o.Face() != nil {
					p.carrier = preptargetCarriers[o.Face().Name]
				}
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield {
				delete(p.prepared, e.Obj)
			}
			if src, ok := p.srcOf[e.Obj]; ok && e.To == state.ZCeased && e.From == state.ZExile {
				ceased = append(ceased, src)
			}
		case events.AlterAttribute:
			if e.Text != "Prepared" {
				continue
			}
			if e.Amount >= 1 {
				if !p.prepared[e.Obj] {
					p.prepared[e.Obj] = true
					p.grants++
					p.tgtGrant[e.Obj] = p.carrier
					delete(p.effUnprep, e.Obj)
					if p.carrier {
						p.tgtGrants++
					}
				}
				continue
			}
			if p.prepared[e.Obj] {
				unprep = append(unprep, e.Obj)
			}
			p.prepared[e.Obj] = false
		case events.PutOnStack:
			if src, ok := p.srcOf[e.Obj]; ok {
				cast[e.Obj] = true
				p.casts++
				if p.tgtGrant[src] {
					p.castsAfterTg++
				}
			}
		}
	}
	// an unprepare is the copy's cast (CR 722.3c) when the same batch pushed
	// that copy, an effect's otherwise
	for _, src := range unprep {
		viaCast := false
		for c := range cast {
			if p.srcOf[c] == src {
				viaCast = true
			}
		}
		if !viaCast {
			p.unprepares++
			p.effUnprep[src] = true
		}
	}
	for _, src := range ceased { // the unprepare and the cease may share a batch
		if p.effUnprep[src] {
			p.unlinked++
		}
	}
	p.evAt = len(evs)
	return nil
}

func preptargetSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("preptarget")
	if err != nil {
		t.Fatal(err)
	}
	matchups := [][2][]cardset.Entry{{preptargetDecks[0], preptargetDecks[1]}, {preptargetDecks[1], preptargetDecks[0]}}
	decks := make([][2][]*cards.Card, len(matchups))
	tbls := make([]*kop.Table, len(matchups))
	for mi, md := range matchups {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

func preptargetRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"preptarget-a", "preptarget-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := preparePolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
	CloseSide(k)
	return res
}

const preptargetSeeds = 256

func TestPreptargetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbls := preptargetSetup(t)
	var p preptargetTally
	p.reset()
	var st dirac.PrepareStats
	dirac.DebugPrepareStats = &st
	defer func() { dirac.DebugPrepareStats = nil }()
	games := 0
	for mi := range decks {
		for seed := uint64(1); seed <= preptargetSeeds; seed++ {
			p.reset()
			res := preptargetRun(t, reg, set, decks[mi], tbls[mi], seed, p.check)
			games++
			if res.Stall != "" && res.Stall != "turns" { // "turns": the MaxTurns budget, the common end of a short game
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: kernel targeted grants on a preparation creature %d, on another permanent %d (no-op); kernel unprepared records %d; gorge grants %d (%d by a carrier), effect unprepares of a prepared creature %d, orphaned copies ceased %d; copy casts %d (%d after a carrier grant); max nmay %d, max nobj %d",
		games, st.TgtPrep, st.TgtNon, st.Unprepares, p.grants, p.tgtGrants, p.unprepares, p.unlinked, p.casts, p.castsAfterTg, st.MaxMay, st.MaxObj)
	for _, c := range []struct {
		name       string
		n, minimum int
	}{
		{"targeted grants on a preparation creature", st.TgtPrep, 20},
		{"targeted grants on a non-preparation permanent", st.TgtNon, 10},
		{"unprepares of a prepared creature (gorge)", p.unprepares, 10},
		{"unprepared records removed (kernel)", st.Unprepares, 10},
		{"copy casts after a carrier grant", p.castsAfterTg, 3},
	} {
		if c.n < c.minimum {
			t.Errorf("%d %s, want at least %d", c.n, c.name, c.minimum)
		}
	}
}

// TestPreptargetMutation: an unprepare that does nothing leaves the copy
// castable in the kernel only; the lockstep must diverge within 64 seeds.
func TestPreptargetMutation(t *testing.T) {
	reg, set, decks, tbls := preptargetSetup(t)
	dirac.DebugPrepTargetNoUnprepare = true
	defer func() { dirac.DebugPrepTargetNoUnprepare = false }()
	for mi := range decks {
		for seed := uint64(1); seed <= 64; seed++ {
			res := preptargetRun(t, reg, set, decks[mi], tbls[mi], seed, nil)
			if res.Div != nil {
				t.Logf("mutation: matchup %d seed %d diverged at decision %d: %s %s", mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				return
			}
		}
	}
	t.Error("the no-unprepare mutation never diverged in 64 seeds per seating")
}
