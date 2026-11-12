package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The delayed dies-body lockstep (CR 603.7, 603.7c,
// 603.10a): "when that creature dies this turn, <effect>" registered by an
// attack trigger's sub-ability (Grim Javelineer), spells' sub-abilities
// (Turn Inside Out, Scarblade's Malice, Felonious Rage) and a spell's own
// targeted DelayedTrigger (Fatal Fissure), against blockers, burn, removal
// and a sacrifice outlet, and two removal carriers whose bodies hit the
// dead creature's controller (Searing Blood, Virulent Wound), so the remembered creature dies the same turn or
// the registration expires at the turn change. The counters read gorge's
// event log.
var kdelaydiesDecks = [2][]cardset.Entry{
	{
		{Name: "Grim Javelineer", Count: 4}, {Name: "Turn Inside Out", Count: 3}, {Name: "Scarblade's Malice", Count: 3},
		{Name: "Felonious Rage", Count: 3}, {Name: "Fatal Fissure", Count: 3}, {Name: "Goblin Boarders", Count: 3},
		{Name: "Gray Ogre", Count: 2}, {Name: "Eaten Alive", Count: 2}, {Name: "Burst Lightning", Count: 1},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Giant Spider", Count: 4},
		{Name: "Walking Corpse", Count: 3}, {Name: "Burst Lightning", Count: 1}, {Name: "Bake into a Pie", Count: 2},
		{Name: "Stab", Count: 1}, {Name: "Searing Blood", Count: 2}, {Name: "Virulent Wound", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 6},
	},
}

// kdelaydiesBodies are the carriers' Execute SVars: they tell the dies
// registrations apart from Earthbend's own ChangesZone return promise.
var kdelaydiesBodies = map[string]bool{"DBSurveil": true, "DBDread": true, "DBToken": true, "TrigToken": true, "TrigEarthbend": true,
	"TrigDealDamage": true, "TrigPoison": true}

// kdelaydiesTally counts the arms. The event cursor resets when a side is
// rebuilt (the log rewinds).
type kdelaydiesTally struct {
	evAt       int
	live       map[uint32]bool // dies registrations live at the last check
	registered int
	fired      int
	expired    int // removed at a turn change, unfired
	maxNdp     int
}

func (p *kdelaydiesTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
		clear(p.live)
	}
	turnChange := false
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			turnChange = true
		case events.DelayedRegister:
			if kdelaydiesBodies[e.Counter] {
				p.registered++
			}
		case events.DelayedPush:
			if p.live[uint32(e.Amount)] {
				p.fired++
			}
		case events.DelayedRemove:
			if p.live[uint32(e.Amount)] && turnChange {
				p.expired++
			}
		}
	}
	p.evAt = len(evs)
	clear(p.live)
	for i := range g.Delayed {
		if dt := &g.Delayed[i]; dt.EventMode == "ChangesZone" && kdelaydiesBodies[dt.Execute] {
			p.live[dt.ID] = true
		}
	}
	return nil
}

func kdelaydiesSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("delaydies")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdelaydiesDecks {
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
	tbl, _, _, err := set.Matchup(kdelaydiesDecks[0], kdelaydiesDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func kdelaydiesRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"kdelaydies-a", "kdelaydies-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, combatCastPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
	CloseSide(k)
	return res
}

const kdelaydiesSeeds = 256

func TestDelaydiesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set, decks, tbl := kdelaydiesSetup(t)
	p := kdelaydiesTally{live: map[uint32]bool{}}
	prevHook := OccupancyHook
	var occ dirac.Occupancy
	OccupancyHook = func(s *dirac.State) {
		s.Occupancy(&occ)
		if n := occ.N[dirac.PoolDProg]; n > p.maxNdp {
			p.maxNdp = n
		}
	}
	defer func() { OccupancyHook = prevHook }()
	divs, stalls := 0, 0
	for seed := uint64(1); seed <= kdelaydiesSeeds; seed++ {
		res := kdelaydiesRun(t, reg, set, decks, tbl, seed, p.check)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d divergences, %d kernel stalls, %d registered, %d dies-bodies fired, %d expired unfired, max ndp %d",
		kdelaydiesSeeds, divs, stalls, p.registered, p.fired, p.expired, p.maxNdp)
	if p.fired < 40 {
		t.Errorf("dies-bodies fired: %d, want at least 40", p.fired)
	}
	if p.expired < 40 {
		t.Errorf("registrations expired unfired: %d, want at least 40", p.expired)
	}
}

// TestDelaydiesMutation: a dies registration that survives the turn change
// must diverge on some seed (the remembered creature dies a later turn and
// the kernel fires a body gorge does not).
func TestDelaydiesMutation(t *testing.T) {
	reg, set, decks, tbl := kdelaydiesSetup(t)
	dirac.DebugDelayDiesKeep = true
	defer func() { dirac.DebugDelayDiesKeep = false }()
	for seed := uint64(1); seed <= kdelaydiesSeeds; seed++ {
		if res := kdelaydiesRun(t, reg, set, decks, tbl, seed, nil); res.Div != nil {
			t.Logf("mutation: seed %d diverged at decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatalf("a registration kept past the turn change never diverged in %d seeds", kdelaydiesSeeds)
}
