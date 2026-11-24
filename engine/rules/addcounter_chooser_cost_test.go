package rules

// AddCounter<N/M1M1/Creature.YouCtrl/desc> as a cost (Lethal Sting, Scarscale
// Ritual, Hatchet Bully, Wandering Mage) is a blight payment (CR 701.68a): N
// -1/-1 counters on a creature the payer controls, chosen after the mode and
// targets are announced. It parses to Cost.Blight and rides the whole blight
// pipeline, with no phantom generic mana and no reported Unknown.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const chooserCostToken = "AddCounter<1/M1M1/Creature.YouCtrl/a creature you control>"

const chooserCostSpellSrc = "Name:Sting Rite\nTypes:Sorcery\nManaCost:2 B\n" +
	"A:SP$ Destroy | Cost$ 2 B " + chooserCostToken + " | ValidTgts$ Creature | SpellDescription$ Destroy target creature.\nOracle:x\n"

func TestAddCounterChooserCostParses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		src     string
		generic int32
	}{
		{"2 B " + chooserCostToken, 2},
		{"2 R T " + chooserCostToken, 2},
		{"B " + chooserCostToken, 0},
		{chooserCostToken, 0},
	} {
		c := ParseCost(tc.src)
		if len(c.Blight) != 1 || c.Blight[0].Spec != "Creature.YouCtrl" || c.Blight[0].N != 1 {
			t.Fatalf("%q: Blight = %+v, want one {N:1 Creature.YouCtrl}", tc.src, c.Blight)
		}
		if c.Generic != tc.generic {
			t.Fatalf("%q: Generic = %d, want %d (no phantom generic)", tc.src, c.Generic, tc.generic)
		}
		if len(c.Unknown) != 0 || len(c.AddCounter) != 0 {
			t.Fatalf("%q: Unknown=%v AddCounter=%+v, want neither", tc.src, c.Unknown, c.AddCounter)
		}
	}
	// Other kinds and filters keep the reported one-generic fallback.
	for _, src := range []string{
		"AddCounter<1/P1P1/Creature.OppCtrl/x>",
		"AddCounter<1/M1M1/Creature.OppCtrl/x>",
		"AddCounter<1/Stun/Card.targetedBy/x>",
		"AddCounter<0/M1M1/Creature.YouCtrl/x>",
	} {
		c := ParseCost(src)
		if len(c.Blight) != 0 || len(c.Unknown) == 0 || c.Generic != 1 {
			t.Fatalf("%q = %+v, want the reported fallback", src, c)
		}
	}
}

// answerChooserCost drives the pending cast/activation decisions to priority:
// the target decision picks the option naming targetObj (or the opponent when
// targetObj is zero), and the "blightcost" pick takes candidate #pick. It
// returns the candidates offered.
func answerChooserCost(t *testing.T, e *Engine, targetObj state.ObjID, pick int) []decision.Option {
	t.Helper()
	var cands []decision.Option
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			return cands
		}
		switch {
		case d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "blightcost":
			if pick >= len(d.Options) {
				t.Fatalf("blightcost offered %d candidates, want > %d", len(d.Options), pick)
			}
			cands = d.Options
			submitChoices(t, e, d.Options[pick].Index)
		case d.Kind == decision.KChoose && len(d.Options) > 0 && strings.HasPrefix(d.Options[0].Kind, "pay_"):
			// A hybrid symbol's colour choice (Scarscale Ritual's {U/B}).
			submitChoices(t, e, d.Options[0].Index)
		case d.Kind == decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if (targetObj != 0 && o.Obj == targetObj) || (targetObj == 0 && o.Player == 1) {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("target %d not offered: %+v", targetObj, d.Options)
			}
			submitChoices(t, e, idx)
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	t.Fatalf("decisions did not settle: %+v", e.Pending())
	return cands
}

