package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// numTimesCastAndAsk casts the named creature with six {G}{W}-capable mana
// floating and drives to its NumTimes announcement ask.
func numTimesCastAndAsk(t *testing.T, name, mana string) (*Engine, *decision.Decision) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, name)}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	tb := findAndMoveToHand(t, e, 0, name)
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	addMana(t, e, 0, mana)
	submitChoices(t, e, castOptionIdx(e.Pending(), "cast", tb))

	var ask *decision.Decision
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_x" {
			ask = d
			break
		}
		if d.Kind == decision.KPriority {
			pass := 0
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			submitChoices(t, e, pass)
			continue
		}
		submitChoices(t, e, 0)
	}
	if ask == nil {
		t.Fatal("the NumTimes announcement ask never appeared")
	}
	return e, ask
}

// TestTranquilFrillbackNumTimesAnnounce pins Announce$ NumTimes with
// AnnounceMax$ 3 on `Cost$ Mana<G\NumTimes>`: the enter trigger asks how many
// times to pay (0..3), announcing 2 charges {G}{G} from the pool, and the
// reflexive charm then offers 2 modes (CharmNum$ Count$TriggerRememberAmount).
func TestTranquilFrillbackNumTimesAnnounce(t *testing.T) {
	t.Parallel()
	e, ask := numTimesCastAndAsk(t, "Tranquil Frillback", "GGGGGG")
	if len(ask.Options) != 4 {
		t.Fatalf("announce options = %d, want 0..3 (4): %+v", len(ask.Options), ask.Options)
	}
	for i, o := range ask.Options {
		if o.Amount != i {
			t.Fatalf("announce option %d has amount %d, want ascending 0..3", i, o.Amount)
		}
	}
	poolBefore := e.G.Players[0].Pool.Total()
	submitChoices(t, e, ask.Options[2].Index)
	activateWindowMana(t, e)
	pay, _ := triggerCostWindowAsk(t, e)
	if pay < 0 {
		t.Fatalf("the {G}{G} payment was not offered: %+v", e.Pending())
	}
	submitChoices(t, e, pay)
	if got := poolBefore - e.G.Players[0].Pool.Total(); got != 2 {
		t.Fatalf("paid %d mana for announcing 2, want 2", got)
	}
	d := passUntilNonPriority(t, e, 40)
	if d.Kind != decision.KModes || d.Max != 2 {
		t.Fatalf("expected a 0..2 mode ask for the charm, got kind=%s min=%d max=%d prompt=%q", d.Kind, d.Min, d.Max, d.Prompt)
	}
}

// TestIntrepidAdversaryNumTimesBindsRememberedAmount pins the no-AnnounceMax
// sibling: announcing 2 of {1}{W} charges four mana and the reflexive
// PutCounter reads the announced count (RememberSVarAmount$ NumTimes).
func TestIntrepidAdversaryNumTimesBindsRememberedAmount(t *testing.T) {
	t.Parallel()
	e, ask := numTimesCastAndAsk(t, "Intrepid Adversary", "WWWWWW")
	if len(ask.Options) < 3 {
		t.Fatalf("announce options = %+v, want at least 0..2", ask.Options)
	}
	submitChoices(t, e, ask.Options[2].Index)
	activateWindowMana(t, e)
	pay, _ := triggerCostWindowAsk(t, e)
	if pay < 0 {
		t.Fatalf("the {1}{W}{1}{W} payment was not offered: %+v", e.Pending())
	}
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 40)
	var id state.ObjID
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == "Intrepid Adversary" {
			id = oid
		}
	}
	if id == 0 {
		t.Fatal("Intrepid Adversary is not on the battlefield")
	}
	if got := e.G.Obj(id).Counter("VALOR"); got != 2 {
		t.Fatalf("valor counters = %d, want 2 (announced 2 times)", got)
	}
}
