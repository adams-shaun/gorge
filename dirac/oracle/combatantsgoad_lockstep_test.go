package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// combatantsGoadDecks is the combatantsgoad fixture's four decks: seat 0
// carries the ChangeCombatants redirects and both the static and the trigger
// IsGoaded readers; the other seats are aggressive creatures and goad sources
// (the goads make the readers' words matter in combat). The aggro decks carry
// Baeloth Barrityl, Entertainer too: its goaded-attacker dies trigger (a
// departed ValidCard$ IsGoaded, CR 603.10a's look-back) is what seed 30 of the
// pod exercises, with its own static goading the opponents' Taunting Kobolds.
// Each deck is 40.
func combatantsGoadDecks() [4][]cardset.Entry {
	redirects := []cardset.Entry{
		{Name: "Misleading Signpost", Count: 4}, {Name: "Portal Mage", Count: 4},
		{Name: "Island", Count: 16}, {Name: "Mountain", Count: 16},
	}
	aggro := []cardset.Entry{
		{Name: "Taunting Kobold", Count: 4}, {Name: "Goblin Racketeer", Count: 4},
		{Name: "Insufferable Balladeer", Count: 4},
		{Name: "Bothersome Quasit", Count: 4}, {Name: "Puppet Master, String Puller", Count: 4},
		{Name: "Baeloth Barrityl, Entertainer", Count: 2},
		{Name: "Grizzly Bears", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 4},
	}
	rani := []cardset.Entry{
		{Name: "The Rani", Count: 4}, {Name: "Taunting Kobold", Count: 4},
		{Name: "Insufferable Balladeer", Count: 4}, {Name: "Bothersome Quasit", Count: 4},
		{Name: "Puppet Master, String Puller", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 4}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 3},
	}
	return [4][]cardset.Entry{redirects, rani, aggro, aggro}
}

// combatantsGoadPod is the fixture set and its four-seat table.
type combatantsGoadPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
}

func newCombatantsGoadPod(t *testing.T) combatantsGoadPod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("combatantsgoad")
	if err != nil {
		t.Fatal(err)
	}
	decks := combatantsGoadDecks()
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return combatantsGoadPod{reg: reg, set: set, tbl: tbl, decks: decks}
}

// combatantsGoadCounts is what a finished gorge log shows the mechanic did.
type combatantsGoadCounts struct {
	redirects int // CombatRetarget events (a moved attack)
}

func countCombatantsGoadEvents(g *GorgeSide) (c combatantsGoadCounts) {
	for _, e := range g.E.L.Events {
		if e.Kind == events.CombatRetarget {
			c.redirects++
		}
	}
	return c
}

func expandCombatantsGoadDeck(d []cardset.Entry) []string {
	var out []string
	for _, e := range d {
		for i := 0; i < e.Count; i++ {
			out = append(out, e.Name)
		}
	}
	return out
}

func isRedirectCarrier(name string) bool {
	return name == "Misleading Signpost" || name == "Portal Mage"
}

// combatantsGoadDebug counts what the steering policy reached; test only.
var combatantsGoadDebug struct{ casts, triggers int }