func TestAddCounterChooserCostSpellPutsCounterOnPickedCreature(t *testing.T) {
	t.Parallel()
	spell := card(t, chooserCostSpellSrc)
	mine := card(t, blightCreatureSrc)
	opp := card(t, "Name:Opp Bear\nTypes:Creature Bear\nPT:9/9\nOracle:x\n")
	e, cfg := tokenReplGameSeats(t, 311, []*cards.Card{spell, mine, mine}, []*cards.Card{opp})
	sp := moveSeededCard(t, e, 0, spell, state.ZHand)
	a := moveSeededCard(t, e, 0, mine, state.ZBattlefield)
	b := moveSeededCard(t, e, 0, mine, state.ZBattlefield)
	victim := moveSeededCard(t, e, 1, opp, state.ZBattlefield)
	if a == b {
		t.Fatalf("precondition: both creatures are the same object %d", a)
	}
	addMana(t, e, 0, "BBB")
	if got := e.G.Players[0].Pool.Total(); got != 3 {
		t.Fatalf("precondition: pool %d, want exactly {2}{B}", got)
	}
	submitChoices(t, e, castOptionFor(t, e, sp).Index)
	cands := answerChooserCost(t, e, victim, 1)
	if len(cands) != 2 || cands[0].Obj == cands[1].Obj {
		t.Fatalf("blightcost candidates %+v, want two distinct creatures", cands)
	}
	picked, other := cands[1].Obj, cands[0].Obj
	if got := e.G.Obj(picked).Counter("M1M1"); got != 1 {
		t.Fatalf("picked creature %d has %d M1M1 counters, want 1", picked, got)
	}
	if got := e.G.Obj(other).Counter("M1M1"); got != 0 {
		t.Fatalf("unpicked creature %d has %d M1M1 counters, want 0", other, got)
	}
	if got := countCounterChanges(e, picked, "M1M1", 1); got != 1 {
		t.Fatalf("got %d CounterChange(M1M1,+1) on the picked creature, want exactly 1", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool %d after the cast, want 0 (cost is {2}{B}, no phantom generic)", got)
	}
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(victim); o.Zone != state.ZGraveyard {
		t.Fatalf("target in zone %v after resolution, want graveyard", o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestAddCounterChooserCostSpellNotOfferedWithoutCreature(t *testing.T) {
	t.Parallel()
	spell := card(t, chooserCostSpellSrc)
	mine := card(t, blightCreatureSrc)
	opp := card(t, "Name:Opp Bear\nTypes:Creature Bear\nPT:9/9\nOracle:x\n")
	e, _ := tokenReplGameSeats(t, 312, []*cards.Card{spell, mine}, []*cards.Card{opp})
	sp := moveSeededCard(t, e, 0, spell, state.ZHand)
	moveSeededCard(t, e, 1, opp, state.ZBattlefield) // a target exists; the payer has no creature
	addMana(t, e, 0, "BBBB")
	for _, o := range castOptions(t, e) {
		if o.Obj == sp {
			t.Fatalf("cast offered with no creature to put the counter on (CR 118.3): %+v", o)
		}
	}
	// Control: with a creature of the payer's own, the same cast is offered.
	moveSeededCard(t, e, 0, mine, state.ZBattlefield)
	addMana(t, e, 0, "")
	_ = castOptionFor(t, e, sp)
}

func TestAddCounterChooserCostActivatedAbilities(t *testing.T) {
	t.Parallel()
	bully := card(t, "Name:Sting Bully\nTypes:Creature Goblin Warrior\nPT:3/3\nK:Haste\n"+
		"A:AB$ DealDamage | Cost$ 2 R T "+chooserCostToken+" | ValidTgts$ Any | NumDmg$ 2 | SpellDescription$ Deal 2.\nOracle:x\n")
	mage := card(t, "Name:Sting Mage\nTypes:Creature Human Wizard\nPT:0/3\n"+
		"A:AB$ GainLife | Cost$ B "+chooserCostToken+" | LifeAmount$ 1 | SpellDescription$ Gain 1.\nOracle:x\n")
	bear := card(t, blightCreatureSrc)
	e, cfg := tokenReplGame(t, 313, bully, mage, bear)
	bl := moveSeededCard(t, e, 0, bully, state.ZBattlefield)
	mg := moveSeededCard(t, e, 0, mage, state.ZBattlefield)
	br := moveSeededCard(t, e, 0, bear, state.ZBattlefield)

	// Hatchet Bully shape: candidates are bully, mage, bear; pick the bear.
	addMana(t, e, 0, "RRR")
	submitChoices(t, e, abilityOption(t, e, bl, 0).Index)
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision after the activation")
	}
	pick := -1
	if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "blightcost" {
		for _, o := range d.Options {
			if o.Obj == br {
				pick = o.Index
			}
		}
	}
	// The target is announced before the cost pick (CR 601.2c); answer it first.
	if d.Kind == decision.KTarget {
		submitChoices(t, e, d.Options[0].Index)
		d = e.Pending()
		for _, o := range d.Options {
			if o.Obj == br {
				pick = o.Index
			}
		}
	}
	if pick < 0 {
		t.Fatalf("bear not offered as the counter's creature: %+v", d)
	}
	submitChoices(t, e, pick)
	if got := e.G.Obj(br).Counter("M1M1"); got != 1 {
		t.Fatalf("picked bear has %d M1M1 counters, want 1", got)
	}
	if got := e.G.Obj(mg).Counter("M1M1") + e.G.Obj(bl).Counter("M1M1"); got != 0 {
		t.Fatalf("unpicked creatures carry %d M1M1 counters, want 0", got)
	}
	if !e.G.Obj(bl).Tapped {
		t.Fatal("Hatchet Bully's own {T} was not paid")
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool %d after the activation, want 0 (cost is {2}{R})", got)
	}
	passUntilStackEmpty(t, e, 20)

	// Wandering Mage shape: {B} plus the counter, picking the second candidate.
	addMana(t, e, 0, "B")
	submitChoices(t, e, abilityOption(t, e, mg, 0).Index)
	cands := answerChooserCost(t, e, 0, 1)
	if len(cands) < 2 {
		t.Fatalf("blightcost offered %+v, want several creatures", cands)
	}
	if got := e.G.Obj(cands[1].Obj).Counter("M1M1"); got < 1 {
		t.Fatalf("picked creature %d has no M1M1 counter after the {B} activation", cands[1].Obj)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool %d after the {B} activation, want 0", got)
	}
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}

func TestAddCounterChooserCostCorpusCards(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Lethal Sting", "Scarscale Ritual")
	sting := searchMoveByName(t, e, "Lethal Sting", state.ZHand)
	ritual := searchMoveByName(t, e, "Scarscale Ritual", state.ZHand)
	var bears []state.ObjID
	for i := 0; i < 3; i++ {
		bears = append(bears, searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield))
	}
	if bears[0] == bears[1] || bears[1] == bears[2] {
		t.Fatalf("precondition: bears are not distinct objects: %v", bears)
	}
	untapped := func() int {
		n := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o != nil && o.Tapped {
				n++
			}
		}
		return n
	}
	if untapped() != 0 {
		t.Fatal("precondition: a permanent is tapped before any cast")
	}

	// Lethal Sting: exactly {2}{B}, the counter on the SECOND candidate.
	addMana(t, e, 0, "BBB")
	submitChoices(t, e, castOptionFor(t, e, sting).Index)
	cands := answerChooserCost(t, e, bears[0], 1)
	if len(cands) < 2 || cands[0].Obj == cands[1].Obj {
		t.Fatalf("blightcost candidates %+v, want distinct creatures", cands)
	}
	if got := e.G.Obj(cands[1].Obj).Counter("M1M1"); got != 1 {
		t.Fatalf("Lethal Sting: picked creature has %d M1M1 counters, want 1", got)
	}
	if got := e.G.Obj(cands[0].Obj).Counter("M1M1"); got != 0 {
		t.Fatalf("Lethal Sting: unpicked creature has %d M1M1 counters, want 0", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("Lethal Sting left %d mana in the pool, want 0 ({2}{B} exactly)", got)
	}
	passUntilStackEmpty(t, e, 30)

	// Scarscale Ritual: exactly {1}{U/B}.
	handBefore := len(e.G.Zone(state.ZHand, 0))
	addMana(t, e, 0, "BB")
	submitChoices(t, e, castOptionFor(t, e, ritual).Index)
	cands = answerChooserCost(t, e, 0, 0)
	if len(cands) == 0 {
		t.Fatal("Scarscale Ritual posed no blightcost pick")
	}
	// The picked bear may already carry Lethal Sting's counter and die to this
	// one (1/1 -> 0/0), so read the event, not the live counter.
	if got := countCounterChanges(e, cands[0].Obj, "M1M1", 1); got < 1 {
		t.Fatalf("Scarscale Ritual: %d CounterChange(M1M1,+1) on the picked creature, want >= 1", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("Scarscale Ritual left %d mana in the pool, want 0 ({1}{U/B} exactly)", got)
	}
	passUntilStackEmpty(t, e, 30)
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("hand %d -> %d after Scarscale Ritual, want +1 (-1 the cast, +2 drawn)", handBefore, got)
	}
	replayCheck(t, e, cfg)
}

func TestAddCounterChooserCostBotAnswerValidates(t *testing.T) {
	t.Parallel()
	spell := card(t, chooserCostSpellSrc)
	mine := card(t, blightCreatureSrc)
	opp := card(t, "Name:Opp Bear\nTypes:Creature Bear\nPT:9/9\nOracle:x\n")
	e, _ := tokenReplGameSeats(t, 314, []*cards.Card{spell, mine, mine}, []*cards.Card{opp})
	sp := moveSeededCard(t, e, 0, spell, state.ZHand)
	moveSeededCard(t, e, 0, mine, state.ZBattlefield)
	moveSeededCard(t, e, 0, mine, state.ZBattlefield)
	victim := moveSeededCard(t, e, 1, opp, state.ZBattlefield)
	addMana(t, e, 0, "BBB")
	submitChoices(t, e, castOptionFor(t, e, sp).Index)
	d := e.Pending()
	if d != nil && d.Kind == decision.KTarget {
		for _, o := range d.Options {
			if o.Obj == victim {
				submitChoices(t, e, o.Index)
			}
		}
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KChoose || len(d.Options) < 2 || d.Options[0].Kind != "blightcost" {
		t.Fatalf("precondition: want the blightcost pick over two creatures, got %+v", d)
	}
	got := botpolicy.NewGameBot(7).Answer(e.G, e, d)
	if err := d.Validate(got); err != nil {
		t.Fatalf("bot answer %+v fails Validate: %v", got, err)
	}
	if err := d.Validate(botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player})); err != nil {
		t.Fatalf("clamped answer fails Validate: %v", err)
	}
}
