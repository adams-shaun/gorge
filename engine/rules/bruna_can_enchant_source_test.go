package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// brunaAttacksWithCowedOnShieldSphere builds the xproof board: seat 0's
// Bruna, Light of Alabaster attacks while seat 1's Shield Sphere is enchanted
// by Cowed by Wisdom (Enchant creature). Bruna's attack trigger is
// RepeatCards$ Aura.CanEnchantSource+!Attached, so Cowed by Wisdom is an
// eligible battlefield Aura: CanEnchantSource reads "this Aura could legally
// enchant the ability's source", not "it is already attached to the source".
func brunaAttacksWithCowedOnShieldSphere(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Bruna, Light of Alabaster")},
		[]*cards.Card{lookup(t, reg, "Shield Sphere"), lookup(t, reg, "Cowed by Wisdom")})
	bruna := moveCorpusCard(t, e, "Bruna, Light of Alabaster", 0, state.ZBattlefield)
	sphere := moveCorpusCard(t, e, "Shield Sphere", 1, state.ZBattlefield)
	cowed := lossAttach(t, e, "Cowed by Wisdom", 1, sphere)
	if e.G.Obj(bruna).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Bruna is in %v, want the battlefield", e.G.Obj(bruna).Zone)
	}
	if e.G.Obj(sphere).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Shield Sphere is in %v, want the battlefield", e.G.Obj(sphere).Zone)
	}
	if got := e.G.Obj(cowed).AttachedTo; got != sphere {
		t.Fatalf("precondition: Cowed by Wisdom attached to %d, want Shield Sphere %d", got, sphere)
	}
	if bruna == sphere || bruna == cowed {
		t.Fatal("precondition: Bruna, Shield Sphere and Cowed by Wisdom must be distinct objects")
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bruna}})
	e.pending = nil
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 || len(e.pendingTriggers) != 0 {
		t.Fatalf("precondition: Bruna's attack trigger must be on the stack (depth %d, pending %d)", len(e.G.Stack), len(e.pendingTriggers))
	}
	return e, cfg, bruna, sphere, cowed
}

// driveBrunaAttachAsk drives the queued attack trigger to its Optional$
// Attach election and answers it yes or no. It fails if the trigger never
// offers the attach, and declines any later optional ask (the ZoneAuras SVar
// poses one only when an Aura card is in hand/graveyard, which this board
// does not have). The returned decision is the attach ask.
func driveBrunaAttachAsk(t *testing.T, e *Engine, yes bool) *decision.Decision {
	t.Helper()
	var ask *decision.Decision
	for i := 0; i < 120; i++ {
		if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			break
		}
		d := e.Pending()
		if d == nil {
			if len(e.G.Stack) == 0 {
				break
			}
			e.Advance()
			continue
		}
		switch {
		case d.Kind == decision.KPriority:
			submitPass(t, e)
		case d.Kind == decision.KChoose && d.ResumeKind == "attach_optional":
			ask = d
			if yes {
				submitChoices(t, e, 0)
			} else {
				submitChoices(t, e, 1)
			}
		case d.Kind == decision.KChoose:
			// Decline the optional ZoneAuras election and any other
			// optional rider (the board has no hand/graveyard Aura).
			picked := 0
			for _, o := range d.Options {
				if o.Kind == "no" {
					picked = o.Index
				}
			}
			submitChoices(t, e, picked)
		default:
			t.Fatalf("unexpected decision while resolving Bruna's attack trigger: %+v", d)
		}
	}
	if ask == nil {
		t.Fatal("Bruna's attack trigger never offered the Aura attach (CanEnchantSource matched no Aura)")
	}
	return ask
}

// TestBrunaAttachesBattlefieldAura pins Bruna, Light of Alabaster's attack
// trigger (RepeatCards$ Aura.CanEnchantSource+!Attached): an Aura that could
// enchant Bruna moves from another permanent onto her on a yes, and stays put
// on a no. Before CanEnchantSource had a reader the unknown word failed
// closed, so RepeatCards$ matched nothing and the attach was never offered.
func TestBrunaAttachesBattlefieldAura(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		yes  bool
		want func(bruna, sphere state.ObjID) state.ObjID
	}{
		{"accept moves Cowed by Wisdom onto Bruna", true, func(bruna, _ state.ObjID) state.ObjID { return bruna }},
		{"decline leaves Cowed by Wisdom on Shield Sphere", false, func(_, sphere state.ObjID) state.ObjID { return sphere }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg, bruna, sphere, cowed := brunaAttacksWithCowedOnShieldSphere(t)
			ask := driveBrunaAttachAsk(t, e, tc.yes)
			if ask.Min != 1 || ask.Max != 1 || len(ask.Options) != 2 {
				t.Fatalf("attach election = Min %d Max %d %d options, want the 1/1 yes-no ask", ask.Min, ask.Max, len(ask.Options))
			}
			want := tc.want(bruna, sphere)
			if got := e.G.Obj(cowed).AttachedTo; got != want {
				t.Fatalf("Cowed by Wisdom attached to %d, want %d", got, want)
			}
			if e.G.Obj(cowed).Zone != state.ZBattlefield {
				t.Fatalf("Cowed by Wisdom is in %v, want the battlefield", e.G.Obj(cowed).Zone)
			}
			replayCheck(t, e, cfg)
		})
	}
}