// combatantsGoadPolicy drives the mechanic deterministically: during the
// declare attackers step it casts a carrier if one is offered, accepts the
// carrier's optional ETB trigger, targets an attacking creature, and answers
// the redirect ask with a seat other than the attacker's current defender
// (the last is what makes the attack move). Every other decision falls back to
// castFirstPolicy, so the rest of the game stays the seeded play.
func combatantsGoadPolicy(seed uint64) Policy {
	base := castFirstPolicy(seed)
	yesCode, _ := chooseCode("yes")
	yesKind := ActChoose + CanonKind(yesCode)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.G.E == nil {
			return base.Pick(c)
		}
		g := c.G.E.G
		for _, o := range c.D.Options {
			if o.Kind() != ActCast && o.Kind() != ActCastFace {
				continue
			}
			opt, ok := c.G.GorgeOption(o)
			if !ok {
				continue
			}
			ob := g.Obj(opt.Obj)
			if ob == nil || ob.Face() == nil || !isRedirectCarrier(ob.Face().Name) {
				continue
			}
			if g.Step == state.StepDeclareAttackers {
				combatantsGoadDebug.casts++
				return o
			}
			// hold the carrier until a declare attackers step, when its
			// trigger can fire
			for _, p := range c.D.Options {
				if p.Kind() == ActPass {
					return p
				}
			}
		}
		if g.Step != state.StepDeclareAttackers {
			return base.Pick(c)
		}
		d := c.G.Pending()
		if d == nil || d.Source == 0 {
			return base.Pick(c)
		}
		src := g.Obj(d.Source)
		if src == nil || src.Face() == nil || !isRedirectCarrier(src.Face().Name) {
			return base.Pick(c)
		}
		switch d.Kind {
		case decision.KTarget:
			for _, o := range c.D.Options {
				if o.Kind() != ActTarget {
					continue
				}
				if opt, ok := c.G.GorgeOption(o); ok {
					if ob := g.Obj(opt.Obj); ob != nil && ob.IsAttacking && ob.Controller == state.PlayerID(g.Active) {
						return o
					}
				}
			}
		case decision.KChoose:
			for _, o := range c.D.Options {
				if o.Kind() == yesKind {
					combatantsGoadDebug.triggers++
					return o // the optional ETB trigger: reselect
				}
			}
			const prefix, suffix = "Choose which player ", " attacks"
			if name := strings.TrimSuffix(strings.TrimPrefix(d.Prompt, prefix), suffix); name != d.Prompt {
				for _, ob := range g.Objs {
					if ob.Face() == nil || ob.Face().Name != name || !ob.IsAttacking {
						continue
					}
					for _, o := range c.D.Options {
						if opt, ok := c.G.GorgeOption(o); ok && opt.Kind == "player" && opt.Player != ob.Attacking {
							return o
						}
					}
				}
			}
		}
		return base.Pick(c)
	})
}

// play plays one seed of the pod. fourSeat=false plays the first two decks
// only (the two-seat form). It returns the lockstep result, the
// gorge-side redirect count, and how many IsGoaded predicate matches the
// kernel made.
func (p combatantsGoadPod) play(t *testing.T, seed uint64, fourSeat bool, maxTurns int32) (Result, combatantsGoadCounts, int64) {
	t.Helper()
	d := p.decks
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{expandCombatantsGoadDeck(d[0]), expandCombatantsGoadDeck(d[1])},
		Seed:  seed, PassElision: true, Surface: "full"}
	if fourSeat {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expandCombatantsGoadDeck(d[2]), expandCombatantsGoadDeck(d[3])}
	}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var c combatantsGoadCounts
	rc.Finish = func(g *GorgeSide) { c = countCombatantsGoadEvents(g) }
	dirac.DebugCountGoadedHits.Store(true)
	defer dirac.DebugCountGoadedHits.Store(false)
	before := dirac.DebugGoadedPredicateHits.Load()
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, combatantsGoadPolicy(seed), rc)
	return res, c, dirac.DebugGoadedPredicateHits.Load() - before
}

func combatantsGoadFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v",
		seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// combatantsGoadSeeds are the four-seat seeds the lockstep plays.
var combatantsGoadSeeds = []uint64{1, 2, 3, 4, 5, 6}

// combatantsGoadSweepSeeds is the four-seat lockstep's wide sweep (seed 30
// once found a divergence the first six seeds missed).
func combatantsGoadSweepSeeds() []uint64 {
	s := make([]uint64, 70)
	for i := range s {
		s[i] = uint64(i + 1)
	}
	return s
}

