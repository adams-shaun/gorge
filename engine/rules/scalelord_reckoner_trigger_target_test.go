package rules

// xproof gorge-suspect:scalelord-reckoner:f0t0:stack (gorge~forge): Scalelord
// Reckoner's BecomesTarget trigger destroys "target nonland permanent that
// player controls" (ValidTgts$ Permanent.nonLand,
// TargetsWithDefinedController$ TriggeredSourceController). For a
// BecomesTarget trigger the triggered source is the targeting spell or
// ability, so "that player" is the spell's controller. When p0 controls no
// nonland permanent the trigger has no legal target and CR 603.3d removes it
// from the stack; gorge instead offered the Dragon's own permanent, because
// the unknown selector left the candidate list unchanged.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scalelordReckonerEngine seats a Scalelord Reckoner on seat 1's battlefield
// and the named corpus permanents on seat 0's, with three Murders seeded in
// seat 0's deck. Seat 1 is the Dragon's controller, so "that player" is
// whoever targets it.
func scalelordReckonerEngine(t *testing.T, reg *cards.Registry, p0Board []string) (*Engine, map[string]state.ObjID) {
	t.Helper()
	murder, ok := reg.Lookup("Murder")
	if !ok {
		t.Fatal("precondition: Murder is missing from the corpus")
	}
	scalelord, ok := reg.Lookup("Scalelord Reckoner")
	if !ok {
		t.Fatal("precondition: Scalelord Reckoner is missing from the corpus")
	}
	e := New(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{murder, murder, murder}, mountainDeck(t, 40)...),
			mountainDeck(t, 40)}})
	ids := map[string]state.ObjID{"Scalelord Reckoner": onBoardCard(t, e, 1, scalelord)}
	for _, name := range p0Board {
		ids[name] = onBoardCard(t, e, 0, mustCorpusCard(t, reg, name))
	}
	keeper := e.G.Obj(ids["Scalelord Reckoner"])
	if keeper == nil || keeper.Zone != state.ZBattlefield || keeper.Controller != 1 {
		t.Fatal("precondition: Scalelord Reckoner not a battlefield permanent of seat 1")
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o == nil {
			continue
		}
		nonland := false
		for _, ty := range o.Face().Types {
			if ty != "Land" {
				nonland = true
			}
		}
		if len(o.Face().Types) == 0 || nonland {
			t.Fatalf("precondition: seat 0 controls nonland permanent %s", o.Face().Name)
		}
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.askPriority(0)
	return e, ids
}

// castMurderAtReckoner casts Murder from seat 0's hand at the Reckoner, then
// walks the pending decisions until they settle, recording every target ask
// beyond the spell's own announcement ask and what it offered. The spell's
// target is the Reckoner; any later ask is answered with `pickFor` (its first
// option when pickFor is 0). The asks are the observable: with the selector
// unbound gorge offers the Dragon's own permanent, and with no "that player"
// permanent at all the whole ability must never be asked.
func castMurderAtReckoner(t *testing.T, e *Engine, reckoner, pickFor state.ObjID) [][]state.ObjID {
	t.Helper()
	ensureInHand(t, e, 0, "Murder")
	addMana(t, e, 0, "BBB")
	castCardNow(t, e, "Murder")
	d := passToTargetAsk(t, e)
	idx := -1
	for _, o := range d.Options {
		if o.Obj == reckoner {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Murder did not offer the Reckoner %d: %+v", reckoner, d.Options)
	}
	submitChoices(t, e, idx)
	var asks [][]state.ObjID
	for i := 0; i < 60; i++ {
		if len(e.G.Stack) == 0 {
			return asks
		}
		d := e.Pending()
		if d == nil {
			return asks
		}
		switch d.Kind {
		case decision.KTarget:
			var offered []state.ObjID
			for _, o := range d.Options {
				offered = append(offered, o.Obj)
			}
			asks = append(asks, offered)
			choice := -1
			for _, o := range d.Options {
				if pickFor != 0 && o.Obj == pickFor || pickFor == 0 {
					choice = o.Index
					break
				}
			}
			if choice < 0 {
				t.Fatalf("ask did not offer %d: %+v", pickFor, d.Options)
			}
			submitChoices(t, e, choice)
		case decision.KChoose:
			submitChoices(t, e, 0)
		case decision.KPriority:
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if pass < 0 {
				t.Fatalf("no pass option on the priority ask: %+v", d.Options)
			}
			submitChoices(t, e, pass)
		default:
			t.Fatalf("unexpected decision while resolving: %+v", d)
		}
	}
	t.Fatal("decisions did not settle in 60 rounds")
	return nil
}

func TestScalelordReckonerTriggerNeedsThatPlayersPermanent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	scalelord, ok := reg.Lookup("Scalelord Reckoner")
	if !ok {
		t.Fatal("precondition: Scalelord Reckoner is missing from the corpus")
	}
	trig := corpusTriggerMode(t, scalelord, "BecomesTarget")
	sa := cards.ResolveSVar(scalelord.Faces[0].SVars, trig.Params["Execute"])
	if sa == nil || sa.Params["TargetsWithDefinedController"] != "TriggeredSourceController" ||
		sa.Params["ValidTgts"] != "Permanent.nonLand" {
		t.Fatalf("corpus fixture drift: Scalelord Reckoner %s body = %+v, want Destroy with TargetsWithDefinedController$ TriggeredSourceController",
			trig.Params["Execute"], sa)
	}

	t.Run("no nonland permanent that player controls", func(t *testing.T) {
		e, ids := scalelordReckonerEngine(t, reg, nil)
		asks := castMurderAtReckoner(t, e, ids["Scalelord Reckoner"], 0)
		if len(asks) != 0 {
			t.Fatalf("Scalelord trigger raised %d target ask(s) %v; with no nonland permanent that player controls CR 603.3d removes it from the stack",
				len(asks), asks)
		}
		if o := e.G.Obj(ids["Scalelord Reckoner"]); o == nil || o.Zone == state.ZBattlefield {
			t.Fatalf("Murder did not resolve: Reckoner still %+v", o)
		}
	})

	t.Run("that player's creature is the only legal target", func(t *testing.T) {
		e, ids := scalelordReckonerEngine(t, reg, []string{"Prodigal Pyromancer"})
		asks := castMurderAtReckoner(t, e, ids["Scalelord Reckoner"], ids["Prodigal Pyromancer"])
		if len(asks) != 1 {
			t.Fatalf("got %d trigger target ask(s) %v, want exactly one", len(asks), asks)
		}
		offer := asks[0]
		if len(offer) != 1 || offer[0] != ids["Prodigal Pyromancer"] {
			t.Fatalf("trigger offered %v, want only the Pyromancer %d that player controls (the Reckoner itself is the Dragon's, not that player's)",
				offer, ids["Prodigal Pyromancer"])
		}
		if o := e.G.Obj(ids["Prodigal Pyromancer"]); o == nil || o.Zone == state.ZBattlefield {
			t.Fatalf("the chosen permanent was not destroyed: %+v", o)
		}
	})
}
