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

// The permanent-spell copy lockstep (CR 111.13, 608.3f, 707.10f): a copy of
// a permanent spell becomes a token as it resolves. Deck a copies artifact
// and creature spells (Sage of the Skies' own cast, Drafna's targeted copy,
// Mendicant Core, Jin-Gitaxias); deck b copies instants, sorceries and
// Adventure spells (Social Snub, Chancellor of Tales).
//
// The counters come from gorge's event log, where the branch is taken.
var permcopyDecks = [2][]cardset.Entry{
	{
		{Name: "Sage of the Skies", Count: 4}, {Name: "Drafna, Founder of Lat-Nam", Count: 4},
		{Name: "Mendicant Core, Guidelight", Count: 2}, {Name: "Jin-Gitaxias, Progress Tyrant", Count: 1},
		{Name: "Ornithopter", Count: 1}, {Name: "Patchwork Automaton", Count: 4},
		{Name: "Juggernaut", Count: 4}, {Name: "Mind Stone", Count: 3}, {Name: "Opt", Count: 3},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 7},
	},
	{
		{Name: "Social Snub", Count: 4}, {Name: "Chancellor of Tales", Count: 4},
		{Name: "Beluna's Gatekeeper", Count: 2}, {Name: "Obyra's Attendants", Count: 2},
		{Name: "Bilbo Baggins, Burglar", Count: 2}, {Name: "Mind Spring", Count: 4}, {Name: "Healer's Hawk", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Coral Merfolk", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 5},
	},
}

// permcopyPolicy casts (a card or an Adventure face) or activates whenever
// it can, answers yes to every yes/no ask (the copy and pay asks) and is
// otherwise random. Gorge prices an Adventure face from untapped lands too
// so on seeds 1 mod 4 (gorge's lowering surface) a face cast is
// taken whenever it is offered.
// Seeds 0 mod 4 are the plain random policy. It never removes a legal
// option from the random pick's menu when nothing is preferred.
func permcopyPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	if seed%4 == 0 {
		return rnd
	}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		var yes Canon
		hasYes, hasNo := false, false
		for _, a := range c.D.Options {
			switch k := a.Kind(); k {
			case ActCastFace:
				if seed%4 == 1 {
					return a
				}
				buf = append(buf, a)
			case ActCast, ActActivate:
				buf = append(buf, a)
			case ActChoose:
				yes, hasYes = a, true
			case ActChoose + 1:
				hasNo = true
			}
		}
		if hasYes && hasNo && len(c.D.Options) == 2 {
			return yes
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

type permcopyMint struct {
	perm, adv bool
	src       string
}

type permcopyTally struct {
	evAt     int
	scanned  int // g.Objs scanned
	pend     []permcopyMint
	copies   map[state.ObjID]bool // permanent-spell copies minted (live ids)
	onBF     map[state.ObjID]bool
	lastSrc  string
	minted   int // a StackCopy of a creature or artifact spell
	token    int // that copy moved stack -> battlefield as a token, not a copy
	stillCp  int // ... but still IsCopy on the battlefield (must be 0)
	ceased   int // such a token reached ZCeased
	targeted int // Drafna's copy of a target artifact spell
	selfCopy int // Social Snub's copy of itself
	advCopy  int // Chancellor of Tales copying an Adventure spell
	tokCr    int // TokenCreate events (the pool has no token maker: must be 0)
}

func (c *permcopyTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt, c.scanned = 0, 0
		c.pend = c.pend[:0]
		c.copies, c.onBF = map[state.ObjID]bool{}, map[state.ObjID]bool{}
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind.String() {
		case events.TokenCreate.String():
			c.tokCr++
		case "stack_resolve":
			c.lastSrc = ""
			if o := g.Obj(e.Obj); o != nil {
				src := o
				if o.Source != 0 {
					if so := g.Obj(o.Source); so != nil {
						src = so
					}
				}
				if src.Card != nil {
					c.lastSrc = src.Face().Name
				}
			}
		case "stack_copy":
			// the copy is the next card object gorge creates
			perm, adv := true, false
			if so := g.Obj(e.Obj); so != nil && so.Card != nil {
				for _, t := range so.Face().Types {
					if t == "Instant" || t == "Sorcery" {
						perm = false
					}
					if t == "Adventure" {
						adv = true
					}
				}
			}
			c.pend = append(c.pend, permcopyMint{perm: perm, adv: adv, src: c.lastSrc})
			if perm {
				c.minted++
				if c.lastSrc == "Drafna, Founder of Lat-Nam" {
					c.targeted++
				}
			} else if c.lastSrc == "Social Snub" {
				c.selfCopy++
			} else if adv && c.lastSrc == "Chancellor of Tales" {
				c.advCopy++
			}
		case "move_zone":
			if e.From == state.ZStack && e.To == state.ZBattlefield && c.copies[e.Obj] {
				if o := g.Obj(e.Obj); o != nil {
					if o.IsToken && !o.IsCopy {
						c.token++
					}
					if o.IsCopy {
						c.stillCp++
					}
				}
				c.onBF[e.Obj] = true
			} else if e.To == state.ZCeased && c.onBF[e.Obj] {
				c.ceased++
				delete(c.onBF, e.Obj)
			}
		}
	}
	// bind the copy objects created since the last call (in this pool every
	// card object born after genesis with a copy or token flag is a spell
	// copy) to the mints, in creation order
	for ; c.scanned < len(g.Objs); c.scanned++ {
		o := &g.Objs[c.scanned]
		if o.Card == nil || !(o.IsCopy || o.IsToken) || len(c.pend) == 0 {
			continue
		}
		if c.pend[0].perm {
			c.copies[o.ID] = true
		}
		c.pend = c.pend[1:]
	}
	c.evAt = len(evs)
	return nil
}

