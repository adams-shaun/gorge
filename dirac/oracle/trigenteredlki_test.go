package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The departed-entrant lockstep (matchTrigLKI): Ensign Harry Kim, Operations
// triggers on a bigger creature entering, and checks again on resolution
// whether it still has greater power (CR 603.4). Viscera Seer sacrifices
// the entrant with the trigger on the stack, and a Glorious Anthem
// makes its last known power differ from its printed one (CR 608.2h). The
// expected answer at every decision is gorge's.
var ktrigEnteredLKIDecks = [2][]cardset.Entry{
	{
		{Name: "Ensign Harry Kim, Operations", Count: 4}, {Name: "Glorious Anthem", Count: 4}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Viscera Seer", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Ensign Harry Kim, Operations", Count: 4}, {Name: "Glorious Anthem", Count: 4}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Viscera Seer", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 5},
	},
}

func TestTrigEnteredLKILockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "trigenteredlki", ktrigEnteredLKIDecks, 400, "KTRIGENTEREDLKI_SEEDS")
}

// TestTrigEnteredLKIResponseLockstep scripts the case the random games
// rarely reach: with Ensign Kim's trigger on the stack, Viscera Seer
// sacrifices the newest Grizzly Bears it may be about, under a Glorious
// Anthem (a 3/3 whose printed power, 2, is not greater than Kim's 2). Every other
// decision is the seeded random policy's.
func TestTrigEnteredLKIResponseLockstep(t *testing.T) {
	t.Parallel()
	fired, div := ktrigEnteredLKIRun(t, 200)
	if fired == 0 {
		t.Fatal("no seed sacrificed an entrant with Kim's trigger on the stack")
	}
	if div != "" {
		t.Fatal(div)
	}
	t.Logf("%d seeds answered the entrant in response, no divergence", fired)
}

// TestTrigEnteredLKILockstepMutation: with a departed trigger object read as
// the card in its new zone, its printed power, some scripted seed diverges.
func TestTrigEnteredLKILockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugTrigLKIOff = true
	defer func() { dirac.DebugTrigLKIOff = false }()
	if _, div := ktrigEnteredLKIRun(t, 200); div == "" {
		t.Fatal("no seed diverged with the departed trigger object read off its card")
	}
}

// ktrigEnteredLKIRun plays seeds 1..n under ktrigEnteredLKIPolicy and returns
// how many seeds sacrificed an entrant in response, and the first divergence.
func ktrigEnteredLKIRun(t *testing.T, n uint64) (fired int, div string) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigenteredlki")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range ktrigEnteredLKIDecks {
		for _, e := range ktrigEnteredLKIDecks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(ktrigEnteredLKIDecks[0], ktrigEnteredLKIDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"kim-a", "kim-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		p := &ktrigEnteredLKIPolicy{rnd: RandomPolicy(seed)}
		res := Lockstep(m, seed, k, p, RunConfig{MaxDecisions: 20000, MaxTurns: 40})
		CloseSide(k)
		if p.sacs > 0 {
			fired++
		}
		if res.Div != nil && div == "" {
			div = fmt.Sprintf("seed %d: diverged at decision %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
	}
	return fired, div
}

// ktrigEnteredLKIPolicy sacrifices the newest Grizzly Bears of Kim's
// controller to its Viscera Seer while Kim's trigger is the top of the stack
// and that player has a Glorious Anthem out, once per trigger.
type ktrigEnteredLKIPolicy struct {
	rnd  Policy
	want state.ObjID
	done state.ObjID
	sacs int
}

func (p *ktrigEnteredLKIPolicy) Pick(c *PickCtx) Canon {
	g := c.G.E.G
	if c.D.Kind == DMulligan {
		return pickKind(c, ActKeep)
	}
	if p.want != 0 {
		ref := c.G.InstRef(p.want)
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() != ActPass && (a.Tgt() == ref || a.Src() == ref) }); ok {
			p.want = 0
			return a
		}
	}
	if c.D.Kind != DPriority {
		return p.rnd.Pick(c)
	}
	if len(g.Stack) == 0 {
		// a Viscera Seer first, so the response has its outlet
		me := state.PlayerID(c.D.Player)
		if !ktrigHas(g, "Viscera Seer", me) {
			for i := range g.Objs {
				o := &g.Objs[i]
				if o.Zone != state.ZHand || o.Owner != me || o.Face() == nil || o.Face().Name != "Viscera Seer" {
					continue
				}
				ref := c.G.InstRef(o.ID)
				if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast && a.Src() == ref }); ok {
					return a
				}
			}
		}
		return p.rnd.Pick(c)
	}
	top := g.Obj(g.Stack[len(g.Stack)-1])
	if top == nil || top.ID == p.done || top.Ability == nil {
		return p.rnd.Pick(c)
	}
	src := g.Obj(top.Source)
	if src == nil || src.Face() == nil || src.Face().Name != "Ensign Harry Kim, Operations" ||
		state.PlayerID(c.D.Player) != src.Controller || !ktrigHas(g, "Glorious Anthem", src.Controller) {
		return p.rnd.Pick(c)
	}
	var bear, seer *state.Object
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Controller != src.Controller || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Grizzly Bears":
			if bear == nil || o.ZoneGen > bear.ZoneGen {
				bear = o
			}
		case "Viscera Seer":
			seer = o
		}
	}
	if bear == nil || seer == nil {
		return p.rnd.Pick(c)
	}
	ref := c.G.InstRef(seer.ID)
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActActivate && a.Src() == ref }); ok {
		p.done, p.want = top.ID, bear.ID
		p.sacs++
		return a
	}
	return p.rnd.Pick(c)
}

func ktrigHas(g *state.Game, name string, ctrl state.PlayerID) bool {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone == state.ZBattlefield && o.Controller == ctrl && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}
