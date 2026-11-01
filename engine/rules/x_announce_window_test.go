package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 601.2b announces X before CR 601.2g's window opens, so the announcement
// reaches what the untapped lands can pay, and the window then pays it.
func TestXAnnouncementReachesUntappedSources(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Cryptic Trilobite"))
	id := e.G.Zone(state.ZHand, 0)[0]
	for range 8 {
		onBoard(t, e, 0, xForest)
	}
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, castModeOption(t, e, id, ""))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the X ask, got %+v", d)
	}
	// {X}{X} on eight lands: X up to 4.
	if got := d.Options[len(d.Options)-1].Amount; got != 4 {
		t.Fatalf("largest X offered = %d, want 4: %+v", got, d.Options)
	}
	submitChoices(t, e, d.Options[len(d.Options)-1].Index)
	if d = e.Pending(); d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the CR 601.2g mana window, got %+v", d)
	}
}

// A window that cannot pay a larger X does not offer it.
func TestXAnnouncementStopsAtWhatTheSourcesPay(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Cryptic Trilobite"))
	id := e.G.Zone(state.ZHand, 0)[0]
	for range 3 {
		onBoard(t, e, 0, xForest)
	}
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, castModeOption(t, e, id, ""))
	d := e.Pending()
	if got := d.Options[len(d.Options)-1].Amount; got != 1 {
		t.Fatalf("largest X offered = %d, want 1: %+v", got, d.Options)
	}
}

// A source tapped as part of the cost cannot also pay X (CR 601.2h pays both).
func TestXAnnouncementLeavesOutATapCostSource(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t)
	id := onBoard(t, e, 0, "Name:Tap Sink\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ one green\n"+
		"A:AB$ GainLife | Cost$ X T | LifeAmount$ X | SpellDescription$ Gain X.\nSVar:X:Count$xPaid\nOracle:x\n")
	e.G.Obj(id).SummonSick = false
	onBoard(t, e, 0, xForest)
	e.pending = nil
	e.priorityRound()
	abilityIdx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == id {
			abilityIdx = o.Index
		}
	}
	if abilityIdx < 0 {
		t.Fatalf("X ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, abilityIdx)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the X ask, got %+v", d)
	}
	if got := d.Options[len(d.Options)-1].Amount; got != 1 {
		t.Fatalf("largest X offered = %d, want 1 from the other land: %+v", got, d.Options)
	}
}

const xForest = "Name:Fixture Forest\nTypes:Basic Land Forest\nA:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ one green\nOracle:x\n"

// A payer who stops short of an announced X reverses the cast (CR 733.1): the
// spell goes back to hand, the tapped lands' mana keeps floating, and priority
// returns.
func TestXAnnouncementStoppedShortReverses(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Cryptic Trilobite"))
	id := e.G.Zone(state.ZHand, 0)[0]
	for range 8 {
		onBoard(t, e, 0, xForest)
	}
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, castModeOption(t, e, id, ""))
	d := e.Pending()
	submitChoices(t, e, d.Options[len(d.Options)-1].Index)
	d = e.Pending()
	for _, o := range d.Options {
		if o.Kind == "activate" {
			submitChoices(t, e, o.Index)
			break
		}
	}
	d = e.Pending()
	done := -1
	for _, o := range d.Options {
		if o.Kind == "done" {
			done = o.Index
		}
	}
	if done < 0 {
		t.Fatalf("no way out of the window: %+v", d.Options)
	}
	submitChoices(t, e, done)
	if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("want priority after the reversal, got %+v", d)
	}
	if o := e.G.Obj(id); o.Zone != state.ZHand || e.cast != nil {
		t.Fatalf("spell not reversed: zone %s cast %+v", o.Zone, e.cast)
	}
	if got := e.G.Players[0].Pool.Total(); got != 1 {
		t.Fatalf("pool = %d, want the tapped land's 1", got)
	}
}

// floatOneMana taps one land from priority, so the pool covers a coloured pip
// and the cast is offered.
func floatOneMana(t *testing.T, e *Engine) {
	t.Helper()
	for _, o := range e.Pending().Options {
		if o.Kind == "activate" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no mana ability offered: %+v", e.Pending().Options)
}

// CR 601.2c: "X target nonland permanents" needs X legal targets, however much
// mana the window could pay for.
func TestXAnnouncementStopsAtTheLegalTargets(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Gridlock"))
	id := e.G.Zone(state.ZHand, 0)[0]
	for range 8 {
		onBoard(t, e, 0, "Name:Fixture Island\nTypes:Basic Land Island\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ one blue\nOracle:x\n")
	}
	for range 2 {
		onBoard(t, e, 1, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	}
	e.pending = nil
	e.priorityRound()
	floatOneMana(t, e)
	submitChoices(t, e, castModeOption(t, e, id, ""))
	d := e.Pending()
	if got := d.Options[len(d.Options)-1].Amount; got != 2 {
		t.Fatalf("largest X offered = %d, want the 2 legal targets: %+v", got, d.Options)
	}
}

// A spell priced per target tells an unattended bot how many targets its
// announced X left room for (CR 601.2f).
func TestXAnnouncementLeavesRoomForPricedTargets(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ x, want int }{{5, 1}, {4, 2}} {
		e := handEngineTokens(t, corpusAlternativeCard(t, "Fireball"))
		id := e.G.Zone(state.ZHand, 0)[0]
		for range 6 {
			onBoard(t, e, 0, "Name:Fixture Mountain\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ one red\nOracle:x\n")
		}
		e.pending = nil
		e.priorityRound()
		floatOneMana(t, e)
		submitChoices(t, e, castModeOption(t, e, id, ""))
		for _, o := range e.Pending().Options {
			if o.Amount == c.x {
				submitChoices(t, e, o.Index)
				break
			}
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("X=%d: want the target ask, got %+v", c.x, d)
		}
		if d.AffordableTargets != c.want {
			t.Fatalf("X=%d: AffordableTargets = %d, want %d", c.x, d.AffordableTargets, c.want)
		}
	}
}
