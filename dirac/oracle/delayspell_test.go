package oracle

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The delayed SpellCast lockstep (CR 603.7, 603.7b):
// "when you next cast an instant or sorcery spell this turn, copy that
// spell" registered by spells (Teach by Example, Twinferno, Striking
// Palette), a sorcery (Doublecast), an activation (Rimefire Torque; Cursed Recording is build-refused: no TIME counter kind), an attack
// trigger (Najal), a combat-damage trigger (Mercurial Spelldancer) and Saga
// chapters (Summon: G.F. Cerberus). The counters read gorge's event log,
// where the branches are taken.
var delaySpellDecks = [2][]cardset.Entry{
	{
		{Name: "Teach by Example", Count: 4}, {Name: "Twinferno", Count: 4},
		{Name: "Najal, the Storm Runner", Count: 2}, {Name: "Mercurial Spelldancer", Count: 3},
		{Name: "Doublecast", Count: 2}, {Name: "Way of the Cryomancer", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Opt", Count: 2},
		{Name: "Burst Lightning", Count: 3}, {Name: "Rimefire Torque", Count: 1},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Summon: G.F. Cerberus", Count: 3}, {Name: "Pigment Wrangler", Count: 3},
		{Name: "Twinferno", Count: 3}, {Name: "Lightning Strike", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Gray Ogre", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 6},
	},
}

// delaySpellReg is a live SpellCast registration as gorge holds it.
type delaySpellReg struct {
	src     state.ObjID
	ctrl    state.PlayerID
	granted bool // registered by a granted ability (DelayedRegister Text "|GB=")
}

// delaySpellTally counts the arms. The event cursor resets when a side is
// rebuilt (the log rewinds).
type delaySpellTally struct {
	evAt        int
	live        map[uint32]delaySpellReg // SpellCast registrations live at the last check
	lastCaster  state.PlayerID
	lastIS      bool
	castPushes  int      // DelayedPushes since the last cast
	skipWait    []uint32 // live registrations a non-instant/sorcery cast must leave alone
	registered  int
	fired       int
	copies      int
	two         int // casts that fired two registrations
	skipped     int // a non-instant/sorcery cast by a registration's controller
	oppCast     int // an instant or sorcery cast by the other player while a registration is live
	oppPush     int // a push for a registration whose controller did not cast (must be 0)
	wrongFire   int // a push for a cast that was not an instant or sorcery (must be 0)
	granted     int // registrations made by a GRANTED delayed SpellCast ability (Way of the Cryomancer)
	grantedFire int // those registrations that fired
	expired     int // a SpellCast registration removed at a turn change
	srcGone     int // a push whose registration's source is off the stack and battlefield
	maxNdp      int
}

// add sums o's counts into p.
func (p *delaySpellTally) add(o *delaySpellTally) {
	p.registered += o.registered
	p.fired += o.fired
	p.copies += o.copies
	p.two += o.two
	p.skipped += o.skipped
	p.oppCast += o.oppCast
	p.oppPush += o.oppPush
	p.wrongFire += o.wrongFire
	p.granted += o.granted
	p.grantedFire += o.grantedFire
	p.expired += o.expired
	p.srcGone += o.srcGone
}

func delaySpellIS(g *state.Game, id state.ObjID) bool {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		for _, t := range o.Face().Types {
			if t == "Instant" || t == "Sorcery" {
				return true
			}
		}
	}
	return false
}

func (p *delaySpellTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
		p.live = map[uint32]delaySpellReg{}
	}
	turnChange := false
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			turnChange = true
		case events.DelayedRegister:
			if strings.HasPrefix(e.Text, "SpellCast:") {
				p.registered++
				if strings.Contains(e.Text, "|GB=") {
					p.granted++
				}
			}
		case events.PutOnStack:
			p.lastCaster, p.castPushes = e.Player, 0
			p.lastIS = delaySpellIS(g, e.Obj)
			opp := false
			for id, r := range p.live {
				opp = opp || r.ctrl != e.Player
				if r.ctrl == e.Player && !p.lastIS {
					p.skipWait = append(p.skipWait, id)
				}
			}
			if opp && p.lastIS {
				p.oppCast++
			}
		case events.DelayedPush:
			r, ok := p.live[uint32(e.Amount)]
			if !ok {
				continue
			}
			p.fired++
			if r.granted {
				p.grantedFire++
			}
			if p.castPushes++; p.castPushes == 2 {
				p.two++
			}
			if r.ctrl != p.lastCaster {
				p.oppPush++
			}
			if !p.lastIS {
				p.wrongFire++
			}
			if o := g.Obj(r.src); o != nil && o.Zone != state.ZStack && o.Zone != state.ZBattlefield {
				p.srcGone++
			}
		case events.StackCopy:
			p.copies++
		case events.DelayedRemove:
			// only a removal after this batch's TurnChange (gorge's
			// collectExpiredDelayedTriggers runs there) of a registration
			// live before the batch counts
			if _, ok := p.live[uint32(e.Amount)]; ok && turnChange {
				p.expired++
			}
		}
	}
	p.evAt = len(evs)
	clear(p.live)
	for i := range g.Delayed {
		if dt := &g.Delayed[i]; dt.EventMode == "SpellCast" {
			p.live[dt.ID] = delaySpellReg{src: dt.Source, ctrl: dt.Controller, granted: dt.Grantor != 0}
		}
	}
	// A non-instant/sorcery cast leaves its controller's registrations
	// live: count it only when the registration survived the cast.
	for _, id := range p.skipWait {
		if _, ok := p.live[id]; ok {
			p.skipped++
		}
	}
	p.skipWait = p.skipWait[:0]
	return nil
}