// permcopyFix is permcopySetup's result: the registry, the set and every
// matchup's decks and table (per test, so parallel tests share nothing).
type permcopyFix struct {
	reg  *cards.Registry
	set  cardset.Set
	all  [permcopyMatchups][2][]*cards.Card
	tbls [permcopyMatchups]*kop.Table
}

func permcopySetup(t *testing.T) *permcopyFix {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("permcopy")
	if err != nil {
		t.Fatal(err)
	}
	// matchup 0: a vs b, 1: b vs a, 2: b vs b (a vs a loops Drafna's bounce past the cast ring)
	pairs := [permcopyMatchups][2]int{{0, 1}, {1, 0}, {1, 1}}
	var allDecks [permcopyMatchups][2][]*cards.Card
	var allTbls [permcopyMatchups]*kop.Table
	for mi, pr := range pairs {
		md := [2][]cardset.Entry{permcopyDecks[pr[0]], permcopyDecks[pr[1]]}
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					allDecks[mi][s] = append(allDecks[mi][s], c)
				}
			}
			if len(allDecks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(allDecks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		allTbls[mi] = tbl
	}
	return &permcopyFix{reg: reg, set: set, all: allDecks, tbls: allTbls}
}

const (
	permcopySeeds    = 240
	permcopyMatchups = 3
)

func permcopyRun(t *testing.T, fx *permcopyFix, mi int, seed uint64, tal *permcopyTally, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	m, err := NewMatchup(fx.reg, [2]string{"permcopy-a", "permcopy-b"}, fx.all[mi], 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(fx.set, fx.tbls[mi], cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, permcopyPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 60, Check: check})
	CloseSide(k)
	return res
}

func newPermcopyTally() *permcopyTally {
	return &permcopyTally{copies: map[state.ObjID]bool{}, onBF: map[state.ObjID]bool{}}
}

func TestPermcopyLockstep(t *testing.T) {
	t.Parallel()

	fx := permcopySetup(t)
	tal := newPermcopyTally()
	games, ringStalls := 0, 0
	for mi := 0; mi < permcopyMatchups; mi++ {
		for seed := uint64(1); seed <= permcopySeeds; seed++ {
			res := permcopyRun(t, fx, mi, seed, tal, tal.check)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if res.Stall == KernelStallPrefix+"castring" {
				// a greedy caster pushes more than castRingN spells in a
				// turn (the face casts are priced from untapped lands): a
				// capacity stall, not a divergence. Re-measured 0/720 with
				// the shown-face cast planner (rules payment_plan.go
				// plannedCastFace); a stall here is a new declared limit, not
				// tolerated silently.
				ringStalls++
			} else if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	t.Logf("%d cast-ring capacity stalls", ringStalls)
	if ringStalls != 0 {
		t.Errorf("%d cast-ring capacity stalls, want 0 (the measured count)", ringStalls)
	}
	t.Logf("%d games: %d permanent copies minted, %d resolved as tokens (%d still copies), %d ceased, %d targeted, %d self-copies, %d adventure copies, %d TokenCreate events",
		games, tal.minted, tal.token, tal.stillCp, tal.ceased, tal.targeted, tal.selfCopy, tal.advCopy, tal.tokCr)
	for _, n := range []struct {
		name string
		got  int
	}{
		{"permanent copies minted", tal.minted}, {"copies resolved as tokens", tal.token},
		{"token copies ceased", tal.ceased}, {"targeted permanent copies", tal.targeted},
		{"instant/sorcery self-copies", tal.selfCopy}, {"adventure copies", tal.advCopy},
	} {
		if n.got < 10 {
			t.Errorf("%d %s, want at least 10", n.got, n.name)
		}
	}
	if tal.tokCr != 0 || tal.stillCp != 0 {
		t.Errorf("%d TokenCreate events, %d resolved copies still IsCopy; want 0", tal.tokCr, tal.stillCp)
	}
}

// TestPermcopyMutation: each mutation must diverge on some seed.
func TestPermcopyMutation(t *testing.T) {
	fx := permcopySetup(t)
	muts := []struct {
		name string
		set  func(bool)
	}{
		{"no token flip", func(b bool) { dirac.DebugPermCopyNoToken = b }},
		{"permanent copy skipped", func(b bool) { dirac.DebugPermCopySkip = b }},
	}
	for _, mu := range muts {
		mu.set(true)
		diverged := false
		for mi := 0; mi < permcopyMatchups && !diverged; mi++ {
			for seed := uint64(1); seed <= permcopySeeds && !diverged; seed++ {
				res := permcopyRun(t, fx, mi, seed, newPermcopyTally(), nil)
				if diverged = res.Div != nil; diverged {
					t.Logf("mutation %q: matchup %d seed %d diverged at decision %d: %s %s", mu.name, mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				}
			}
		}
		mu.set(false)
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
