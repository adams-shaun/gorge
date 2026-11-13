package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Wrath of Khan lockstep: chapter I deals 4 damage to an opposing creature
// and registers a delayed trigger that, at the beginning of the next combat
// this turn, gains control of it until end of turn (CR 603.7; the creature
// that died to the damage is found nowhere), chapter II exiles a card from
// hand as its own cost to draw, chapter III makes the opponent sacrifice. The
// opposing deck carries a creature that survives the damage and ones that do
// not, and burn that kills the remembered creature before combat. The
// counters read gorge's event log.
var kwrathkhanDecks = [2][]cardset.Entry{
	{
		{Name: "The Wrath of Khan", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 2}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Colossal Dreadmaw", Count: 5}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Shock", Count: 2}, {Name: "Mountain", Count: 11}, {Name: "Forest", Count: 10},
	},
}

// kwrathkhanBody is chapter I's delayed body SVar (the Execute$ name).
const kwrathkhanBody = "TrigGainControl"

// kwrathkhanTally counts the arms. The event cursor resets when a side is
// rebuilt (the log rewinds).
type kwrathkhanTally struct {
	evAt       int
	live       map[uint32]bool // chapter I registrations live at the last check
	registered int
	fired      int
	expired    int // removed unfired
	gained     int // control changes at a beginning of combat after a fire
	exiled     int // cards exiled from hand (chapter II's cost; no other card here exiles from hand)
	maxNdp     int
}

func (p *kwrathkhanTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
		clear(p.live)
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.DelayedRegister:
			if e.Counter == kwrathkhanBody {
				p.registered++
			}
		case events.DelayedPush:
			if p.live[uint32(e.Amount)] {
				p.fired++
			}
		case events.DelayedRemove:
			if p.live[uint32(e.Amount)] {
				p.expired++
			}
		case events.ControlChange:
			p.gained++
		case events.MoveZone:
			if e.From == state.ZHand && e.To == state.ZExile {
				p.exiled++
			}
		}
	}
	p.evAt = len(evs)
	clear(p.live)
	for i := range g.Delayed {
		if dt := &g.Delayed[i]; dt.Execute == kwrathkhanBody {
			p.live[dt.ID] = true
		}
	}
	return nil
}

func kwrathkhanSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wrathkhan")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwrathkhanDecks {
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
	tbl, _, _, err := set.Matchup(kwrathkhanDecks[0], kwrathkhanDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func kwrathkhanRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"kwrathkhan-a", "kwrathkhan-b"}, decks, 1)
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

const kwrathkhanSeeds = 256

func TestTheWrathOfKhanLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbl := kwrathkhanSetup(t)
	p := kwrathkhanTally{live: map[uint32]bool{}}
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
	for seed := uint64(1); seed <= kwrathkhanSeeds; seed++ {
		res := kwrathkhanRun(t, reg, set, decks, tbl, seed, p.check)
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
	t.Logf("%d games: %d divergences, %d kernel stalls, %d registered, %d fired, %d expired unfired, %d control changes, %d hand cards exiled, max ndp %d",
		kwrathkhanSeeds, divs, stalls, p.registered, p.fired, p.expired, p.gained, p.exiled, p.maxNdp)
	if p.registered < 40 {
		t.Errorf("chapter I registrations: %d, want at least 40", p.registered)
	}
	if p.gained < 20 {
		t.Errorf("control changes: %d, want at least 20", p.gained)
	}
	if p.exiled < 20 {
		t.Errorf("cards exiled from hand: %d, want at least 20", p.exiled)
	}
}

// TestTheWrathOfKhanMutation: a beginning-of-combat program the kernel drops
// unrun must diverge on some seed (gorge gains control of the creature, the
// kernel does not).
func TestTheWrathOfKhanMutation(t *testing.T) {
	reg, set, decks, tbl := kwrathkhanSetup(t)
	dirac.DebugDelayBeginCombatOff = true
	defer func() { dirac.DebugDelayBeginCombatOff = false }()
	for seed := uint64(1); seed <= kwrathkhanSeeds; seed++ {
		if res := kwrathkhanRun(t, reg, set, decks, tbl, seed, nil); res.Div != nil {
			t.Logf("mutation: seed %d diverged at decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatalf("a dropped beginning-of-combat program never diverged in %d seeds", kwrathkhanSeeds)
}