// TestCombatantsGoadFourSeatLockstep plays the four-seat pod: the goad
// readers must see goaded creatures, and a flash ChangeCombatants carrier
// entering during the declare attackers step must move an attack (one
// CombatRetarget). The lockstep compares the four seats' decisions option by
// option, so the redirect ask's player list and the goad reads must be
// gorge's at every seat. No divergence and no stall.
func TestCombatantsGoadFourSeatLockstep(t *testing.T) {
	p := newCombatantsGoadPod(t)
	combatantsGoadDebug = struct{ casts, triggers int }{}
	var redirects int
	var goadHits int64
	for _, seed := range combatantsGoadSweepSeeds() {
		res, c, hits := p.play(t, seed, true, 40)
		if res.Div != nil {
			combatantsGoadFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		redirects += c.redirects
		goadHits += hits
		t.Logf("seed %d: %d decisions, over %v %s; %d redirect(s), %d IsGoaded match(es); casts %d, triggers %d", seed, res.Decisions, res.Over, res.Stall, c.redirects, hits, combatantsGoadDebug.casts, combatantsGoadDebug.triggers)
	}
	if redirects == 0 {
		t.Fatalf("no attack was redirected: casts %d, triggers %d", combatantsGoadDebug.casts, combatantsGoadDebug.triggers)
	}
	if goadHits == 0 {
		t.Fatal("no IsGoaded predicate matched: the pool does not exercise the word")
	}
}

// TestCombatantsGoadTwoSeatLockstep is the two-seat form: with one living
// opponent, an attacker's only candidate defender is the one it already
// attacks, so ChangeCombatants asks nothing (and emits no CombatRetarget).
// No divergence and no stall.
func TestCombatantsGoadTwoSeatLockstep(t *testing.T) {
	p := newCombatantsGoadPod(t)
	combatantsGoadDebug = struct{ casts, triggers int }{}
	for _, seed := range combatantsGoadSeeds {
		res, c, _ := p.play(t, seed, false, 40)
		if res.Div != nil {
			combatantsGoadFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if c.redirects != 0 {
			t.Fatalf("seed %d: %d two-seat redirect(s), want none (the only candidate is the current defender)", seed, c.redirects)
		}
	}
	if combatantsGoadDebug.casts == 0 {
		t.Fatal("no carrier was cast in a declare attackers step: the no-ask path was never reached")
	}
}

// TestChangeCombatantsMutation is the mutation test: leaving a redirected
// attack on its old defender must diverge on the next combat decision. (The
// KeepBlockers mutation cannot be the lockstep break here: the carriers'
// trigger is during declare attackers, before any blocker exists, so there is
// nothing to keep; the no-move break exercises the same retarget path.)
func TestChangeCombatantsMutation(t *testing.T) {
	p := newCombatantsGoadPod(t)
	dirac.DebugChangeCombatantsNoMove = true
	defer func() { dirac.DebugChangeCombatantsNoMove = false }()
	for _, seed := range combatantsGoadSeeds {
		res, c, _ := p.play(t, seed, true, 40)
		if c.redirects == 0 {
			continue // no redirect in this seed: nothing to mutate
		}
		if res.Div == nil {
			t.Fatalf("seed %d: leaving %d redirected attack(s) on their old defender diverged on no decision", seed, c.redirects)
		}
		t.Logf("mutation seed %d diverged at %d: %s %s %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		return
	}
	t.Fatal("no seed redirected an attack: the mutation has nothing to break")
}

// TestCombatantsGoadPinnedRepro pins seed 6, which casts a carrier during a
// declare attackers step, moves that attack, and evaluates IsGoaded, so a
// green lockstep cannot be a game where nothing happened.
func TestCombatantsGoadPinnedRepro(t *testing.T) {
	p := newCombatantsGoadPod(t)
	res, c, hits := p.play(t, 6, true, 40)
	if res.Div != nil {
		combatantsGoadFail(t, 6, res)
	}
	if c.redirects == 0 || hits == 0 {
		t.Fatalf("seed 6: %d redirect(s), %d IsGoaded match(es): want both above zero", c.redirects, hits)
	}
	t.Logf("seed 6: %d redirect(s), %d IsGoaded match(es)", c.redirects, hits)
}
