package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCCosts3Lowers: the Mystery Booster Commander costs-3 spelling family
// in lockstep, one subtest per spelling over the mbccosts3 fixture pool:
//
//   - alt-return: Gush's and Thwart's "return N Islands you control to their
//     owner's hand rather than pay this spell's mana cost" (and Daze's),
//     gorge's AltCostIndex 1 cast and its return pick (kernel cast_altfree.go
//     altReturnCost);
//   - jump-start: Chemister's Insight, Direct Current and Radical Idea cast
//     from the graveyard for the printed cost plus a discarded card, then
//     exiled (kernel kw_jumpstart.go);
//   - surge: Grip of the Roil and Jwar Isle Avenger cast for the surge cost
//     after another spell this turn (kernel kw_surge.go);
//   - court: Court of Ambition's upkeep drain "unless they discard a card"
//     (two cards for a monarch), its Discard<Y/Card> count folded from
//     Count$Monarch.2.1 (gorge effects.UnlessCostResolved; kgen
//     resUnlessDiscard).
//
// Each subtest's probe counts the gorge options its spelling exists for and
// fails when none was taken, so a refused card (the pool would not build) or
// a mechanism the games never reach fails as surely as a divergence.
func TestMBCCosts3Lowers(t *testing.T) {
	t.Parallel()
	isRet := func(o decision.Option, name string) bool {
		return o.Kind == "cast" && o.AltCostIndex == 1 && o.Mode == "" && (name == "Gush" || name == "Thwart" || name == "Daze")
	}
	isJump := func(o decision.Option, _ string) bool { return o.Kind == "cast" && o.Mode == "jumpstart" }
	isSurge := func(o decision.Option, _ string) bool { return o.Kind == "cast" && o.Mode == "surged" }
	isCourtPay := func(o decision.Option, name string) bool {
		return o.Mode == decision.ModeUnlessPay && name == "Court of Ambition"
	}
	// the jump-start cast is offered only when the floating pool pays the
	// printed base (gorge's payment planner plans no jumpstart mode, and
	// under the oracle surfaces mana abilities are no options): float it
	// with a ritual once a jump-start card is in the decider's graveyard
	jumpReady := func(c *PickCtx, o decision.Option, name string) bool {
		if o.Kind != "cast" || o.Mode != "" || name != "Seething Song" && name != "Manamorphose" {
			return false
		}
		for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(c.D.Player)) {
			if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil && mbcJumpStartNames[ob.Face().Name] {
				return true
			}
		}
		return false
	}
	for _, tc := range []struct {
		name  string
		decks [2][]cardset.Entry
		want  func(decision.Option, string) bool
		prep  func(*PickCtx, decision.Option, string) bool
		seeds uint64
		need  int
	}{
		{"alt-return", mbcAltReturnDecks, isRet, nil, 120, 20},
		{"jump-start", mbcJumpStartDecks, isJump, jumpReady, 120, 20},
		{"surge", mbcSurgeDecks, isSurge, nil, 120, 20},
		{"court", mbcCourtDecks, isCourtPay, nil, 120, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			taken := 0
			pol := func(seed uint64) Policy {
				return mbcCosts3Probe(castFirstPolicy(seed), rand.New(rand.NewPCG(seed, 0x3c05)), tc.want, tc.prep, &taken)
			}
			if div := lockstepRunPolicy(t, "mbccosts3", tc.decks, tc.seeds, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %d gorge options of the spelling taken", tc.name, taken)
			if taken < tc.need {
				t.Fatalf("%s: %d options of the spelling taken over %d seeds, want at least %d: the pool does not exercise it", tc.name, taken, tc.seeds, tc.need)
			}
		})
	}
}

