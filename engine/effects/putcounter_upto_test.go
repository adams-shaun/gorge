package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// upToHost serves one scripted "counter_upto" answer (an Amount) through the
// ask seam; with none, the ask is observed and the R-9 stand-in applies.
type upToHost struct {
	*fakeHost
	answer *int
	asks   []*decision.Decision
}

func (h *upToHost) AskCount() uint64 { return 0 }

func (h *upToHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if d.ResumeKind != "counter_upto" {
		return decision.Intent{}, false
	}
	h.asks = append(h.asks, d)
	if h.answer == nil {
		return decision.Intent{}, false
	}
	for _, o := range d.Options {
		if o.Amount == *h.answer {
			return decision.Intent{Choices: []int{o.Index}}, true
		}
	}
	return decision.Intent{}, false
}

func upToSaga(t *testing.T, ans *int) (*upToHost, state.ObjID, *Ctx) {
	t.Helper()
	card, sa := corpusSA(t, "Terra, Magical Adept", "DBPutCounter")
	_ = card
	h := &upToHost{fakeHost: newHost(t, 2), answer: ans}
	saga := h.g.AddObject(mkCard(t, "Name:Test Saga\nTypes:Enchantment Saga\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: saga.ID, From: state.ZLibrary, To: state.ZBattlefield})
	ctx := &Ctx{Controller: 0, Remembered: []state.Target{{Obj: saga.ID}}}
	Resolve(h, ctx, sa)
	return h, saga.ID, ctx
}

// TestPutCounterUpToNoAnswerPlacesTheMaximum: with no answer served (the
// no-host R-9 stand-in) UpTo$ keeps today's behaviour, the full CounterNum$.
func TestPutCounterUpToNoAnswerPlacesTheMaximum(t *testing.T) {
	h, saga, _ := upToSaga(t, nil)
	if len(h.asks) != 1 {
		t.Fatalf("asks = %d, want the one up-to amount ask", len(h.asks))
	}
	if got := h.g.Obj(saga).Counter("LORE"); got != 3 {
		t.Fatalf("LORE = %d, want 3", got)
	}
}

// TestPutCounterUpToAnswerIsHonoured: the served answer is the amount placed,
// and the options run max-first so the bot's first-offer pick is the maximum.
func TestPutCounterUpToAnswerIsHonoured(t *testing.T) {
	for _, want := range []int{0, 2} {
		ans := want
		h, saga, _ := upToSaga(t, &ans)
		if len(h.asks) != 1 || h.asks[0].Options[0].Amount != 3 || len(h.asks[0].Options) != 4 {
			t.Fatalf("ask options = %+v, want 3,2,1,0", h.asks)
		}
		if got := h.g.Obj(saga).Counter("LORE"); got != int32(want) {
			t.Fatalf("LORE after answering %d = %d", want, got)
		}
	}
}
