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

// "Can block any number of creatures" (CanBlockAny$ True, CR 509.1a): the
// creature is declared against every attacker it can block, and its
// controller divides its combat damage among all of them (CR 510.1d). The
// canblockany fixture seats the carriers (seat 0, and seat 3 in the pod)
// against go-wide token makers (seats 1 and 2), whose attackers outnumber a
// blocker's two.

// canBlockAnyDecks are the fixture's four decks, each 40 cards.
func canBlockAnyDecks() [4][]cardset.Entry {
	blockers := []cardset.Entry{
		{Name: "Wall of Glare", Count: 4}, {Name: "Palace Guard", Count: 4},
		{Name: "Guardian of the Gateless", Count: 3}, {Name: "Ironfist Crusher", Count: 3},
		{Name: "Avatar of Hope", Count: 3}, {Name: "Entangler", Count: 4},
		{Name: "Raise the Alarm", Count: 2}, {Name: "Plains", Count: 17},
	}
	wide := []cardset.Entry{
		{Name: "Dragon Fodder", Count: 4}, {Name: "Krenko's Command", Count: 4},
		{Name: "Hordeling Outburst", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Viashino Runner", Count: 4},
		{Name: "Mountain", Count: 16},
	}
	return [4][]cardset.Entry{blockers, wide, wide, blockers}
}

// canBlockAnyPod is the fixture set and its table.
type canBlockAnyPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newCanBlockAnyPod(t *testing.T, fourSeat bool) canBlockAnyPod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("canblockany")
	if err != nil {
		t.Fatal(err)
	}
	decks := canBlockAnyDecks()
	n := 2
	if fourSeat {
		n = 4
	}
	lists := make([][]cardset.Entry, n)
	for i := range lists {
		c := 0
		for _, e := range decks[i] {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
		lists[i] = decks[i]
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return canBlockAnyPod{reg: reg, set: set, tbl: tbl, decks: decks}
}

// canBlockAnyCounts is what a finished gorge log shows the mechanic did.
type canBlockAnyCounts struct {
	multi   int // blockers declared against two or more attackers
	triple  int // blockers declared against three or more attackers
	divided int // multi-blockers that damaged at least two of their attackers
}

func countCanBlockAny(g *GorgeSide) (c canBlockAnyCounts) {
	e := g.E
	var watch []map[state.ObjID]bool // each multi-blocker's attackers, hit or not
	var hits []map[state.ObjID]bool
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		switch ev.Kind {
		case events.DeclareBlockers:
			atks := map[state.ObjID]map[state.ObjID]bool{}
			for _, pr := range ev.Pairs {
				if atks[pr[1]] == nil {
					atks[pr[1]] = map[state.ObjID]bool{}
				}
				atks[pr[1]][pr[0]] = true
			}
			for _, as := range atks {
				if len(as) >= 2 {
					c.multi++
					watch = append(watch, as)
					hits = append(hits, map[state.ObjID]bool{})
				}
				if len(as) >= 3 {
					c.triple++
				}
			}
		case events.Damage:
			if ev.Amount <= 0 {
				continue
			}
			for j, as := range watch {
				if as[ev.Obj] {
					hits[j][ev.Obj] = true
				}
			}
		}
	}
	for _, h := range hits {
		if len(h) >= 2 {
			c.divided++
		}
	}
	return c
}

func canBlockAnyDeckOf(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for i := 0; i < e.Count; i++ {
			out = append(out, e.Name)
		}
	}
	return out
}

// canBlockAnyCarrier names the creatures that block without limit.
func canBlockAnyCarrier(name string) bool {
	switch name {
	case "Wall of Glare", "Palace Guard", "Guardian of the Gateless", "Ironfist Crusher", "Avatar of Hope":
		return true
	}
	return false
}

