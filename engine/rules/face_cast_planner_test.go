package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// faceCastPlan returns the planner's verdict on the cast of id in mode.
func faceCastPlan(t *testing.T, pps []PotentialPlan, id state.ObjID, mode string) PotentialPlan {
	t.Helper()
	for _, pp := range pps {
		if pp.Action.Kind == "cast" && pp.Action.Obj == id && pp.Action.Mode == mode {
			return pp
		}
	}
	t.Fatalf("no potential cast of %d in mode %q: %+v", id, mode, pps)
	return PotentialPlan{}
}

// faceCastOmenSrc is an authored Omen card whose Omen face is cheaper than
// its front, so two untapped Plains pay only the Omen half.
const faceCastOmenSrc = "Name:Planner Omen Front\nManaCost:2 W\nTypes:Sorcery\n" +
	"AlternateMode:Omen\nOracle:front\nALTERNATE\nName:Planner Omen Back\nManaCost:1 W\n" +
	"Types:Sorcery Omen\nA:SP$ Draw | NumCards$ 1\nOracle:back\n"

// TestOmenOfferedFromUntappedLands: the planner witnesses the Omen face's
// {1}{W} from two untapped Plains and an empty pool, so a seat can tap the
// witness and then cast it. The front face's {2}{W} is not payable.
func TestOmenOfferedFromUntappedLands(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 97201, faceCastOmenSrc)
	toMain1(t, e)
	first := onBoard(t, e, 0, lrPlains)
	second := onBoard(t, e, 0, lrPlains)
	ppPriority(t, e)
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	pp := faceCastPlan(t, e.PotentialPaymentPlans(0), id, "omen_alt")
	if pp.Plan == nil || pp.Reason != "" {
		t.Fatalf("omen_alt verdict %+v, want a witness from the untapped Plains", pp)
	}
	if len(pp.Plan.Activations) != 2 {
		t.Fatalf("witness %s, want both Plains", lrPlanString(pp.Plan.Activations))
	}
	for _, a := range pp.Plan.Activations {
		if a.Source != first && a.Source != second {
			t.Fatalf("witness tapped %d, want only the two Plains %d and %d", a.Source, first, second)
		}
	}
}

// TestModalSpellOfferedFromUntappedLands: the nonland back face of a Modal
// DFC is a separate cast, and the planner witnesses its {2}{U}{U} from four
// untapped Islands and an empty pool.
func TestModalSpellOfferedFromUntappedLands(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{corpusAlternativeCard(t, "Kianne, Dean of Substance")}, nil)
	backID := moveByName(t, e, 0, "Kianne, Dean of Substance", state.ZHand)
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	var lands []state.ObjID
	for range 4 {
		lands = append(lands, onBoardCard(t, e, 0, corpusCard(t, "Island")))
	}
	ppPriority(t, e)
	pp := faceCastPlan(t, e.PotentialPaymentPlans(0), backID, "modal_spell")
	if pp.Plan == nil || pp.Reason != "" {
		t.Fatalf("modal_spell verdict %+v, want a witness from the four untapped Islands", pp)
	}
	if len(pp.Plan.Activations) != len(lands) {
		t.Fatalf("witness %s, want all four lands", lrPlanString(pp.Plan.Activations))
	}
	for _, a := range pp.Plan.Activations {
		found := false
		for _, id := range lands {
			found = found || a.Source == id
		}
		if !found {
			t.Fatalf("witness tapped %d, want only the four lands", a.Source)
		}
	}
}

// TestPrepareCastOfferedFromUntappedLands: the exile-resident prepared copy
// of Vicious Verse ({B/R}) is witnessed from one untapped Mountain and an
// empty pool, not only from floating mana.
func TestPrepareCastOfferedFromUntappedLands(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Whiplash Wordsmith")}, nil)
	moveByName(t, e, 0, "Whiplash Wordsmith", state.ZHand)
	addMana(t, e, 0, "BBBB")
	castNamed(t, e, "Whiplash Wordsmith")
	answerHybridPayment(t, e)
	passUntilStackEmpty(t, e, 40)
	copyID := sosPreparedCopy(e, "Vicious Verse")
	if copyID == 0 {
		t.Fatal("precondition: no Vicious Verse prepared copy in exile")
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	mountain := onBoardCard(t, e, 0, corpusCard(t, "Mountain"))
	ppPriority(t, e)
	pp := faceCastPlan(t, e.PotentialPaymentPlans(0), copyID, "prepared_copy")
	if pp.Plan == nil || pp.Reason != "" {
		t.Fatalf("prepared_copy verdict %+v, want a witness from the untapped Mountain", pp)
	}
	if len(pp.Plan.Activations) != 1 || pp.Plan.Activations[0].Source != mountain {
		t.Fatalf("witness %s, want the Mountain %d", lrPlanString(pp.Plan.Activations), mountain)
	}
}

// TestSneakOfferedFromUntappedLands: Oroku Saki's Sneak {1}{B} is witnessed
// from two untapped Swamps and an empty pool at the declare-blockers step;
// the normal {2}{B} cost is not payable from them.
func TestSneakOfferedFromUntappedLands(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	saki := searchCorpusCard(t, reg, "Oroku Saki, Shredder Rising")
	e, _ := ninjutsuDeck(t, 97202, saki)
	sakiID := searchMoveByName(t, e, "Oroku Saki, Shredder Rising", state.ZHand)
	attackWithBear(t, e)
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	first := onBoardCard(t, e, 0, corpusCard(t, "Swamp"))
	second := onBoardCard(t, e, 0, corpusCard(t, "Swamp"))
	ppPriority(t, e)
	pp := faceCastPlan(t, e.PotentialPaymentPlans(0), sakiID, "sneak")
	if pp.Plan == nil || pp.Reason != "" {
		t.Fatalf("sneak verdict %+v, want a witness from the untapped Swamps", pp)
	}
	if len(pp.Plan.Activations) != 2 {
		t.Fatalf("witness %s, want both Swamps", lrPlanString(pp.Plan.Activations))
	}
	for _, a := range pp.Plan.Activations {
		if a.Source != first && a.Source != second {
			t.Fatalf("witness tapped %d, want only the two Swamps %d and %d", a.Source, first, second)
		}
	}
}
