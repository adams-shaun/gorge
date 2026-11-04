package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGrantedAbilityReadsGrantorSVars: Food Fight gives each artifact its
// controller controls a sacrifice ability dealing 1 plus the number of Food
// Fights that player controls. The amount is an SVar of Food Fight, not of
// the artifact, and the ability exists on the stack apart from both (CR
// 113.7a), so with one Food Fight it deals 2. gorge read the recipient's
// (Prophetic Prism's) table, found no such name and fell back to the
// ability's own paid X, dealing 0. An X spell below it (Stonesplitter Bolt
// announced with X=3) must not lend its X either.
func TestGrantedAbilityReadsGrantorSVars(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, _ := edrBoard(t, reg, 81, map[string]state.Zone{
		"Food Fight":         state.ZBattlefield,
		"Prophetic Prism":    state.ZBattlefield,
		"Stonesplitter Bolt": state.ZHand,
		"Grizzly Bears":      state.ZBattlefield,
	})
	addMana(t, e, 0, "RRRRRRRR")
	edrSeatZeroPriority(t, e)
	// play answers priority with the option labelled want, then every other
	// ask with X = 3 or the opposing player, until priority returns.
	play := func(want string) {
		t.Helper()
		for i := 0; i < 12; i++ {
			d := e.Pending()
			if d == nil {
				t.Fatal("no decision pending")
			}
			if d.Kind == decision.KPriority {
				if want == "" {
					return
				}
				found := false
				for _, o := range d.Options {
					if strings.HasPrefix(o.Label, want) {
						submitChoices(t, e, o.Index)
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("no priority option starting %q", want)
				}
				want = ""
				continue
			}
			pick := len(d.Options) - 1
			for _, o := range d.Options {
				if o.Label == "X = 3" || o.Label == "b" {
					pick = o.Index
				}
			}
			submitChoices(t, e, pick)
		}
		t.Fatalf("priority never returned after %q", want)
	}
	play("Cast Stonesplitter Bolt")
	play("Prophetic Prism: It deals damage")
	if got := len(e.G.Stack); got < 2 {
		t.Fatalf("stack holds %d objects, want the bolt and the granted ability", got)
	}
	before := e.G.Players[1].Life
	passPriority(t, e)
	passPriority(t, e)
	if got := e.G.Players[1].Life; got != before-2 {
		t.Fatalf("the granted ability left the opponent at %d life, want %d (1 plus one Food Fight)", got, before-2)
	}
	replayCheck(t, e, cfg)
}
