package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// brazenBluffFixture sets up the scenario: seat 0 casts a noncreature spell
// (Gain Two, W) and a creature spell; seat 1 holds Brazen Bluff plus, for the
// free cast, a noncreature spell (Draw One: a card, via a distinct name so the
// events say which was cast) and a creature card the free cast must not offer.
func brazenBluffFixture(t *testing.T) (e *Engine, gain, bear, bluff, free, crit state.ObjID) {
	t.Helper()
	gainSrc := "Name:Gain Two\nManaCost:W\nTypes:Instant\nA:SP$ GainLife | Defined$ You | LifeAmount$ 2\nOracle:x\n"
	bearSrc := "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e = handEngine(t, card(t, gainSrc), card(t, bearSrc))
	add := func(src string) state.ObjID {
		o := e.G.AddObject(card(t, src), 1)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, 1, append(append([]state.ObjID{}, e.G.Zone(state.ZHand, 1)...), o.ID))
		return o.ID
	}
	b := e.G.AddObject(choiceCorpusCard(t, "Brazen Bluff"), 1)
	b.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{b.ID})
	free = add("Name:Free Two\nManaCost:5 W\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n")
	crit = add("Name:Crit\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	for _, id := range e.G.Zone(state.ZHand, 0) {
		switch e.G.Obj(id).Face().Name {
		case "Gain Two":
			gain = id
		case "Bear":
			bear = id
		}
	}
	e.G.Players[0].Pool[state.MW] = 5
	e.G.Players[0].Pool[state.MG] = 5
	e.G.Players[1].Pool[state.MR] = 5
	e.askPriority(0)
	return e, gain, bear, b.ID, free, crit
}

func TestBrazenBluffGorge(t *testing.T) {
	t.Parallel()
	t.Run("creature spell is no target", func(t *testing.T) {
		t.Parallel()
		e, _, bear, bluff, _, _ := brazenBluffFixture(t)
		submitChoices(t, e, passToCast(t, e, bear))
		submitChoicePass(t, e) // seat 0 passes; seat 1 holds priority
		d := e.Pending()
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == bluff {
				t.Fatalf("Brazen Bluff castable with only a creature spell on the stack: %+v", o)
			}
		}
	})
	for _, accept := range []bool{true, false} {
		name := "decline"
		if accept {
			name = "accept"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, gain, bear, bluff, free, crit := brazenBluffFixture(t)
			submitChoices(t, e, passToCast(t, e, bear))
			submitChoices(t, e, passToCast(t, e, gain))
			submitChoicePass(t, e)
			submitChoices(t, e, passToCast(t, e, bluff))
			d := e.Pending()
			if d == nil || d.Kind != decision.KTarget {
				t.Fatalf("want a target ask, got %+v", d)
			}
			if len(d.Options) != 1 || d.Options[0].Obj != gain {
				t.Fatalf("targets %+v, want only the noncreature spell %d (not the creature spell %d)", d.Options, gain, bear)
			}
			submitChoices(t, e, d.Options[0].Index)
			for i := 0; i < 8; i++ {
				d = e.Pending()
				if d.Kind == decision.KChoose {
					break
				}
				submitChoicePass(t, e)
			}
			if d.Kind != decision.KChoose || d.Player != 0 {
				t.Fatalf("the spell's controller (seat 0) must answer, got %+v", d)
			}
			want := "no"
			if accept {
				want = "yes"
			}
			for _, o := range d.Options {
				if o.Kind == want {
					submitChoices(t, e, o.Index)
					break
				}
			}
			if accept {
				// exiled: the spell never resolves (CR 608.2b) and no free cast is offered
				passUntilStackEmpty(t, e, 8)
				if z := e.G.Obj(gain).Zone; z != state.ZExile {
					t.Fatalf("Gain Two zone %v, want exile", z)
				}
				if e.G.Players[0].Life != 20 || e.G.Players[1].Life != 20 {
					t.Fatalf("lives %d/%d: the exiled spell must not resolve", e.G.Players[0].Life, e.G.Players[1].Life)
				}
				if z := e.G.Obj(free).Zone; z != state.ZHand {
					t.Fatalf("free spell zone %v, want hand (no free cast after an exile)", z)
				}
				return
			}
			d = e.Pending()
			if d == nil || d.ResumeKind != "play" || d.Player != 1 {
				t.Fatalf("want seat 1's free-cast ask, got %+v", d)
			}
			for _, o := range d.Options {
				if o.Obj == crit {
					t.Fatalf("a creature card is offered for the noncreature free cast: %+v", d.Options)
				}
			}
			if len(d.Options) != 1 || d.Options[0].Obj != free {
				t.Fatalf("free cast options %+v, want only %d", d.Options, free)
			}
			submitChoices(t, e, d.Options[0].Index)
			passUntilStackEmpty(t, e, 12)
			// the free spell (+3 to seat 1) resolved without mana; Gain Two (+2 to seat 0) is not exiled
			if l0, l1 := e.G.Players[0].Life, e.G.Players[1].Life; l0 != 22 || l1 != 23 {
				t.Fatalf("lives %d/%d, want 22/23", l0, l1)
			}
			if p := e.G.Players[1].Pool[state.MR]; p != 3 {
				t.Fatalf("seat 1 red pool %d, want 3 (Bluff cost RR, free spell cost nothing)", p)
			}
		})
	}
}
