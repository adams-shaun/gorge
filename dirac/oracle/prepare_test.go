package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Prepare lockstep (CR 722): constructed matchups over the generated
// prepare pool. The decks carry every arm:
//
//   - the designation granted as a permanent enters (an Updated ETB
//     replacement) and by a resolving trigger, each minting the exile copy;
//   - a grant to an already prepared permanent (no new instance);
//   - the copy cast from exile (gorge "prepared_copy"), the source
//     unprepared at the push, the copy ceasing from the graveyard;
//   - a source leaving while prepared, its copy ceasing from exile;
//   - a sorcery copy withheld at instant speed, an instant copy cast there;
//   - Count$ValidSelf Card.[!]IsPrepared read both ways.
//
// The counters come from gorge's event log and option lists. The mutations
// break one rule each and must diverge.
var prepareDecks = [2][]cardset.Entry{
	{
		{Name: "Abigale, Poet Laureate", Count: 2}, {Name: "Emeritus of Conflict", Count: 2},
		{Name: "Emeritus of Woe", Count: 2}, {Name: "Honorbound Page", Count: 2},
		{Name: "Leech Collector", Count: 2}, {Name: "Infirmary Healer", Count: 2},
		{Name: "Pompous Battlemage", Count: 2}, {Name: "Adventurous Eater", Count: 2},
		{Name: "Elite Interceptor", Count: 2}, {Name: "Bloodline Recollector", Count: 2},
		{Name: "Bake into a Pie", Count: 2}, {Name: "Exsanguinate", Count: 3},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Tam, Observant Sequencer", Count: 2}, {Name: "Emeritus of Abundance", Count: 2},
		{Name: "Kirol, History Buff", Count: 2}, {Name: "Woodwork Prodigy", Count: 2},
		{Name: "Stingerquill Voxmancer", Count: 2}, {Name: "Paradox Shaper", Count: 2},
		{Name: "Landscape Painter", Count: 2}, {Name: "Campus Composer", Count: 2},
		{Name: "Goblin Negotiation", Count: 2}, {Name: "Skycoach Conductor", Count: 4},
		{Name: "Encouraging Aviator", Count: 3},
		// the two carriers whose hybrid pips are printed outside WUBRG order
		// (Seed Suture's {G/W}, Vigorbloom's {1(G/W)}); the deck keeps Forest
		// and Plains so either face of the pip is payable
		{Name: "Blossom-Blessed Angel", Count: 2}, {Name: "Vigorbloom Vanguard", Count: 2},
		{Name: "Island", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 2},
		{Name: "Plains", Count: 2}, {Name: "Swamp", Count: 1},
	},
}

// prepTally counts the arms. The event cursor mirrors kplot's: the log only
// grows, but resets when a side is rebuilt.
type prepTally struct {
	evAt, objAt int
	prepared    map[state.ObjID]bool        // source -> the designation is on
	srcOf       map[state.ObjID]state.ObjID // exile copy -> its source
	offered     map[state.ObjID]bool        // copies offered prepared_copy in the last priority frame
	lastActor   state.PlayerID

	grantEnter, grantTrig, grantNoop int
	mintBad                          int // grants whose minted-copy count was not 1 (0 for a no-op)
	casts, castUnprep                int
	instCasts                        int // instant copies cast outside the caster's main phase
	ceasedCast, ceasedExile          int
	castEver                         map[state.ObjID]bool
	lastSorc                         bool
	ceasedOther                      map[state.Zone]int
	withheld, badOffer               int  // sorcery copies at instant speed: not offered / offered
	offers                           int  // prepared_copy options seen
	autopay                          bool // the run plays the autopay surface
	offersAutopay                    int  // prepared_copy options seen on the autopay surface
	emptyOffer                       map[state.ObjID]bool
	emptyOffers, emptyTaken          int
	maxMay                           int
	hybOrdPips                       int            // pip announcements for a spell whose printed hybrid pip is out of WUBRG order
	hybOrdCasts                      map[string]int // such a spell's PutOnStack, by face name
}

func (p *prepTally) reset() {
	p.evAt, p.objAt = 0, 0
	p.prepared = map[state.ObjID]bool{}
	p.srcOf = map[state.ObjID]state.ObjID{}
	p.offered = map[state.ObjID]bool{}
	p.emptyOffer = map[state.ObjID]bool{}
	p.castEver = map[state.ObjID]bool{}
	if p.hybOrdCasts == nil {
		p.hybOrdCasts = map[string]int{}
	}
	if p.ceasedOther == nil {
		p.ceasedOther = map[state.Zone]int{}
	}
}

