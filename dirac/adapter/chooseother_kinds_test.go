package adapter

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
)

// TestChooseOtherAsksReadBackGorgeKinds: the three kernel asks that
// canonicalise as choose:other (ChGenRingBearer, ChGenPair,
// ChMoveCounterAny) read back with gorge's option kind ("ring_bearer",
// effRingTemptsYou; "pair", effPair; "move_counter", the MoveCounter amount
// ask with the count on Amount and as the label), never "other".
func TestChooseOtherAsksReadBackGorgeKinds(t *testing.T) {
	e, c := benchCommander(t)
	for _, tc := range []struct {
		sub  dirac.ChoiceKind
		kind string
	}{
		{dirac.ChGenRingBearer, "ring_bearer"},
		{dirac.ChGenPair, "pair"},
		{dirac.ChMoveCounterAny, "move_counter"},
	} {
		gm, err := e.New(c)
		if err != nil {
			t.Fatal(err)
		}
		g := gm.(*Game)
		g.d = dirac.Decision{Kind: dirac.DkChoose, Sub: tc.sub, Player: 0, N: 3}
		for i := 0; i < 3; i++ {
			g.d.Options[i] = dirac.MakeAction(dirac.AkChoose, 0, 0, uint8(i))
		}
		g.pose()
		d := g.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) != 3 {
			t.Fatalf("%s: posed %+v, want a three-option choose", tc.kind, d)
		}
		for i, o := range d.Options {
			if o.Kind != tc.kind {
				t.Errorf("%s: option %d kind %q", tc.kind, i, o.Kind)
			}
			if tc.kind == "move_counter" && (o.Amount != i || o.Label != strconv.Itoa(i)) {
				t.Errorf("move_counter option %d = amount %d label %q, want %d", i, o.Amount, o.Label, i)
			}
		}
	}
}