// mbcCosts3Probe wraps base: when gorge leads and offers an option want
// accepts, it is taken three times in four, else an option prep (when set)
// accepts two times in three; every taken want option is counted.
func mbcCosts3Probe(base Policy, r *rand.Rand, want func(decision.Option, string) bool, prep func(*PickCtx, decision.Option, string) bool, taken *int) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		var hit, pre []Canon
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			name := mbcOptionName(c, o)
			switch {
			case want(o, name):
				hit = append(hit, x)
			case prep != nil && prep(c, o, name):
				pre = append(pre, x)
			}
		}
		switch {
		case len(hit) > 0 && r.IntN(4) != 0:
			a = hit[r.IntN(len(hit))]
		case len(pre) > 0 && r.IntN(3) != 0:
			a = pre[r.IntN(len(pre))]
		}
		if o, ok := c.G.GorgeOption(a); ok && want(o, mbcOptionName(c, o)) {
			*taken++
		}
		return a
	})
}

// mbcOptionName is the face name of option o's object (the source of an
// unless ask), "" when it has none.
func mbcOptionName(c *PickCtx, o decision.Option) string {
	if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
		return ob.Face().Name
	}
	return ""
}

// mbcAltReturnDecks: Islands to return, bodies for Thwart and Daze to
// counter.
var mbcAltReturnDecks = [2][]cardset.Entry{
	{
		{Name: "Gush", Count: 6}, {Name: "Thwart", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 14}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Thwart", Count: 4}, {Name: "Daze", Count: 4}, {Name: "Gush", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 12}, {Name: "Forest", Count: 8},
	},
}

// mbcJumpStartNames are the pool's Jump-start cards.
var mbcJumpStartNames = map[string]bool{"Chemister's Insight": true, "Direct Current": true, "Radical Idea": true}

// mbcJumpStartDecks: the jump-start spells beside the rituals that float
// their printed bases (Seething Song's {R}{R}{R}{R}{R}, Manamorphose's two
// of any colour).
var mbcJumpStartDecks = [2][]cardset.Entry{
	{
		{Name: "Direct Current", Count: 8}, {Name: "Radical Idea", Count: 4}, {Name: "Chemister's Insight", Count: 2},
		{Name: "Seething Song", Count: 6}, {Name: "Manamorphose", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Direct Current", Count: 6}, {Name: "Radical Idea", Count: 4}, {Name: "Chemister's Insight", Count: 2},
		{Name: "Seething Song", Count: 6}, {Name: "Manamorphose", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 7}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 3},
	},
}

// mbcSurgeDecks: Shock and Radical Idea are the first spell of the turn the
// surge casts follow.
var mbcSurgeDecks = [2][]cardset.Entry{
	{
		{Name: "Grip of the Roil", Count: 6}, {Name: "Jwar Isle Avenger", Count: 6}, {Name: "Shock", Count: 8},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Grip of the Roil", Count: 6}, {Name: "Jwar Isle Avenger", Count: 4}, {Name: "Radical Idea", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
}

// mbcCourtDecks: Court of Ambition (its controller becomes the monarch; an
// attacker that connects takes it, so both the one- and two-card discard are
// asked) against bodies and burn.
var mbcCourtDecks = [2][]cardset.Entry{
	{
		{Name: "Court of Ambition", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Swamp", Count: 14}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "Shock", Count: 6},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 10},
	},
}

// TestMBCJumpStartExileMutation: with the jump-start exile dropped (the
// spell rests in the graveyard as a retrace cast does) some game diverges,
// so the pool resolves jump-start casts and the lockstep compares where
// they go (CR 702.84a).
func TestMBCJumpStartExileMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugJumpStartNoExile = true
	defer func() { dirac.DebugJumpStartNoExile = false }()
	if lockstepDiverges(t, "mbccosts3", mbcJumpStartDecks, 60) {
		return
	}
	t.Fatal("no seed diverged with the jump-start exile dropped")
}

// TestMBCSurgeGateMutation: with the "another spell this turn" gate dropped
// the kernel offers a surged cast gorge does not (CR 702.117a), so some
// game diverges.
func TestMBCSurgeGateMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSurgeAnyTurn = true
	defer func() { dirac.DebugSurgeAnyTurn = false }()
	if lockstepDiverges(t, "mbccosts3", mbcSurgeDecks, 60) {
		return
	}
	t.Fatal("no seed diverged with the surge gate dropped")
}
