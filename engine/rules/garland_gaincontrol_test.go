package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Garland, Royal Kidnapper's second ability ("Whenever an opponent becomes
// the monarch, gain control of target creature that player controls for as
// long as they're the monarch"): the DB$ Effect's StaticAbilities$ GainControl
// static is delivered for real -- while the remembered opponent is the
// monarch the targeted creature is controlled by Garland's controller, and
// when the monarchy moves away the creature returns to its controller (the
// static's CheckSVar$ X over PlayerCountRememberedPlayer$ HasPropertyisMonarch
// flips to 0 and the tracked grant ends through the static-control reconcile).
func TestGarlandStealsTheMonarchsCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Garland, Royal Kidnapper")},
		[]*cards.Card{lookup(t, reg, "Grizzly Bears")})
	bears := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	garland := moveByName(t, e, 0, "Garland, Royal Kidnapper", state.ZHand)
	addMana(t, e, 0, "UUBB")
	cast := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == garland {
			cast = o.Index
		}
	}
	if cast < 0 {
		t.Fatalf("Garland is not castable: %+v", e.Pending().Options)
	}
	submitChoices(t, e, cast)
	// Resolve the spell and both triggers, choosing the first target each
	// time, until nothing is on the stack.
	for range 60 {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			passOnce(t, e)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if e.G.Obj(garland).Zone != state.ZBattlefield || !e.G.IsMonarch(1) {
		t.Fatalf("precondition: Garland on battlefield %v, seat 1 monarch %v",
			e.G.Obj(garland).Zone, e.G.IsMonarch(1))
	}
	if e.G.Obj(bears).Controller != 0 {
		t.Fatalf("the monarch's bear is controlled by seat %d, want seat 0 (Garland's controller stole it)",
			e.G.Obj(bears).Controller)
	}
	// The monarchy moves to Garland's controller: the remembered player is no
	// longer the monarch, the gate reads 0 and the bear returns.
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	for range 30 {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		passOnce(t, e)
	}
	if got := e.G.Obj(bears).Controller; got != 1 {
		t.Fatalf("after the monarchy moved, the bear is controlled by seat %d, want seat 1", got)
	}
	replayCheck(t, e, cfg)
}