func (p *prepTally) check(ref *GorgeSide, k Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt || len(g.Objs) < p.objAt {
		p.reset()
	}
	minted := map[state.ObjID]int{}
	for ; p.objAt < len(g.Objs); p.objAt++ {
		if o := &g.Objs[p.objAt]; o.IsCopy && o.PreparedSource != 0 && o.Zone == state.ZExile {
			minted[o.PreparedSource]++
			p.srcOf[o.ID] = o.PreparedSource
		}
	}
	entered := map[state.ObjID]bool{}
	cast := map[state.ObjID]bool{} // copies pushed in this batch
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.MoveZone:
			if e.To == state.ZBattlefield {
				entered[e.Obj] = true
			}
			if e.From == state.ZBattlefield {
				delete(p.prepared, e.Obj)
			}
			if _, ok := p.srcOf[e.Obj]; ok && e.To == state.ZCeased {
				switch {
				case p.castEver[e.Obj]:
					p.ceasedCast++ // a cast copy resting after its spell (gorge spellRestZone: exile, then ceases)
				case e.From == state.ZExile:
					p.ceasedExile++
				default:
					p.ceasedOther[e.From]++
				}
			}
		case events.AlterAttribute:
			if e.Text != "Prepared" {
				continue
			}
			if e.Amount >= 1 {
				if p.prepared[e.Obj] {
					p.grantNoop++
					if minted[e.Obj] != 0 {
						p.mintBad++
					}
					continue
				}
				p.prepared[e.Obj] = true
				if minted[e.Obj] != 1 {
					p.mintBad++
				}
				if entered[e.Obj] {
					p.grantEnter++
				} else {
					p.grantTrig++
				}
				continue
			}
			p.prepared[e.Obj] = false
			for c := range cast {
				if p.srcOf[c] == e.Obj {
					p.castUnprep++
				}
			}
		case events.PutOnStack:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && outOfOrderHybrid(o.Face().ManaCost) {
				p.hybOrdCasts[o.Face().Name]++
			}
			if _, ok := p.srcOf[e.Obj]; !ok {
				continue
			}
			cast[e.Obj] = true
			p.castEver[e.Obj] = true
			if p.emptyOffer[e.Obj] {
				p.emptyTaken++
			}
			p.casts++
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().IsInstant() && !p.lastSorc {
				p.instCasts++
			}
		}
	}
	p.evAt = len(evs)
	fr := ref.fr
	if fr != nil && fr.d != nil && fr.d.Kind == "choose" {
		for i := range fr.d.Options {
			if !strings.HasPrefix(fr.d.Options[i].Kind, "pay_") {
				continue
			}
			if o := g.Obj(fr.d.Source); o != nil && o.Face() != nil && outOfOrderHybrid(o.Face().ManaCost) {
				p.hybOrdPips++
			}
			break
		}
	}
	if fr == nil || fr.d == nil || fr.d.Kind != "priority" {
		return nil
	}
	actor := state.PlayerID(fr.d.Player)
	p.lastActor = actor
	sorc := g.Active == actor && g.Step.IsMain() && len(g.Stack) == 0
	p.lastSorc = sorc
	p.offered = map[state.ObjID]bool{}
	for _, r := range kernelFaceRefs(k, faceCodePrepared) {
		id := ref.id.obj[r]
		p.offered[id] = true
		p.offers++
		if p.autopay {
			p.offersAutopay++
		}
		if g.Players[actor].Pool.Total() == 0 {
			p.emptyOffers++
			p.emptyOffer[id] = true
		}
	}
	if !sorc {
		for i := range g.Objs {
			o := &g.Objs[i]
			if !o.IsCopy || o.PreparedSource == 0 || o.Zone != state.ZExile || o.Face() == nil || !o.Face().IsSorcery() {
				continue
			}
			if src := g.Obj(o.PreparedSource); src == nil || src.Zone != state.ZBattlefield || !src.Prepared || src.Controller != actor {
				continue
			}
			if p.offered[o.ID] {
				p.badOffer++
			} else {
				p.withheld++
			}
		}
	}
	return nil
}

// preparePolicy prefers the prepared_copy cast, else plays or casts.
func preparePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&0x0f == faceCodePrepared {
				return a
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func prepareRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"prepare-a", "prepare-b"}, decks, 1)
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
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: check})
	CloseSide(k)
	return res
}

func prepareSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("prepare")
	if err != nil {
		t.Fatal(err)
	}
	// both seatings of the two decks
	matchups := [][2][]cardset.Entry{{prepareDecks[0], prepareDecks[1]}, {prepareDecks[1], prepareDecks[0]}}
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

const prepareSeeds = 200

func TestPrepareLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set, decks, tbls := prepareSetup(t)
	var p prepTally
	p.reset()
	var st dirac.PrepareStats
	dirac.DebugPrepareStats = &st
	defer func() { dirac.DebugPrepareStats = nil }()
	prevHook := OccupancyHook
	OccupancyHook = func(s *dirac.State) {
		if n := s.FeatMayCount(); n > p.maxMay {
			p.maxMay = n
		}
	}
	defer func() { OccupancyHook = prevHook }()
	games := 0
	for mi := range decks {
		for seed := uint64(1); seed <= prepareSeeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			p.reset()
			p.autopay = cfg.GorgeAutopaySurface
			res := prepareRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, p.check)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
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
	t.Logf("%d games: grants %d entering, %d triggered, %d no-op (mint errors %d); %d copy casts (%d unprepared, %d instant at non-sorcery timing), %d resting cast copies ceased, %d orphaned; %d offers (%d on the autopay surface), %d sorcery copies withheld at instant speed (%d offered); empty-pool prepared_copy offers %d, taken %d; ExSelfPrepared true %d false %d; max nmay %d (%d), max nobj %d; ceased from other zones %v",
		games, p.grantEnter, p.grantTrig, p.grantNoop, p.mintBad, p.casts, p.castUnprep, p.instCasts, p.ceasedCast, p.ceasedExile, p.offers, p.offersAutopay, p.withheld, p.badOffer, p.emptyOffers, p.emptyTaken, st.SelfTrue, st.SelfFalse, p.maxMay, st.MaxMay, st.MaxObj, p.ceasedOther)
	t.Logf("out-of-order hybrid: %d pip announcements, casts %v", p.hybOrdPips, p.hybOrdCasts)
	for _, c := range []struct {
		name string
		n    int
	}{
		{"entering grants", p.grantEnter}, {"triggered grants", p.grantTrig}, {"no-op grants", p.grantNoop},
		{"copy casts", p.casts}, {"unprepares at the push", p.castUnprep}, {"instant copy casts at non-sorcery timing", p.instCasts},
		{"cast copies ceased", p.ceasedCast}, {"orphaned copies ceased", p.ceasedExile},
		{"withheld sorcery copies", p.withheld}, {"empty-pool prepared_copy offers", p.emptyOffers},
		{"empty-pool prepared_copy offers taken", p.emptyTaken},
		{"ExSelfPrepared true", st.SelfTrue}, {"ExSelfPrepared false", st.SelfFalse},
		{"out-of-order hybrid pip announcements", p.hybOrdPips},
	} {
		if c.n < 10 {
			t.Errorf("%d %s, want at least 10", c.n, c.name)
		}
	}
	for _, name := range []string{"Seed Suture", "Vigorbloom Vanguard"} {
		if n := p.hybOrdCasts[name]; n < 5 {
			t.Errorf("%s cast %d times, want at least 5", name, n)
		}
	}
	if p.badOffer != 0 {
		t.Errorf("%d sorcery copies offered at instant speed, want 0", p.badOffer)
	}
	if p.mintBad != 0 {
		t.Errorf("%d grants minted the wrong number of copies, want 0", p.mintBad)
	}
}

// TestPrepareMutation: each mutation must diverge on some seed.
func TestPrepareMutation(t *testing.T) {
	reg, set, decks, tbls := prepareSetup(t)
	muts := []struct {
		name string
		on   *bool
	}{
		{"push keeps the record", &dirac.DebugPrepareKeep},
		{"exile copy not exempt from ceasing", &dirac.DebugPrepareCease},
		{"second grant mints a copy", &dirac.DebugPrepareRegrant},
		{"face cast offered only when the pool pays", &dirac.DebugFacePoolOnly},
	}
	for _, mu := range muts {
		*mu.on = true
		diverged := false
		for mi := range decks {
			for seed := uint64(1); seed <= prepareSeeds && !diverged; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				res := prepareRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, nil)
				if diverged = res.Div != nil; diverged {
					t.Logf("mutation %q: matchup %d seed %d diverged at decision %d: %s %s", mu.name, mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				}
			}
		}
		*mu.on = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}

// outOfOrderHybrid reports whether a printed mana cost carries a hybrid pip
// printed outside WUBRG order (Seed Suture's {G/W}). The kernel records the
// reversal in kop.ManaCost.HybOrd, so counting these announcements (and the
// casts) is the occurrence proof for the re-added carriers; the cost walk
// picks up any future out-of-order hybrid card without naming it.
func outOfOrderHybrid(manaCost string) bool {
	for _, h := range cost.ParseCost(manaCost).Hybrid {
		a, b := strings.IndexByte("WUBRG", h.A), strings.IndexByte("WUBRG", h.B)
		if a >= 0 && b >= 0 && a > b {
			return true
		}
	}
	return false
}