// canBlockAnyPolicy steers the games to the mechanic: the go-wide seats
// attack with everything, and a block declaration piles attackers on a
// carrier before any other blocker is declared. Every other decision falls
// to castFirstPolicy, so the divisions are the seeded random splits.
func canBlockAnyPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.G.E == nil {
			return base.Pick(c)
		}
		switch c.D.Kind {
		case DAttackers:
			if c.D.Player == 1 || c.D.Player == 2 {
				for _, o := range c.D.Options {
					if o.Kind() == ActAttack {
						return o
					}
				}
			}
		case DBlockers:
			for _, o := range c.D.Options {
				if o.Kind() != ActBlock {
					continue
				}
				opt, ok := c.G.GorgeOption(o)
				if !ok {
					continue
				}
				if ob := c.G.E.G.Obj(opt.Obj); ob != nil && ob.Face() != nil && canBlockAnyCarrier(ob.Face().Name) {
					return o
				}
			}
		}
		return base.Pick(c)
	})
}

// play plays one seed of the pod (fourSeat=false: the first two decks only).
func (p canBlockAnyPod) play(t *testing.T, seed uint64, fourSeat bool, maxTurns int32) (Result, canBlockAnyCounts) {
	t.Helper()
	d := p.decks
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{canBlockAnyDeckOf(d[0]), canBlockAnyDeckOf(d[1])},
		Seed:  seed, PassElision: true, Surface: "full"}
	if fourSeat {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{canBlockAnyDeckOf(d[2]), canBlockAnyDeckOf(d[3])}
	}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var c canBlockAnyCounts
	rc.Finish = func(g *GorgeSide) { c = countCanBlockAny(g) }
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, canBlockAnyPolicy(seed), rc)
	return res, c
}

func canBlockAnyFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v",
		seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// canBlockAnySweep plays seeds 1..n and sums what the games reached.
func canBlockAnySweep(t *testing.T, fourSeat bool, n int) canBlockAnyCounts {
	t.Helper()
	p := newCanBlockAnyPod(t, fourSeat)
	var sum canBlockAnyCounts
	for seed := uint64(1); seed <= uint64(n); seed++ {
		res, c := p.play(t, seed, fourSeat, 40)
		if res.Div != nil {
			canBlockAnyFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.multi += c.multi
		sum.triple += c.triple
		sum.divided += c.divided
		t.Logf("seed %d: %d decisions, over %v; %d multi-blocker(s), %d of three or more, %d divided", seed, res.Decisions, res.Over, c.multi, c.triple, c.divided)
	}
	return sum
}

// TestCanBlockAnyFourSeatLockstep plays the four-seat pod: carriers must
// block three or more attackers and divide their damage among them, and the
// kernel must offer, declare and divide as gorge does at every seat. No
// divergence and no stall.
func TestCanBlockAnyFourSeatLockstep(t *testing.T) {
	sum := canBlockAnySweep(t, true, 100)
	if sum.triple == 0 {
		t.Fatalf("no blocker was declared against three attackers: %+v", sum)
	}
	if sum.divided == 0 {
		t.Fatalf("no multi-blocker damaged two attackers: %+v", sum)
	}
}

// TestCanBlockAnyTwoSeatLockstep is the two-seat form.
func TestCanBlockAnyTwoSeatLockstep(t *testing.T) {
	sum := canBlockAnySweep(t, false, 60)
	if sum.triple == 0 {
		t.Fatalf("no blocker was declared against three attackers: %+v", sum)
	}
	if sum.divided == 0 {
		t.Fatalf("no multi-blocker damaged two attackers: %+v", sum)
	}
}

// TestCanBlockAnyDivisionMutation is the mutation test: putting all of a
// multi-blocker's damage on its first attacker must diverge from gorge's
// division on the first seed that divides one.
func TestCanBlockAnyDivisionMutation(t *testing.T) {
	p := newCanBlockAnyPod(t, false)
	dirac.DebugExtraBlockDivideFirst = true
	defer func() { dirac.DebugExtraBlockDivideFirst = false }()
	for seed := uint64(1); seed <= 24; seed++ {
		res, c := p.play(t, seed, false, 40)
		if c.divided == 0 {
			continue // nothing to mutate in this seed
		}
		if res.Div == nil {
			t.Fatalf("seed %d: all damage on the first attacker diverged on no decision (%d divided)", seed, c.divided)
		}
		t.Logf("mutation seed %d diverged at %d: %s %s %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		return
	}
	t.Fatal("no seed divided a multi-blocker's damage: the mutation has nothing to break")
}
