package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A lowering whose funding activations win the game is a completed play. The
// Treasure sacrificed for the black mana of an Adventure half's {1}{B}
// triggers The Sackville-Bagginses, which takes the opponent's last life; the
// kernel makes the cast from inside its payment window, so the trigger waits
// until the spell is cast (CR 601.2g, 603.3b), while the lowering floats the
// mana first and the trigger ends the game before the cast. The vocabulary
// must still carry the play the kernel offers.
func TestLoweringThatWinsTheGameIsACompletedPlay(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	look := func(name string) *cards.Card {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the corpus", name)
		}
		return c
	}
	var deck [40]*cards.Card
	for i := range deck {
		deck[i] = look("Mountain")
	}
	deck[0], deck[1] = look("The Sackville-Bagginses"), look("Great Ugly-Looking Goblin")
	decks := [2][]*cards.Card{deck[:], deck[:]}
	m, err := NewMatchup(reg, [2]string{"drain", "opponent"}, decks, 0)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewGorgeSide(m, 1, Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	move := func(name string, to state.Zone) state.ObjID {
		id := ebgFind(t, g, name, state.ZLibrary)
		g.E.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: to})
		return id
	}
	move("The Sackville-Bagginses", state.ZBattlefield)
	goblin := move("Great Ugly-Looking Goblin", state.ZHand)
	for i := 0; i < 2; i++ {
		move("Mountain", state.ZBattlefield)
	}
	g.E.Emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
	g.E.Emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 1 - g.E.G.Players[1].Life})

	var d *decision.Decision
	for i := 0; i < 800; i++ {
		d = g.E.Pending()
		if d == nil || g.E.G.Over {
			t.Fatalf("game ended before seat 0 reached main (iter %d, over %v, pending nil %v, step %v)", i, g.E.G.Over, d == nil, g.E.G.Step)
		}
		if d.Kind == decision.KPriority && d.Player == 0 && g.E.G.Active == 0 && g.E.G.Step == state.StepMain1 && len(g.E.G.Stack) == 0 {
			break
		}
		in := botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player})
		in.Seq, in.Player = d.Seq, d.Player
		for _, o := range d.Options {
			// pass at priority, and decline the "may sacrifice" ask so the
			// drain waits for the play
			if d.Kind == decision.KPriority && o.Kind == "pass" || o.Kind == "trigger_cost_decline" {
				in.Choices = []int{o.Index}
			}
		}
		if err := g.submit(in); err != nil {
			t.Fatal(err)
		}
	}
	if d.Kind != decision.KPriority || g.E.G.Step != state.StepMain1 {
		t.Fatal("never reached seat 0's main phase")
	}
	g.id.sync(g.E.G)
	fc, ok := faceCastCode("adventure_alt")
	if !ok {
		t.Fatal("no face code for adventure_alt")
	}
	want := Pack(ActCastFace, g.id.ref(g.E.G, goblin), RefNone, fc)
	fr := &frame{seq: d.Seq, d: d.Clone(), dk: DPriority}
	g.priorityVocabulary(fr)
	for _, pe := range fr.prio {
		if pe.a == want {
			return
		}
	}
	t.Fatalf("the vocabulary lacks the Adventure cast the kernel offers; unpriced %v", g.Stats.UnpricedPlays)
}
