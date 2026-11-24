package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Tests for FocusPolicy (design scripted-scenario-starts, section 4, ticket
// T5): it picks a wanted key when offered, passes through to (turn, phase),
// defers to base otherwise, and is deterministic over a replayed action list.

const (
	fpSeat  = 0
	fpFocus = Ref(5) // the focus instance
	fpOther = Ref(9) // a card that is not the focus
)

// fpClock is a settable clock.
type fpClock struct {
	turn int
	step state.Step
	ok   bool
}

func (k *fpClock) read(*PickCtx) (int, state.Step, bool) { return k.turn, k.step, k.ok }

// fpBase answers the last option and counts its calls: a pick the focus rules
// never make, so a deferral is visible.
type fpBase struct{ calls int }

func (b *fpBase) Pick(c *PickCtx) Canon {
	b.calls++
	return c.D.Options[len(c.D.Options)-1]
}

func fpDecision(kind DKind, seat uint8, opts ...Canon) *CDecision {
	sortCanon(opts)
	return &CDecision{Kind: kind, Player: seat, Options: opts}
}

func fpPick(p Policy, d *CDecision) Canon { return p.Pick(&PickCtx{D: d}) }

var (
	fpPass      = Pack(ActPass, RefNone, RefNone, 0)
	fpDone      = Pack(ActDone, RefNone, RefNone, 0)
	fpActivate1 = Pack(ActActivate, fpFocus, RefNone, 1)
	fpActivate2 = Pack(ActActivate, fpFocus, RefNone, 2)
	fpCastOther = Pack(ActCast, fpOther, RefNone, 0)
)

func fpSpec() FocusSpec {
	return FocusSpec{Seat: fpSeat, Refs: []Ref{fpFocus}, Keys: []string{"f0/a1", "f0/t0"}}
}

func TestFocusPolicy(t *testing.T) {
	t.Run("picks the wanted key when offered", func(t *testing.T) {
		base := &fpBase{}
		p := newFocusPolicy(base, fpSpec(), (&fpClock{}).read)
		d := fpDecision(DPriority, fpSeat, fpPass, fpActivate1, fpActivate2, fpCastOther)
		if got := fpPick(p, d); got != fpActivate1 {
			t.Fatalf("picked %#x, want the wanted key's activation %#x", got, fpActivate1)
		}
		if base.calls != 0 {
			t.Fatalf("base consulted %d times for an offered key", base.calls)
		}
		// The same mode on a card that is not the focus is not the key.
		other := Pack(ActActivate, fpOther, RefNone, 1)
		if got := fpPick(p, fpDecision(DPriority, fpSeat, fpPass, other)); got != other || base.calls != 1 {
			t.Fatalf("picked %#x (base calls %d), want base's answer %#x", got, base.calls, other)
		}
	})

	t.Run("a key is picked at most three times a game", func(t *testing.T) {
		base := &fpBase{}
		p := newFocusPolicy(base, fpSpec(), (&fpClock{}).read)
		d := fpDecision(DPriority, fpSeat, fpPass, fpActivate1)
		for i := 0; i < focusKeyMaxPicks; i++ {
			if got := fpPick(p, d); got != fpActivate1 {
				t.Fatalf("pick %d: %#x, want the key", i, got)
			}
		}
		if base.calls != 0 {
			t.Fatalf("base consulted %d times before the cap", base.calls)
		}
		// Past the cap base answers; it takes the last option, the activation.
		d = fpDecision(DPriority, fpSeat, fpActivate1, fpPass)
		if got := fpPick(p, d); got != d.Options[1] || base.calls != 1 {
			t.Fatalf("past the cap: picked %#x with base calls %d, want base's %#x", got, base.calls, d.Options[1])
		}
	})

	t.Run("passes through to the turn and phase", func(t *testing.T) {
		clk := &fpClock{turn: 1, step: state.StepUpkeep, ok: true}
		spec := fpSpec()
		spec.Turn, spec.Phase = 2, "main2"
		base := &fpBase{}
		p := newFocusPolicy(base, spec, clk.read)
		d := fpDecision(DPriority, fpSeat, fpActivate1, fpPass, fpCastOther)
		for _, at := range []struct {
			turn int
			step state.Step
			pass bool
		}{
			{1, state.StepUpkeep, true},
			{1, state.StepMain2, true},   // an earlier turn's later phase is not it
			{2, state.StepMain1, true},   // the turn, before the phase
			{2, state.StepMain2, false},  // reached
			{2, state.StepEnd, false},    // past the phase
			{3, state.StepUpkeep, false}, // a later turn
		} {
			clk.turn, clk.step = at.turn, at.step
			got := fpPick(p, d)
			if at.pass && got != fpPass {
				t.Fatalf("turn %d step %v: picked %#x, want the pass", at.turn, at.step, got)
			}
			if !at.pass && got != fpActivate1 {
				t.Fatalf("turn %d step %v: picked %#x, want the key %#x", at.turn, at.step, got, fpActivate1)
			}
		}
		if base.calls != 0 {
			t.Fatalf("base consulted %d times", base.calls)
		}
		clk.turn, clk.step = 1, state.StepUpkeep
		// Without a pass the pass-through takes Done, else the first option.
		if got := fpPick(newFocusPolicy(base, spec, clk.read), fpDecision(DChoose, fpSeat, fpCastOther, fpDone)); got != fpDone {
			t.Fatalf("no pass offered: picked %#x, want Done", got)
		}
		d = fpDecision(DTarget, fpSeat, fpCastOther, fpActivate1)
		if got := fpPick(newFocusPolicy(base, spec, clk.read), d); got != d.Options[0] {
			t.Fatalf("no pass or Done: picked %#x, want the first option %#x", got, d.Options[0])
		}
		// A clock that cannot tell counts the point as reached.
		clk.ok = false
		if got := fpPick(newFocusPolicy(base, spec, clk.read), fpDecision(DPriority, fpSeat, fpPass, fpActivate1)); got != fpActivate1 {
			t.Fatalf("no clock: picked %#x, want the key", got)
		}
	})

	t.Run("defers to base", func(t *testing.T) {
		clk := &fpClock{ok: true}
		spec := fpSpec()
		spec.Turn, spec.Phase = 1, "main1"
		clk.turn, clk.step = 1, state.StepMain1
		base := &fpBase{}
		p := newFocusPolicy(base, spec, clk.read)
		// Nothing of the focus is offered: base answers.
		d := fpDecision(DPriority, fpSeat, fpPass, fpCastOther)
		if got := fpPick(p, d); got != d.Options[1] || base.calls != 1 {
			t.Fatalf("picked %#x with base calls %d, want base's %#x", got, base.calls, d.Options[1])
		}
		// The other seat is base's, even before the pass-through point and
		// with a focus action offered to it.
		clk.turn, clk.step = 1, state.StepUpkeep
		d = fpDecision(DPriority, 1, fpPass, fpActivate1)
		if got := fpPick(p, d); got != d.Options[1] || base.calls != 2 {
			t.Fatalf("other seat: picked %#x with base calls %d, want base's %#x", got, base.calls, d.Options[1])
		}
		// A t key is not picked directly, and an activation of another mode is not a key.
		clk.step = state.StepMain1
		d = fpDecision(DPriority, fpSeat, fpPass, fpActivate2)
		if got := fpPick(p, d); got != d.Options[1] || base.calls != 3 {
			t.Fatalf("unwanted mode: picked %#x with base calls %d, want base's", got, base.calls)
		}
	})

	t.Run("rules in order", func(t *testing.T) {
		special := Pack(ActSpecial, fpFocus, RefNone, 0)
		castHand := Pack(ActCast, fpFocus, RefNone, 0)
		castGrave := Pack(ActCast, fpFocus, RefNone, 2) // flashback
		castFace := Pack(ActCastFace, fpFocus, RefNone, 3)
		land := Pack(ActPlayLand, fpFocus, RefNone, 0)
		attack := Pack(ActAttack, fpFocus, RefPlayer+1, 0)
		otherAttack := Pack(ActAttack, fpOther, RefPlayer+1, 0)
		yes := Pack(ActYes, fpFocus, RefNone, 0)
		clk := &fpClock{ok: true}
		for _, step := range []struct {
			name string
			kind DKind
			opts []Canon
			want Canon
		}{
			{"key over all", DPriority, []Canon{fpPass, special, castHand, land, fpActivate1}, fpActivate1},
			{"special over casts", DPriority, []Canon{fpPass, special, castHand, castFace, land}, special},
			{"cast over lands", DPriority, []Canon{fpPass, castHand, castFace, land}, castHand},
			{"a face cast is a cast", DPriority, []Canon{fpPass, castFace, land}, castFace},
			{"a cast from another zone is not", DPriority, []Canon{fpPass, castGrave, land}, land},
			{"land", DPriority, []Canon{fpPass, land}, land},
			{"focus attacker", DAttackers, []Canon{otherAttack, attack, fpDone}, attack},
			{"yes", DTriggerOptional, []Canon{yes, Pack(ActNo, fpFocus, RefNone, 0)}, yes},
			{"another card's yes is not", DTriggerOptional, []Canon{Pack(ActYes, fpOther, RefNone, 0), Pack(ActNo, fpOther, RefNone, 0)}, 0},
		} {
			base := &fpBase{}
			p := newFocusPolicy(base, fpSpec(), clk.read)
			d := fpDecision(step.kind, fpSeat, step.opts...)
			want := step.want
			if want == 0 {
				want = d.Options[len(d.Options)-1] // base's
			}
			if got := fpPick(p, d); got != want {
				t.Errorf("%s: picked %#x, want %#x", step.name, got, want)
			}
		}
		// An attack of a focus creature is only picked at an attackers
		// decision.
		base := &fpBase{}
		p := newFocusPolicy(base, fpSpec(), clk.read)
		d := fpDecision(DPriority, fpSeat, attack, fpPass)
		if got := fpPick(p, d); got != d.Options[1] || base.calls != 1 {
			t.Errorf("an attack outside an attackers decision: picked %#x, base calls %d", got, base.calls)
		}
	})

	t.Run("a bad phase is refused", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("FocusPolicy took a phase it does not know")
			}
		}()
		spec := fpSpec()
		spec.Turn, spec.Phase = 2, "dusk"
		FocusPolicy(&fpBase{}, spec)
	})

	t.Run("deterministic over a replayed action list", func(t *testing.T) {
		// One scripted game: the clock walks turns 1 and 2 and the offers
		// repeat. Two policies built alike answer alike, and a third driven by
		// the first's action list sees every action offered.
		spec := fpSpec()
		spec.Turn, spec.Phase = 2, "main1"
		type at struct {
			turn int
			step state.Step
			d    *CDecision
		}
		var script []at
		for turn := 1; turn <= 3; turn++ {
			for _, step := range []state.Step{state.StepUpkeep, state.StepMain1, state.StepMain2, state.StepEnd} {
				script = append(script,
					at{turn, step, fpDecision(DPriority, fpSeat, fpPass, fpActivate1, fpCastOther, fpDone)},
					at{turn, step, fpDecision(DPriority, 1, fpPass, fpCastOther)},
					at{turn, step, fpDecision(DAttackers, fpSeat, Pack(ActAttack, fpFocus, RefPlayer+1, 0), fpDone)})
			}
		}
		play := func() []Canon {
			clk := &fpClock{ok: true}
			p := newFocusPolicy(&fpBase{}, spec, clk.read)
			var acts []Canon
			for _, s := range script {
				clk.turn, clk.step = s.turn, s.step
				acts = append(acts, fpPick(p, s.d))
			}
			return acts
		}
		first, second := play(), play()
		if !slices.Equal(first, second) {
			t.Fatal("two policies built alike played differently")
		}
		for i, a := range first {
			if !slices.Contains(script[i].d.Options, a) {
				t.Fatalf("step %d: action %#x is not offered", i, a)
			}
		}
		// The key is picked three times, then base answers.
		keys := 0
		for _, a := range first {
			if a == fpActivate1 {
				keys++
			}
		}
		if keys != focusKeyMaxPicks {
			t.Fatalf("the key was picked %d times over the script, want %d", keys, focusKeyMaxPicks)
		}
	})

	t.Run("reads the gorge side's turn and step", func(t *testing.T) {
		c, _ := testCorpus(t)
		sc, err := ParseScenario([]byte(scenarioExample))
		if err != nil {
			t.Fatal(err)
		}
		m, err := c.ScenarioMatchup(sc, 0)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, 1, Config{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		spec := FocusSpec{Seat: 0, Turn: 2, Phase: "main2"}
		base := &fpBase{}
		pol := FocusPolicy(base, spec)
		before, after := 0, 0
		for n := 0; n < 600; n++ {
			d := g.Step()
			if d.Kind == DNone || len(d.Options) == 0 {
				break
			}
			turn, step := int(g.E.G.Turn), g.E.G.Step
			a := pol.Pick(&PickCtx{D: &d, G: g, Seq: n})
			if d.Player == spec.Seat {
				if turn < spec.Turn || turn == spec.Turn && step < state.StepMain2 {
					before++
					if want := focusPass(&d); a != want {
						t.Fatalf("turn %d step %v: picked %#x, want the pass-through %#x", turn, step, a, want)
					}
				} else {
					after++
					if want := d.Options[len(d.Options)-1]; a != want {
						t.Fatalf("turn %d step %v: picked %#x, want base's %#x", turn, step, a, want)
					}
				}
			}
			g.Apply(a)
			if g.Err() != nil {
				t.Fatal(g.Err())
			}
			if after >= 3 {
				break
			}
		}
		if before == 0 || after == 0 {
			t.Fatalf("the game made %d decisions before and %d after turn 2 main2 for seat 0, want both", before, after)
		}
	})
}
