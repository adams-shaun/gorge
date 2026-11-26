package rules

// Prowling Geistcatcher: "Whenever you sacrifice another creature, exile it.
// If that creature was a token, put a +1/+1 counter on CARDNAME."
// The "was a token" check is judged on last-known information (CR 608.2h):
// a nontoken creature sacrificed to its own ability gets no counter even
// though its ChangeZone sub has already exiled the card by the time the
// ConditionDefined$ TriggeredCardLKICopy sub runs.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sacSpiritSrc is a freely-authored 1/1 Spirit with the same self-sacrifice
// ability shape as Kami of False Hope, minted as a real token for the token
// half of the test.
const sacSpiritSrc = "Name:Sac Spirit\nManaCost:0\nTypes:Creature Spirit\nPT:1/1\n" +
	"A:AB$ Fog | Cost$ Sac<1/CARDNAME> | SpellDescription$ x\nOracle:x\n"

func geistCounterN(t *testing.T, e *Engine, id state.ObjID) int {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil {
		t.Fatal("precondition: Prowling Geistcatcher is gone")
	}
	n := 0
	for _, c := range o.Counters {
		if c.Kind == "P1P1" {
			n += int(c.N)
		}
	}
	return n
}

// runSelfSacrifice activates the self-sacrifice ability of src and drains
// the stack and trigger queue.
func runSelfSacrifice(t *testing.T, e *Engine, src state.ObjID) {
	t.Helper()
	e.pending = nil
	e.priorityRound()
	opt := abilityOption(t, e, src, 0)
	submitChoices(t, e, opt.Index)
	drainSetMechanic(t, e, firstOption)
}

func TestProwlingGeistcatcherCounterOnlyForTokens(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		token     bool
		wantCount int
	}{
		{name: "nontoken Kami of False Hope sacrificed to itself: no counter", token: false, wantCount: 0},
		{name: "token Sac Spirit sacrificed to itself: one counter", token: true, wantCount: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := testutil.CorpusRegistry(t)
			geist := lookup(t, reg, "Prowling Geistcatcher")
			kami := lookup(t, reg, "Kami of False Hope")
			e := corpusEngine(t, reg, []*cards.Card{geist, kami}, nil)
			geistID := addToBattlefield(t, e, geist, 0)
			if o := e.G.Obj(geistID); o == nil || o.Zone != state.ZBattlefield {
				t.Fatal("precondition: Prowling Geistcatcher is not on the battlefield")
			}
			if n := geistCounterN(t, e, geistID); n != 0 {
				t.Fatalf("precondition: Geistcatcher already has %d P1P1 counters", n)
			}

			var srcID state.ObjID
			if tc.token {
				srcID = putToken(t, e, 0, sacSpiritSrc, state.ZBattlefield)
			} else {
				srcID = addToBattlefield(t, e, kami, 0)
			}
			o := e.G.Obj(srcID)
			if o == nil || o.Zone != state.ZBattlefield {
				t.Fatal("precondition: the sacrificed creature is not on the battlefield")
			}

			runSelfSacrifice(t, e, srcID)

			if o := e.G.Obj(srcID); o != nil && o.Zone == state.ZBattlefield {
				t.Fatal("precondition: the creature was not sacrificed")
			}
			if got := geistCounterN(t, e, geistID); got != tc.wantCount {
				t.Fatalf("Geistcatcher P1P1 counters after sacrificing %s = %d, want %d",
					map[bool]string{true: "token", false: "nontoken"}[tc.token], got, tc.wantCount)
			}
		})
	}
}
