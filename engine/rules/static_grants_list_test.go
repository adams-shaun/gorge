package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMaxSpeedAddStaticAbilityListGrantsEach pins an AddStaticAbility$ value
// that names several statics joined by " & " (Tsagan, Raider Warlord:
// `AddStaticAbility$ SelfDT & WideFS`): each named body is granted while the
// outer static is live, as the AddTrigger$ and Class grant paths already do.
// Reading the whole value as one SVar name looked up nothing and granted
// neither.
func TestMaxSpeedAddStaticAbilityListGrantsEach(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bear := card(t, "Name:BigBear\nManaCost:2 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n")
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Tsagan, Raider Warlord"), bear}, nil)
	ts := moveByName(t, e, 0, "Tsagan, Raider Warlord", state.ZBattlefield)
	bearID := moveByName(t, e, 0, "BigBear", state.ZBattlefield)
	if e.G.Obj(ts).Zone != state.ZBattlefield || e.G.Obj(bearID).Zone != state.ZBattlefield {
		t.Fatal("precondition: Tsagan and a creature must be on the battlefield")
	}
	setSpeed(t, e, 3)
	if e.HasKeyword(ts, "Deathtouch") || e.HasKeyword(bearID, "First Strike") {
		t.Fatalf("speed 3: Tsagan deathtouch %v, the bear's first strike %v, want neither", e.HasKeyword(ts, "Deathtouch"), e.HasKeyword(bearID, "First Strike"))
	}
	setSpeed(t, e, maxSpeed)
	if !e.HasKeyword(ts, "Deathtouch") {
		t.Fatal("speed 4: Tsagan lacks deathtouch: the first named static (SelfDT) was never granted")
	}
	if !e.HasKeyword(bearID, "First Strike") {
		t.Fatal("speed 4: the other creature lacks first strike: the second named static (WideFS) was never granted")
	}
	if e.HasKeyword(bearID, "Deathtouch") {
		t.Fatal("speed 4: the other creature has deathtouch, which only Tsagan is granted")
	}
	setSpeed(t, e, 3)
	if e.HasKeyword(ts, "Deathtouch") || e.HasKeyword(bearID, "First Strike") {
		t.Fatal("the grants outlive the speed drop")
	}
}