// delaySpellRegisters names the cards whose cast or activation registers.
var delaySpellRegisters = map[string]bool{
	"Teach by Example": true, "Twinferno": true, "Doublecast": true, "Rimefire Torque": true,
}

// delaySpellPolicy orders the legal options into classes and picks at
// random within the first non-empty one, falling back to the whole
// decision, so it never removes a legal action. Classes: reg (a cast or
// activation that registers, a prepared Pigment Wrangler's Striking Palette
// among them), spell (an instant or sorcery cast), land, other (any other
// cast from hand), pass.
//
//   - an opponent's registration live: spell first (the opponent's-cast arm);
//   - two or more of the actor's live: spell, reg, land (a registering
//     instant or sorcery fires them too);
//   - one live: with three lands untapped, reg
//     (two registrations on one cast); else spell, reg, land;
//   - none live: with three lands untapped and an instant or sorcery in
//     hand, reg; else land, other, pass -- the instants are held for a
//     registration, and the untapped lands for the opponent's turn.
//
// With a nonempty stack and no opponent's registration it picks at random.
func delaySpellPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var reg, spell, land, other, pass []Canon
	objs := map[Ref]state.ObjID{}
	n := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.G.fr == nil || c.G.fr.d == nil || c.G.fr.d.Kind != "priority" {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		actor := state.PlayerID(c.G.fr.d.Player)
		mine, theirs := 0, 0
		for i := range g.Delayed {
			if dt := &g.Delayed[i]; dt.EventMode == "SpellCast" {
				if dt.Controller == actor {
					mine++
				} else {
					theirs++
				}
			}
		}
		if len(g.Stack) != 0 && theirs == 0 {
			return rnd.Pick(c)
		}
		untapped, held := 0, 0
		for _, id := range g.Zone(state.ZBattlefield, actor) {
			if o := g.Obj(id); o != nil && !o.Tapped && o.Face() != nil {
				for _, t := range o.Face().Types {
					if t == "Land" {
						untapped++
					}
				}
			}
		}
		for _, id := range g.Zone(state.ZHand, actor) {
			if o := g.Obj(id); o != nil && o.Face() != nil && !delaySpellRegisters[o.Face().Name] && delaySpellIS(g, id) {
				held++
			}
		}
		reg, spell, land, other, pass = reg[:0], spell[:0], land[:0], other[:0], pass[:0]
		clear(objs)
		for _, z := range []state.Zone{state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			for _, id := range g.Zone(z, actor) {
				objs[c.G.InstRef(id)] = id
			}
		}
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPass:
				pass = append(pass, a)
				continue
			case ActPlayLand:
				land = append(land, a)
				continue
			case ActCast, ActCastFace, ActActivate:
			default:
				continue
			}
			id, ok := objs[a.Src()]
			ob := g.Obj(id)
			if !ok || ob == nil || ob.Face() == nil {
				continue
			}
			name := ob.Face().Name
			switch {
			case delaySpellRegisters[name] || name == "Pigment Wrangler" && ob.Zone == state.ZBattlefield && a.Kind() != ActActivate:
				reg = append(reg, a)
			case a.Kind() != ActActivate && delaySpellIS(g, id):
				spell = append(spell, a)
			case ob.Zone == state.ZHand:
				other = append(other, a)
			}
		}
		n++
		var classes [3][]Canon
		switch {
		case theirs > 0:
			classes = [3][]Canon{spell, reg, land}
		case mine >= 2:
			classes = [3][]Canon{spell, reg, land}
		case mine == 1 && untapped >= 3:
			classes = [3][]Canon{reg, spell, land}
		case mine == 1:
			classes = [3][]Canon{spell, reg, land}
		case untapped >= 3 && held > 0:
			classes = [3][]Canon{reg, land, other}
		default:
			classes = [3][]Canon{land, other, pass}
		}
		for _, b := range classes {
			if len(b) > 0 {
				d := *c.D
				d.Options = b
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

func delaySpellSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("delayspell")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range delaySpellDecks {
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
	tbl, _, _, err := set.Matchup(delaySpellDecks[0], delaySpellDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func delaySpellRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(reg, [2]string{"delayspell-a", "delayspell-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := delaySpellPolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100, Check: check})
	CloseSide(k)
	return res
}

const delaySpellSeeds = 1500

func TestDelayspellLockstep(t *testing.T) {
	// A fresh clone has no corpus: skip here, before the chunks (each would
	// skip alone and leave this test's totals empty).
	corpusDirOrSkip(t)
	if isolate(t) {
		return
	}

	// the hook is the process's (this test plays alone, isolated); its
	// chunks share it, so it keeps the maximum atomically
	prevHook := OccupancyHook
	var maxNdp atomic.Int64
	OccupancyHook = func(s *dirac.State) {
		var occ dirac.Occupancy
		s.Occupancy(&occ)
		n := int64(occ.N[dirac.PoolDProg])
		for cur := maxNdp.Load(); n > cur && !maxNdp.CompareAndSwap(cur, n); cur = maxNdp.Load() {
		}
	}
	defer func() { OccupancyHook = prevHook }()
	var p delaySpellTally
	seedChunks(t, 1, delaySpellSeeds, 250, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		reg, set, decks, tbl := delaySpellSetup(t)
		part := delaySpellTally{live: map[uint32]delaySpellReg{}}
		for seed := lo; seed <= hi; seed++ {
			res := delaySpellRun(t, reg, set, decks, tbl, seed, part.check)
			if res.Stall != "" {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		mu.Lock()
		p.add(&part)
		mu.Unlock()
	})
	p.maxNdp = int(maxNdp.Load())
	t.Logf("%d games: %d registered (%d granted), %d fired (%d granted), %d copies, %d two-on-one-cast, %d skipped by a non-matching cast, %d opponent casts (%d pushes), %d wrong fires, %d expired, %d fired after the source left, max ndp %d",
		delaySpellSeeds, p.registered, p.granted, p.fired, p.grantedFire, p.copies, p.two, p.skipped, p.oppCast, p.oppPush, p.wrongFire, p.expired, p.srcGone, p.maxNdp)
	for _, c := range []struct {
		name string
		n    int
	}{{"registered", p.registered}, {"fired", p.fired}, {"copies", p.copies}, {"two registrations on one cast", p.two},
		{"granted registrations", p.granted}, {"granted fires", p.grantedFire},
		{"skipped by a non-matching cast", p.skipped}, {"opponent's cast", p.oppCast}, {"expired", p.expired}, {"source gone", p.srcGone}} {
		if c.n < 10 {
			t.Errorf("%s: %d, want at least 10", c.name, c.n)
		}
	}
	if p.oppPush != 0 {
		t.Errorf("%d pushes on an opponent's cast, want 0", p.oppPush)
	}
	if p.wrongFire != 0 {
		t.Errorf("%d pushes on a non-instant/sorcery cast, want 0", p.wrongFire)
	}
}

// TestDelayspellMutation: each mutation must diverge on some seed.
func TestDelayspellMutation(t *testing.T) {
	reg, set, decks, tbl := delaySpellSetup(t)
	muts := []struct {
		name string
		flag *bool
	}{
		{"kept after firing", &dirac.DebugDelaySpellKeep},
		{"never expires", &dirac.DebugDelaySpellNoExpire},
		{"any caster", &dirac.DebugDelaySpellAnyCaster},
	}
	for _, mu := range muts {
		*mu.flag = true
		diverged := false
		for seed := uint64(1); seed <= delaySpellSeeds && !diverged; seed++ {
			res := delaySpellRun(t, reg, set, decks, tbl, seed, nil)
			if diverged = res.Div != nil; diverged {
				t.Logf("mutation %q: seed %d diverged at decision %d: %s %s", mu.name, seed, res.Div.At, res.Div.What, res.Div.Field)
			}
		}
		*mu.flag = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
