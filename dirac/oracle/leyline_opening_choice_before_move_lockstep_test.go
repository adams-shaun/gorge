package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 103.6a / 614.12: a Leyline begun from the opening hand is moved to the
// battlefield as part of its "as this enters" replacement, and the choice
// that replacement makes is made while the card is still in the opening
// hand. gorge asks there; the kernel used to move it first and ask after the
// opening round, so the seat, the hand count and the turn all differed one
// decision after "opening_yes".

// TestLeylineOpeningChoiceBeforeMoveReprosLockstep replays the recorded
// std-dsk game whose first divergence was Cards[57].Zone: gorge held Leyline
// of Transformation in hand and asked its creature type; the kernel had it on
// the battlefield and had begun turn 1.
func TestLeylineOpeningChoiceBeforeMoveReprosLockstep(t *testing.T) {
	stdReproNamed(t, "leyline_opening_choice_before_move")
}

// TestLeylineOpeningChoiceBeforeMoveMutation restores the move-before-ask
// timing: the same recording must diverge again, so the repro test above
// cannot pass without the mechanism.
func TestLeylineOpeningChoiceBeforeMoveMutation(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "leyline_opening_choice_before_move.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugLeylineMoveBeforeAsk
	dirac.DebugLeylineMoveBeforeAsk = true
	defer func() { dirac.DebugLeylineMoveBeforeAsk = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatalf("the mutation moved the Leyline before its choice in %d decisions with no divergence", res.Decisions)
	}
	t.Logf("mutation diverged at decision %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}

// leylineOpeningDeck is an eight-card deck whose 7-card opening hand is
// every card but the last one drawn: two Leylines of Resonance and three
// Leyline of Transformation cannot both be missed (the first seat), and
// three Leyline of Transformation survive any one-card omission (the rest),
// so every seat's opening hand holds the card the test names.
func leylineOpeningDeck(resonance bool) []string {
	d := make([]string, 0, 8)
	if resonance {
		d = append(d, "Leyline of Resonance", "Leyline of Resonance")
	}
	for range 3 {
		d = append(d, "Leyline of Transformation")
	}
	lands := 5
	if resonance {
		lands = 3
	}
	for range lands {
		d = append(d, "Island")
	}
	return d
}

// leylineOpeningYesPolicy answers every begin-the-game offer with yes (the
// branch the timing fix is about), else plays randomly.
func leylineOpeningYesPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	yes, ok := chooseCode("opening_yes")
	if !ok {
		panic("no opening_yes choose code")
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActChoose+CanonKind(yes) {
				return o
			}
		}
		return rnd.Pick(c)
	})
}

// leylineOpeningObs counts the as-enters type ask posed on a Leyline that the
// opening round has offered and not yet moved: the offered object must still
// be in its owner's hand when the ask is posed (CR 614.12), and every seat's
// opening hand must hold the card the test names.
type leylineOpeningObs struct {
	seats       int
	withLeyline int
	withReson   int
	offered     state.ObjID // the last opening offer's object
	typeInHand  int         // type asks posed with that object still in hand
	typeMoved   int         // type asks posed after that object left the hand
}

// isTypeAsk reports a creature-type choice (the ask Leyline of Transformation
// opens with; its options carry the "type" kind).
func isTypeAsk(d *decision.Decision) bool {
	return d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "type"
}

func (o *leylineOpeningObs) check(g *GorgeSide, _ Side) *Divergence {
	if o.seats == 0 {
		for p := range g.E.G.Players {
			o.seats++
			var ley, reson bool
			for _, id := range g.E.G.Zone(state.ZHand, state.PlayerID(p)) {
				ob := g.E.G.Obj(id)
				if ob == nil || ob.Face() == nil {
					continue
				}
				switch ob.Face().Name {
				case "Leyline of Transformation":
					ley = true
				case "Leyline of Resonance":
					reson = true
				}
			}
			if ley {
				o.withLeyline++
			}
			if reson {
				o.withReson++
			}
		}
	}
	d := g.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
		return nil
	}
	if k := d.Options[0].Kind; k == "opening_yes" || k == "opening_no" {
		o.offered = d.Options[0].Obj
		return nil
	}
	if isTypeAsk(d) && o.offered != 0 {
		if ob := g.E.G.Obj(o.offered); ob != nil && ob.Zone == state.ZHand {
			o.typeInHand++
		} else {
			o.typeMoved++
		}
		o.offered = 0
	}
	return nil
}

// TestLeylineOpeningChoiceBeforeMoveFourSeat plays four-seat std-dsk pods in
// which every opening hand holds Leyline of Transformation and one seat also
// holds Leyline of Resonance: every seat answers yes, the type ask is posed
// while the offered card is still in hand, and no seed diverges. The count
// makes the test fail if the ask is never reached (or only reached after the
// move).
func TestLeylineOpeningChoiceBeforeMoveFourSeat(t *testing.T) {
	corpusDirOrSkip(t)
	decks := [4][]string{
		leylineOpeningDeck(true),
		leylineOpeningDeck(false),
		leylineOpeningDeck(false),
		leylineOpeningDeck(false),
	}
	obs := &leylineOpeningObs{}
	play := fourSeatSetPlay(t, cardsettest.Synth(t, "std-dsk"), decks, nil, obs.check,
		func(*GorgeSide) {
			if obs.seats != 4 {
				t.Errorf("the pod has %d seats, want 4", obs.seats)
			}
			if obs.withLeyline != 4 {
				t.Errorf("%d of 4 opening hands held Leyline of Transformation", obs.withLeyline)
			}
			if obs.withReson != 1 {
				t.Errorf("%d opening hands held Leyline of Resonance, want 1", obs.withReson)
			}
		},
		leylineOpeningYesPolicy)
	for seed := uint64(1); seed <= 20; seed++ {
		before := obs.typeInHand
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\n%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if obs.typeInHand == before {
			t.Fatalf("seed %d: no type ask was posed with the offered Leyline still in hand (%d posed after it moved)", seed, obs.typeMoved)
		}
	}
	t.Logf("%d seeds: %d type asks while the Leyline was in hand, %d after it moved", 20, obs.typeInHand, obs.typeMoved)
}
